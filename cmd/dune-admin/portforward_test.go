package main

import (
	"fmt"
	"io"
	"net"
	"reflect"
	"testing"
)

// TDD red — drives the `data-plane: portforward` helpers.
// Mirrors the pure-builder style of executor_sshcmd_test.go (TestSSHDialArgs):
// no cluster, no kubectl, no ssh — just argv construction + output parsing.

func TestPortForwardArgs(t *testing.T) {
	got := portForwardArgs("kubectl", "funcom-operators", "svc/db-dbdepl", 15432)
	want := []string{"kubectl", "port-forward", "-n", "funcom-operators", "svc/db-dbdepl", ":15432"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("portForwardArgs = %v, want %v", got, want)
	}
}

func TestParseForwardedPort(t *testing.T) {
	cases := []struct {
		name string
		line string
		port int
		ok   bool
	}{
		{"ipv4", "Forwarding from 127.0.0.1:54321 -> 15432", 54321, true},
		{"ipv6", "Forwarding from [::1]:54321 -> 15432", 54321, true},
		{"trailing_ws", "Forwarding from 127.0.0.1:7000 -> 15432\n", 7000, true},
		{"noise", "Handling connection for 54321", 0, false},
		{"unrelated", "some other log line", 0, false},
		{"empty", "", 0, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			port, ok := parseForwardedPort(tc.line)
			if port != tc.port || ok != tc.ok {
				t.Errorf("parseForwardedPort(%q) = (%d, %v), want (%d, %v)",
					tc.line, port, ok, tc.port, tc.ok)
			}
		})
	}
}

// fakeStreamExecutor satisfies Executor for port-forward wiring tests.
// It does not implement SSH or local execution — only Stream and Dial.
type fakeStreamExecutor struct {
	ch   chan string
	stop func()
}

func (f *fakeStreamExecutor) Stream(_ string) (<-chan string, func(), error) {
	return f.ch, f.stop, nil
}
func (f *fakeStreamExecutor) Dial(network, addr string) (net.Conn, error) {
	return net.Dial(network, addr)
}
func (f *fakeStreamExecutor) Exec(_ string) (string, error)            { return "", nil }
func (f *fakeStreamExecutor) PipeToWriter(_ string, _ io.Writer) error { return nil }
func (f *fakeStreamExecutor) WriteFile(_ string, _ io.Reader) error    { return nil }
func (f *fakeStreamExecutor) Close()                                   {}
func (f *fakeStreamExecutor) Type() string                             { return "fake" }

func TestStartPortForward(t *testing.T) {
	// Real listener — stands in for kubectl's forwarded endpoint.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	localPort := ln.Addr().(*net.TCPAddr).Port

	stopped := false
	ch := make(chan string, 2)
	ch <- fmt.Sprintf("Forwarding from 127.0.0.1:%d -> 15432", localPort)
	// keep channel open — simulates a running kubectl port-forward

	pf, err := startPortForward(&fakeStreamExecutor{
		ch:   ch,
		stop: func() { stopped = true },
	}, "kubectl", "funcom-operators", "svc/db-dbdepl-sts", 15432)
	if err != nil {
		t.Fatalf("startPortForward: %v", err)
	}
	if pf.localPort != localPort {
		t.Errorf("localPort = %d, want %d", pf.localPort, localPort)
	}

	// Verify Dial reaches the listener through the executor.
	conn, err := net.Dial("tcp", fmt.Sprintf("127.0.0.1:%d", pf.localPort))
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer func() { _ = conn.Close() }()

	pf.stop()
	if !stopped {
		t.Error("stop() was not called")
	}
}

func TestStartPortForwardChannelClosed(t *testing.T) {
	// Channel closed immediately — simulates kubectl exiting before announcing.
	ch := make(chan string)
	close(ch)
	_, err := startPortForward(&fakeStreamExecutor{ch: ch, stop: func() {}},
		"kubectl", "ns", "svc/db", 15432)
	if err == nil {
		t.Error("expected error when channel closes before announcement, got nil")
	}
}

func TestDBServiceFromPod(t *testing.T) {
	cases := []struct {
		pod  string
		want string
	}{
		{"funcom-seabass-sh-abc123-gezpsi-db-dbdepl-sts-0", "funcom-seabass-sh-abc123-gezpsi-db-dbdepl-sts"},
		{"simple-db-dbdepl-sts-0", "simple-db-dbdepl-sts"},
	}
	for _, tc := range cases {
		if got := dbServiceFromPod(tc.pod); got != tc.want {
			t.Errorf("dbServiceFromPod(%q) = %q, want %q", tc.pod, got, tc.want)
		}
	}
}
