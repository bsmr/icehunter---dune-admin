package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sort"
	"strconv"
	"time"
)

// ── Mesh web proxy ───────────────────────────────────────────────────────────
// Game-side HTTP UIs (Battlegroup Director, File Browser) live on node ports the
// operator's browser can't reach over the Nebula mesh. dune-admin already reaches
// them through the executor/SSH tunnel (same path as the DB pool). This serves
// each such service from a dedicated local port via a root reverse proxy, so the
// browser only talks to dune-admin and the services' absolute asset paths
// (/Script/…, /static/…) resolve correctly.

// proxyTarget is a resolved web interface to reverse-proxy: its label, the
// host:port to Dial through the executor, and the local listener port.
type proxyTarget struct {
	label    string
	dialAddr string
	port     int
}

// resolveProxyTargets selects the proxyable web interfaces, sorts them
// deterministically by dial address, and assigns port = listenPort+10+index.
//
// Proxyable = an entry dune-admin can Dial: a discovered entry (its Target, the
// raw CRD host:port) or a hand-configured absolute http(s) URL. Same-origin
// "/path" entries are skipped — they are already reachable as-is.
func resolveProxyTargets(ifaces []webInterface, listenPort int) []proxyTarget {
	var targets []proxyTarget
	for _, w := range ifaces {
		dial := w.Target
		if dial == "" {
			dial = dialAddrFromURL(w.URL)
		}
		if dial == "" {
			continue
		}
		targets = append(targets, proxyTarget{label: w.Label, dialAddr: dial})
	}
	sort.Slice(targets, func(i, j int) bool { return targets[i].dialAddr < targets[j].dialAddr })
	for i := range targets {
		targets[i].port = listenPort + 10 + i
	}
	return targets
}

// dialAddrFromURL returns host:port for an absolute http(s) URL, filling in the
// default port from the scheme. Returns "" for relative or non-http(s) URLs.
func dialAddrFromURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return ""
	}
	if u.Port() != "" {
		return u.Host
	}
	if u.Scheme == "https" {
		return net.JoinHostPort(u.Hostname(), "443")
	}
	return net.JoinHostPort(u.Hostname(), "80")
}

// webInterfaceOut is a web interface enriched with its assigned proxy port for
// the API. ProxyPort is 0 (omitted) for entries that are not proxied.
type webInterfaceOut struct {
	Label     string `json:"label"`
	URL       string `json:"url"`
	ProxyPort int    `json:"proxyPort,omitempty"`
}

// withProxyPorts attaches each entry's proxy port (matched by dial address) so
// the frontend can open http://<window.location.hostname>:<proxyPort>/ instead
// of the unreachable rewritten URL.
func withProxyPorts(ifaces []webInterface, targets []proxyTarget) []webInterfaceOut {
	portByAddr := make(map[string]int, len(targets))
	for _, t := range targets {
		portByAddr[t.dialAddr] = t.port
	}
	out := make([]webInterfaceOut, 0, len(ifaces))
	for _, w := range ifaces {
		dial := w.Target
		if dial == "" {
			dial = dialAddrFromURL(w.URL)
		}
		out = append(out, webInterfaceOut{Label: w.Label, URL: w.URL, ProxyPort: portByAddr[dial]})
	}
	return out
}

// listenPortNum returns the numeric port from listenAddr (e.g. ":8080" → 8080),
// or 0 when it can't be parsed.
func listenPortNum() int {
	_, p, err := net.SplitHostPort(listenAddr)
	if err != nil {
		return 0
	}
	n, _ := strconv.Atoi(p)
	return n
}

// newRootProxy reverse-proxies the entire root path to target, tunneling upstream
// connections through dial. Unlike newDirectorProxy it does NOT strip a prefix:
// the proxied app owns the whole port, so its absolute asset paths resolve.
func newRootProxy(target *url.URL, dial func(network, addr string) (net.Conn, error)) http.HandlerFunc {
	proxy := httputil.NewSingleHostReverseProxy(target)
	proxy.Transport = httpTransportVia(dial)
	return func(w http.ResponseWriter, r *http.Request) {
		r.Host = target.Host
		proxy.ServeHTTP(w, r)
	}
}

// withProxyAuth enforces the same dashboard session as the main UI when auth is
// enabled. Session cookies are host-scoped (not port-scoped), so the login on the
// main port is presented to the proxy ports too.
func withProxyAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authEnabled(loadedConfig) {
			if _, ok := authenticateRequest(w, r); !ok {
				return // authenticateRequest already wrote 401
			}
		}
		next(w, r)
	}
}

// startWebProxies launches one root-proxy http.Server per target and returns a
// func that shuts them all down. Each listener is bound synchronously (so the
// port is open before this returns); a bind failure is logged and that single
// proxy skipped — never fatal. dial is injected for testability.
func startWebProxies(targets []proxyTarget, dial func(network, addr string) (net.Conn, error)) func() {
	var servers []*http.Server
	for _, t := range targets {
		upstream := &url.URL{Scheme: "http", Host: t.dialAddr}
		mux := http.NewServeMux()
		mux.HandleFunc("/", withProxyAuth(newRootProxy(upstream, dial)))
		srv := &http.Server{
			Addr:              fmt.Sprintf(":%d", t.port),
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		}
		ln, err := net.Listen("tcp", srv.Addr)
		if err != nil {
			log.Printf("web-proxy: %s on :%d: %v (skipped)", t.label, t.port, err)
			continue
		}
		log.Printf("web-proxy: %s → :%d (upstream %s)", t.label, t.port, t.dialAddr)
		servers = append(servers, srv)
		go func() { _ = srv.Serve(ln) }()
	}
	return func() {
		sc, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		for _, s := range servers {
			_ = s.Shutdown(sc)
		}
	}
}
