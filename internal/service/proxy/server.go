package proxy

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

type ServerParams struct {
	Port int
	// Span is how many ports from Port are tried. Zero takes the default.
	Span     int
	Registry *Registry
}

type Server struct {
	registry *Registry
	listener net.Listener
	http     *http.Server
	port     int
	span     int
}

func NewServer(params ServerParams) *Server {
	span := params.Span
	if span <= 0 {
		span = domain.ProxyPortScanSpan
	}
	return &Server{registry: params.Registry, port: params.Port, span: span}
}

// Start binds the first usable port from the one asked for and serves until
// Close. Usable means free *and* one a browser will open: a port on the
// browsers' blocked list serves nothing however well it binds, so it is stepped
// over exactly like an occupied one. The port is not stable across restarts when the preferred one is
// taken, and that is the trade this makes: a caller reaches a job by name, so
// the number is a transport detail `run url` resolves for them — where a job's
// own port is one they memorise, which is why those never move.
func (s *Server) Start() error {
	var err error
	for port := s.port; port < s.port+s.span; port++ {
		if rules.IsBrowserBlockedPort(port) {
			continue
		}
		var listener net.Listener
		listener, err = net.Listen("tcp", fmt.Sprintf(domain.ProxyLoopbackFmt, port))
		if err != nil {
			continue
		}
		s.listener = listener
		// Not `port`: a zero asks for an ephemeral one, and the listener is the
		// only thing that knows which.
		s.port = port
		if addr, ok := listener.Addr().(*net.TCPAddr); ok {
			s.port = addr.Port
		}
		s.http = &http.Server{Handler: http.HandlerFunc(s.route)}

		go func() { _ = s.http.Serve(listener) }()
		return nil
	}
	return err
}

// Port is the port the listener actually took, zero before Start.
func (s *Server) Port() int {
	if s.listener == nil {
		return 0
	}
	return s.port
}

func (s *Server) Close() error {
	if s.http == nil {
		return nil
	}
	return s.http.Close()
}

func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	host := r.Host
	if h, _, err := net.SplitHostPort(host); err == nil {
		host = h
	}

	if host == domain.ProxyProbeHost {
		w.Header().Set(domain.ProxyProbeHeader, strconv.Itoa(s.port))
		w.WriteHeader(http.StatusNoContent)
		return
	}

	route, found := s.registry.Lookup(host)
	if !found {
		s.writeUnknownHost(w, host)
		return
	}

	rp := &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetXForwarded()
			pr.SetURL(&url.URL{Scheme: domain.ProxyScheme, Host: route.Target})
			// The inbound Host is what the browser scopes cookies to; forwarding
			// it unchanged is the isolation this whole feature exists for.
			pr.Out.Host = pr.In.Host
		},
		ErrorHandler: func(ew http.ResponseWriter, _ *http.Request, _ error) {
			writeSilentTarget(ew, route)
		},
	}
	rp.ServeHTTP(w, r)
}

func (s *Server) writeUnknownHost(w http.ResponseWriter, host string) {
	w.WriteHeader(http.StatusNotFound)
	fmt.Fprintf(w, domain.ProxyUnknownHostFmt, host)

	fmt.Fprint(w, domain.ProxyKnownRoutesHead)
	routes := s.registry.List()
	if len(routes) == 0 {
		fmt.Fprint(w, domain.ProxyNoRoutesLine)
		return
	}
	for _, route := range routes {
		fmt.Fprintf(w, domain.ProxyRouteLineFmt, route.Host, route.Target, route.Job, route.Worktree, route.Project)
	}
}

func writeSilentTarget(w http.ResponseWriter, route domain.ProxyRoute) {
	w.WriteHeader(http.StatusBadGateway)
	fmt.Fprintf(w, domain.ProxySilentTargetFmt, route.Job, route.Host, route.Target)
}
