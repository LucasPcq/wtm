package proxy

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// serve starts a proxy on a free port with routes already registered, and
// returns the address to dial.
func serve(t *testing.T, routes ...domain.ProxyRoute) string {
	t.Helper()

	registry := NewRegistry()
	for _, route := range routes {
		registry.Add(route)
	}
	server := NewServer(ServerParams{Port: 0, Registry: registry})
	if err := server.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	t.Cleanup(func() { _ = server.Close() })
	return server.listener.Addr().String()
}

// get dials addr but asks for host, which is what a browser resolving
// *.localhost to the loopback does.
func get(t *testing.T, addr, host string) *http.Response {
	t.Helper()

	req, err := http.NewRequest(http.MethodGet, "http://"+addr+"/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Host = host

	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		t.Fatalf("roundtrip: %v", err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestServerPassesTheHostThrough(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, r.Host)
	}))
	t.Cleanup(backend.Close)

	target := mustHost(t, backend.URL)
	addr := serve(t, domain.ProxyRoute{Host: "web.feat.myapp.localhost", Target: target, Job: "web"})

	resp := get(t, addr, "web.feat.myapp.localhost")
	body, _ := io.ReadAll(resp.Body)

	// Rewriting the Host would put every worktree back in one cookie jar, which
	// is the bug this whole feature exists to fix.
	if string(body) != "web.feat.myapp.localhost" {
		t.Errorf("backend saw Host %q, want it untouched", body)
	}
}

func TestServerSetsTheForwardedHeaders(t *testing.T) {
	seen := ""
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Get("X-Forwarded-Host")
	}))
	t.Cleanup(backend.Close)

	addr := serve(t, domain.ProxyRoute{Host: "web.feat.myapp.localhost", Target: mustHost(t, backend.URL), Job: "web"})
	get(t, addr, "web.feat.myapp.localhost")

	if seen != "web.feat.myapp.localhost" {
		t.Errorf("X-Forwarded-Host = %q, want the name the browser asked for", seen)
	}
}

func TestServerUpgradesWebSockets(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Upgrade") != "websocket" {
			t.Errorf("upgrade header lost, headers = %v", r.Header)
		}
		conn, _, err := w.(http.Hijacker).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer conn.Close()
		_, _ = conn.Write([]byte("HTTP/1.1 101 Switching Protocols\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\nping"))
	}))
	t.Cleanup(backend.Close)

	addr := serve(t, domain.ProxyRoute{Host: "web.feat.myapp.localhost", Target: mustHost(t, backend.URL), Job: "web"})

	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	_, _ = conn.Write([]byte("GET / HTTP/1.1\r\nHost: web.feat.myapp.localhost\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n\r\n"))

	reader := bufio.NewReader(conn)
	status, err := reader.ReadString('\n')
	if err != nil {
		t.Fatalf("read status: %v", err)
	}
	if !strings.Contains(status, "101") {
		t.Fatalf("status = %q, want 101 Switching Protocols", status)
	}
}

func TestServerListsRoutesForAnUnknownHost(t *testing.T) {
	addr := serve(t, domain.ProxyRoute{Host: "web.feat.myapp.localhost", Target: "127.0.0.1:1", Job: "web", Worktree: "feat"})

	resp := get(t, addr, "web.typo.myapp.localhost")
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404", resp.StatusCode)
	}
	// Landing on the list of routes that exist beats ERR_CONNECTION_REFUSED.
	if !strings.Contains(string(body), "web.feat.myapp.localhost") {
		t.Errorf("body = %q, want the known routes listed", body)
	}
}

func TestServerReportsASilentTarget(t *testing.T) {
	// Port 1 on the loopback: registered, nothing listening.
	addr := serve(t, domain.ProxyRoute{Host: "web.feat.myapp.localhost", Target: "127.0.0.1:1", Job: "web"})

	resp := get(t, addr, "web.feat.myapp.localhost")
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", resp.StatusCode)
	}
	if !strings.Contains(string(body), "web") {
		t.Errorf("body = %q, want the job named", body)
	}
}

func mustHost(t *testing.T, raw string) string {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u.Host
}

// A dev server that binds the IPv6 loopback only — Vite does — must still be
// reachable, which is why a route names its target by host and not by 127.0.0.1.
func TestServerReachesATargetBoundToIPv6Only(t *testing.T) {
	listener, err := net.Listen("tcp", "[::1]:0")
	if err != nil {
		t.Skipf("no IPv6 loopback on this machine: %v", err)
	}
	backend := &httptest.Server{
		Listener: listener,
		Config:   &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, "v6") })},
	}
	backend.Start()
	t.Cleanup(backend.Close)

	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	target := fmt.Sprintf(domain.ProxyTargetFmt, mustAtoi(t, port))
	addr := serve(t, domain.ProxyRoute{Host: "web.feat.myapp.localhost", Target: target, Job: "web"})

	resp := get(t, addr, "web.feat.myapp.localhost")
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK || string(body) != "v6" {
		t.Errorf("status %d, body %q — want the IPv6-only backend reached", resp.StatusCode, body)
	}
}

func mustAtoi(t *testing.T, s string) int {
	t.Helper()
	n, err := strconv.Atoi(s)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

// Un port occupé par un tiers coûtait toute la fonctionnalité. Le repli la
// garde : l'utilisateur tape un nom, pas un numéro, donc le port n'a pas à
// être stable pour que l'URL réponde.
func TestServerStartSeReplieSurLePortSuivant(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occuper un port: %v", err)
	}
	defer busy.Close()
	taken := busy.Addr().(*net.TCPAddr).Port

	server := NewServer(ServerParams{Port: taken, Registry: NewRegistry()})
	if err := server.Start(); err != nil {
		t.Fatalf("Start = %v, want un repli réussi", err)
	}
	defer server.Close()

	if server.Port() == taken {
		t.Errorf("port = %d, want un port différent de celui occupé", server.Port())
	}
	if server.Port() == 0 {
		t.Error("port = 0, want le port réellement pris")
	}
}

// Passé la fenêtre, l'échec reste un échec : mieux vaut le dire que servir sur
// un port arbitraire très loin de celui qui a été configuré.
//
// Span vaut 1, donc la seule tentative possible est le port occupé — le test
// ne dépend pas de ce qui est libre autour de lui.
func TestServerStartAbandonneApresLaFenetre(t *testing.T) {
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("occuper un port: %v", err)
	}
	defer busy.Close()
	taken := busy.Addr().(*net.TCPAddr).Port

	server := NewServer(ServerParams{Port: taken, Span: 1, Registry: NewRegistry()})
	if err := server.Start(); err == nil {
		server.Close()
		t.Error("Start = nil, want une erreur quand la fenêtre est épuisée")
	}
}

func TestServerAnswersTheProbeWithItsBindPort(t *testing.T) {
	addr := serve(t)

	resp := get(t, addr, domain.ProxyProbeHost)

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("status: got %d, want %d", resp.StatusCode, http.StatusNoContent)
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	if got := resp.Header.Get(domain.ProxyProbeHeader); got != port {
		t.Errorf("header: got %q, want %q", got, port)
	}
}

func TestServerProbeIgnoresTheRegistry(t *testing.T) {
	addr := serve(t, domain.ProxyRoute{Host: domain.ProxyProbeHost, Target: "127.0.0.1:1", Job: "web"})

	resp := get(t, addr, domain.ProxyProbeHost)

	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("une route enregistrée sous l'hôte sentinelle ne doit pas être servie : got %d", resp.StatusCode)
	}
}
