package main

import (
	"fmt"
	"regexp"
	"strconv"
)

// Pure helpers for the `data-plane: portforward` patch (kubectl-control DB/broker
// connection on multi-node clusters reached through a jumphost). See
// dune-admin-notes specs/2026-06-22-portforward-data-plane.md. The managed
// port-forward (startPortForward) + connectServer wiring build on these.

// portForwardArgs builds the kubectl argv that forwards a service/pod port on
// the kubectl host: `<kctl> port-forward -n <ns> <target> :<remotePort>`. The
// leading ":" lets kubectl pick a free local port (parsed via
// parseForwardedPort). kctl is the configured kubectl invocation token.
func portForwardArgs(kctl, ns, target string, remotePort int) []string {
	return []string{kctl, "port-forward", "-n", ns, target, fmt.Sprintf(":%d", remotePort)}
}

// forwardLineRE matches kubectl's "Forwarding from <addr>:<localPort> -> <remote>"
// announcement (IPv4 or [::1]); the captured group is the chosen local port.
var forwardLineRE = regexp.MustCompile(`Forwarding from .*:(\d+)\s*->`)

// parseForwardedPort extracts the local port kubectl chose from a forwarding
// announcement line, returning ok=false for any line that is not such an
// announcement.
func parseForwardedPort(line string) (port int, ok bool) {
	m := forwardLineRE.FindStringSubmatch(line)
	if m == nil {
		return 0, false
	}
	p, err := strconv.Atoi(m[1])
	if err != nil {
		return 0, false
	}
	return p, true
}
