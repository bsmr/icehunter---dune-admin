package main

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"testing"
	"time"
)

// resolveProxyTargets picks proxyable entries, sorts them deterministically by
// dial address, and assigns ports = listenPort+10+index. Sorting by dialAddr
// means File Browser (…:18888) sorts before Director (…:31003).
func TestResolveProxyTargets(t *testing.T) {
	ifaces := []webInterface{
		{Label: "File Browser", URL: "http://vm:18888/", Target: "10.0.0.5:18888"},
		{Label: "Battlegroup Director", URL: "http://vm:31003/", Target: "10.0.0.5:31003"},
		{Label: "Wiki", URL: "https://wiki.example/"}, // manual absolute → proxied, port from scheme
		{Label: "Local", URL: "/grafana"},             // same-origin → NOT proxied
	}
	got := resolveProxyTargets(ifaces, 8080)
	want := []proxyTarget{
		{label: "File Browser", dialAddr: "10.0.0.5:18888", port: 8090},
		{label: "Battlegroup Director", dialAddr: "10.0.0.5:31003", port: 8091},
		{label: "Wiki", dialAddr: "wiki.example:443", port: 8092},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

// withProxyPorts attaches each entry's assigned proxy port (0 when not proxied)
// so the frontend can build the open-URL from window.location + the port.
func TestWithProxyPorts(t *testing.T) {
	ifaces := []webInterface{
		{Label: "Battlegroup Director", URL: "http://vm:31003/", Target: "10.0.0.5:31003"},
		{Label: "Local", URL: "/grafana"}, // same-origin → not proxied
	}
	targets := resolveProxyTargets(ifaces, 8080) // Director → 8090
	got := withProxyPorts(ifaces, targets)
	if len(got) != 2 {
		t.Fatalf("got %d entries, want 2", len(got))
	}
	if got[0].Label != "Battlegroup Director" || got[0].ProxyPort != 8090 {
		t.Errorf("director = %+v, want proxyPort 8090", got[0])
	}
	if got[1].ProxyPort != 0 {
		t.Errorf("local proxyPort = %d, want 0 (not proxied)", got[1].ProxyPort)
	}
}

// newRootProxy must forward the whole path unchanged so absolute asset paths
// (/Script/…, /static/…) resolve at the proxied service's root.
func TestNewRootProxy_PassesAbsoluteAssetPaths(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = fmt.Fprintf(w, "path=%s", r.URL.Path)
	}))
	defer upstream.Close()
	u, _ := url.Parse(upstream.URL)
	h := newRootProxy(u, net.Dial)

	for _, p := range []string{"/", "/Script/app.js", "/static/css/app.css"} {
		rec := httptest.NewRecorder()
		h(rec, httptest.NewRequest("GET", p, nil))
		if rec.Code != http.StatusOK || rec.Body.String() != "path="+p {
			t.Errorf("%s → %d %q", p, rec.Code, rec.Body.String())
		}
	}
}

// withProxyAuth enforces the dashboard session only when auth is enabled.
func TestWithProxyAuth(t *testing.T) {
	prev := loadedConfig
	t.Cleanup(func() { loadedConfig = prev })

	called := false
	gated := withProxyAuth(func(http.ResponseWriter, *http.Request) { called = true })

	// auth off → passes through
	loadedConfig = appConfig{}
	rec := httptest.NewRecorder()
	gated(rec, httptest.NewRequest("GET", "/", nil))
	if !called {
		t.Errorf("auth off: inner handler should have been called")
	}

	// auth on, no session cookie → blocked with 401
	called = false
	enabled := true
	loadedConfig = appConfig{AuthEnabled: &enabled}
	rec = httptest.NewRecorder()
	gated(rec, httptest.NewRequest("GET", "/", nil))
	if called {
		t.Errorf("auth on: inner handler should have been blocked")
	}
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("auth on: code = %d, want 401", rec.Code)
	}
}

// startWebProxies binds a listener per target synchronously, serves it, and the
// returned stop func shuts them down.
func TestStartWebProxies_StartStop(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprint(w, "ok")
	}))
	defer upstream.Close()
	uu, _ := url.Parse(upstream.URL)

	prev := loadedConfig
	t.Cleanup(func() { loadedConfig = prev })
	loadedConfig = appConfig{} // auth off

	port := freePort(t)
	tgt := proxyTarget{label: "x", dialAddr: uu.Host, port: port}
	stop := startWebProxies([]proxyTarget{tgt}, net.Dial)

	base := fmt.Sprintf("http://127.0.0.1:%d/", port)
	if body := httpGet(t, base); body != "ok" {
		t.Fatalf("proxy body = %q, want ok", body)
	}

	stop()
	// listener closed → a fresh request must fail
	client := &http.Client{Timeout: time.Second}
	if _, err := client.Get(base); err == nil {
		t.Errorf("expected error after stop, got none")
	}
}

func freePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = ln.Close() }()
	return ln.Addr().(*net.TCPAddr).Port
}

// httpGet relies on startWebProxies having bound the listener before returning:
// the connection queues in the listen backlog and is served once Serve accepts,
// so no readiness sleep is needed.
func httpGet(t *testing.T, url string) string {
	t.Helper()
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		t.Fatalf("GET %s: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return string(b)
}
