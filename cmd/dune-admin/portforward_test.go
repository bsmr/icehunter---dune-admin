package main

import (
	"fmt"
	"io"
	"net"
	"testing"
)

// TDD red — drives the `data-plane: portforward` helpers.
// Mirrors the pure-builder style of executor_sshcmd_test.go (TestSSHDialArgs):
// no cluster, no kubectl, no ssh — just argv construction and command parsing.

func TestKubectlExecDialCmd(t *testing.T) {
	cases := []struct {
		name    string
		kctl    string
		ns      string
		pod     string
		port    int
		wantCmd string
	}{
		{
			name:    "basic",
			kctl:    "kubectl",
			ns:      "funcom-seabass-sh-abc-gezpsi",
			pod:     "sh-abc-gezpsi-db-dbdepl-sts-0",
			port:    15432,
			wantCmd: "kubectl exec -n funcom-seabass-sh-abc-gezpsi sh-abc-gezpsi-db-dbdepl-sts-0 -i -- nc 127.0.0.1 15432",
		},
		{
			name:    "kubeconfig prefix",
			kctl:    "KUBECONFIG=/home/dune/kubeconfig kubectl",
			ns:      "funcom-test",
			pod:     "test-db-dbdepl-sts-0",
			port:    15432,
			wantCmd: "KUBECONFIG=/home/dune/kubeconfig kubectl exec -n funcom-test test-db-dbdepl-sts-0 -i -- nc 127.0.0.1 15432",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := kubectlExecDialCmd(tc.kctl, tc.ns, tc.pod, tc.port)
			if got != tc.wantCmd {
				t.Errorf("kubectlExecDialCmd = %q, want %q", got, tc.wantCmd)
			}
		})
	}
}

// captureDialCommandExecutor satisfies Executor for connectDBViaPortForward tests.
// It records the command passed to DialCommand and returns an error so the pool
// creation fails without needing a real PostgreSQL server.
type captureDialCommandExecutor struct {
	dialCmd string
}

func (e *captureDialCommandExecutor) DialCommand(cmd string) (net.Conn, error) {
	e.dialCmd = cmd
	return nil, fmt.Errorf("test: stop here (captured %q)", cmd)
}
func (e *captureDialCommandExecutor) Exec(_ string) (string, error) { return "", nil }
func (e *captureDialCommandExecutor) Stream(_ string) (<-chan string, func(), error) {
	return nil, func() {}, nil
}
func (e *captureDialCommandExecutor) PipeToWriter(_ string, _ io.Writer) error { return nil }
func (e *captureDialCommandExecutor) WriteFile(_ string, _ io.Reader) error    { return nil }
func (e *captureDialCommandExecutor) Dial(_ string, _ string) (net.Conn, error) {
	return nil, nil
}
func (e *captureDialCommandExecutor) Close()       {}
func (e *captureDialCommandExecutor) Type() string { return "fake" }

func TestConnectDBViaPortForward_BuildsKubectlExecCommand(t *testing.T) {
	exec := &captureDialCommandExecutor{}
	sc := &ServerContext{
		PodNS: "funcom-test",
		Pod:   "test-db-dbdepl-sts-0",
	}
	cfg := ServerConfig{
		KubectlBin: "kubectl",
		DBPort:     15432,
		DBUser:     "dune",
		DBPass:     "dune",
		DBName:     "dune",
		DBSchema:   "dune",
	}

	_, err := connectDBViaPortForward(exec, sc, cfg)
	if err == nil {
		t.Fatal("expected error (DialCommand returns error), got nil")
	}

	wantCmd := "kubectl exec -n funcom-test test-db-dbdepl-sts-0 -i -- nc 127.0.0.1 15432"
	if exec.dialCmd != wantCmd {
		t.Errorf("DialCommand called with %q, want %q", exec.dialCmd, wantCmd)
	}
}
