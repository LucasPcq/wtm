// Package proxy serves the named URLs of the jobs up on one loopback port: a
// reverse proxy that routes by host, and the probe that tells which proxy
// answers behind a port.
package proxy

import (
	"cmp"
	"crypto/rand"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

const (
	// span is how many ports from the one asked for are tried: a name is what a browser types, so the number may move when another program holds it.
	span         = 16
	probeHost    = "probe.wtm.localhost"
	probeHeader  = "X-Wtm-Proxy"
	tokenHeader  = "X-Wtm-Proxy-Token"
	probeTimeout = 150 * time.Millisecond
)

// A Server is the proxy: the names it serves, each to a port on localhost,
// and, once it listens, the port it serves them on. Its methods are safe for
// concurrent use.
type Server struct {
	mu        sync.RWMutex
	addresses map[string]run.Address
	http      *http.Server
	port      int
	// token is what its probe answers besides its port, so an unrelated server on port 80, or a proxy of a daemon before, is never taken for this one. A process of this user may read it and echo it: such a process is outside what it guards against.
	token string
}

// New returns a proxy serving no name, on no port until Listen.
func New() *Server {
	return &Server{addresses: map[string]run.Address{}, token: rand.Text()}
}

// Listen binds the first port from port, within 16, that is free and that a
// browser opens (any free one for 0), and serves on it until Close.
func (s *Server) Listen(port int) error {
	return s.listen(port, span)
}

func (s *Server) listen(port, span int) error {
	var err error
	for at := port; at < port+span; at++ {
		if blocked[at] {
			continue
		}
		var listener net.Listener
		listener, err = net.Listen("tcp", "127.0.0.1:"+strconv.Itoa(at))
		if err != nil {
			continue
		}
		// A connection kept alive with nothing on it goes: the daemon's descriptors are not a browser's to hold. A websocket, hijacked, is no longer the server's.
		server := &http.Server{Handler: http.HandlerFunc(s.route), ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 2 * time.Minute}
		s.mu.Lock()
		s.http = server
		// The listener's, not at: 0 asks for any port, and only it knows which.
		if addr, ok := listener.Addr().(*net.TCPAddr); ok {
			s.port = addr.Port
		}
		s.mu.Unlock()
		go func() { _ = server.Serve(listener) }()
		return nil
	}

	return cmp.Or(err, fmt.Errorf("no port a browser opens from %d to %d", port, port+span-1))
}

// Port returns the port it serves on, 0 before it listens.
func (s *Server) Port() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.port
}

// Close stops serving; a proxy that never listened has nothing to close.
func (s *Server) Close() error {
	s.mu.Lock()
	server := s.http
	s.port = 0
	s.mu.Unlock()
	if server == nil {
		return nil
	}
	return server.Close()
}

// Add serves address, in place of one of the same host.
func (s *Server) Add(address run.Address) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.addresses[strings.ToLower(address.Host)] = address
}

// Remove stops serving address, unless another job took its host over since.
func (s *Server) Remove(address run.Address) {
	s.mu.Lock()
	defer s.mu.Unlock()
	host := strings.ToLower(address.Host)
	if served, found := s.addresses[host]; found && served.Job == address.Job && served.Port == address.Port {
		delete(s.addresses, host)
	}
}

func (s *Server) lookup(host string) (run.Address, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	address, found := s.addresses[strings.ToLower(host)]
	return address, found
}

func (s *Server) list() []run.Address {
	s.mu.RLock()
	defer s.mu.RUnlock()
	addresses := make([]run.Address, 0, len(s.addresses))
	for _, address := range s.addresses {
		addresses = append(addresses, address)
	}
	slices.SortFunc(addresses, func(a, b run.Address) int { return strings.Compare(a.Host, b.Host) })
	return addresses
}

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}
	if host == probeHost {
		w.Header().Set(probeHeader, strconv.Itoa(s.Port()))
		w.Header().Set(tokenHeader, s.token)
		w.WriteHeader(http.StatusNoContent)
		return
	}
	address, found := s.lookup(host)
	if !found {
		s.unknown(w, host)
		return
	}

	// localhost, not 127.0.0.1: a job bound to ::1 alone answers there too.
	target := &url.URL{Scheme: "http", Host: "localhost:" + strconv.Itoa(address.Port)}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			// Rewrite has dropped what the client said of where it came from (Forwarded, X-Forwarded-*): written afresh.
			pr.SetXForwarded()
			pr.SetURL(target)
			// The host the browser asked for is what it scopes cookies to: forwarded as it came, it keeps two worktrees' sessions apart.
			pr.Out.Host = pr.In.Host
		},
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			w.WriteHeader(http.StatusBadGateway)
			fmt.Fprintf(w, "wtm: job %s is published under %s but nothing answers on %s\n", address.Job, address.Host, target.Host)
		},
	}
	proxy.ServeHTTP(w, r)
}

// unknown answers a host it serves nothing under with the names it serves,
// only to a name of this machine: a page of another site whose name it
// rebound to the loopback reaches the proxy too, and must not read them.
func (s *Server) unknown(w http.ResponseWriter, host string) {
	w.WriteHeader(http.StatusNotFound)
	if !local(host) {
		fmt.Fprint(w, "wtm: no job is published under this host\n")
		return
	}
	fmt.Fprintf(w, "wtm: no job is published under %s\n\nRoutes wtm is currently serving:\n", host)
	addresses := s.list()
	if len(addresses) == 0 {
		fmt.Fprint(w, "  (none — start a job that publishes a named URL)\n")
		return
	}
	for _, address := range addresses {
		fmt.Fprintf(w, "  %s  ->  localhost:%d (job %s, worktree %s of %s)\n", address.Host, address.Port, address.Job, address.Worktree, address.Project)
	}
}

// local reports whether host names this machine: localhost, a name under it,
// or a loopback address.
func local(host string) bool {
	host = strings.ToLower(strings.Trim(host, "[]"))
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return host == "localhost" || strings.HasSuffix(host, ".localhost")
}

// Probe returns the port this proxy serves on when it is what answers on
// port, 0 otherwise: what tells a redirection reaching it from one left
// pointing at a port it no longer holds, or from an unrelated server holding
// port.
func (s *Server) Probe(port int) int {
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(port)+"/", nil)
	if err != nil {
		return 0
	}
	req.Host = probeHost
	resp, err := (&http.Client{Timeout: probeTimeout}).Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	served, err := strconv.Atoi(resp.Header.Get(probeHeader))
	if err != nil || resp.Header.Get(tokenHeader) != s.token {
		return 0
	}

	return served
}
