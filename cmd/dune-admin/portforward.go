package main

import "fmt"

// kubectlExecDialCmd builds the command that runs nc inside the DB pod so the
// TCP connection to PostgreSQL originates from within the pod network. This
// satisfies pg_hba.conf entries that restrict access to pod-network IPs —
// kubectl port-forward appears from the kubelet/node IP and is rejected.
//
// The resulting command is passed to Executor.DialCommand, which runs it on
// the SSH host (vm-jh-01) and pipes its stdin/stdout back as a net.Conn.
// Each pgxpool connection spawns its own nc subprocess.
func kubectlExecDialCmd(kctl, ns, pod string, port int) string {
	return fmt.Sprintf("%s exec -n %s %s -i -- nc 127.0.0.1 %d", kctl, ns, pod, port)
}
