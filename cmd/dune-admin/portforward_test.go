package main

import (
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
