package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
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

// portForward owns one `kubectl port-forward` child kept alive for a
// server's lifetime. localPort is the chosen port on the kubectl host's
// loopback; stop terminates the child.
type portForward struct {
	localPort int
	stop      func()
}

// startPortForward runs `kubectl port-forward <target> :<remotePort>` via
// exec.Stream, waits for the "Forwarding from …" readiness announcement,
// and returns once the port is available. target must be a Service
// (e.g. "svc/<name>") so the forward survives pod restarts.
func startPortForward(exec Executor, kctl, ns, target string, remotePort int) (*portForward, error) {
	cmd := strings.Join(portForwardArgs(kctl, ns, target, remotePort), " ")
	ch, stop, err := exec.Stream(cmd)
	if err != nil {
		return nil, fmt.Errorf("port-forward start: %w", err)
	}
	timeout := time.After(10 * time.Second)
	for {
		select {
		case line, ok := <-ch:
			if !ok {
				stop()
				return nil, fmt.Errorf("port-forward: exited before announcing port")
			}
			if port, ok := parseForwardedPort(line); ok {
				return &portForward{localPort: port, stop: stop}, nil
			}
		case <-timeout:
			stop()
			return nil, fmt.Errorf("port-forward: timed out waiting for forwarding announcement")
		}
	}
}

// dbServiceFromPod derives the StatefulSet Service name from a pod name.
// Pod pattern: <sts-name>-<ordinal>; strip the ordinal to get the Service.
func dbServiceFromPod(pod string) string {
	if idx := strings.LastIndex(pod, "-"); idx > 0 {
		return pod[:idx]
	}
	return pod
}
