package web

import (
	"crypto/subtle"
	"net"
	"net/http"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// HeaderToken is the request header that carries the token of the server's start, required on every request under /api.
const HeaderToken = "X-Wtm-Token"

// loopbackNames are the host names a request may give the server: anything else is a page of another site that resolved its own name to the loopback (DNS rebinding).
var loopbackNames = []string{"127.0.0.1", "localhost"}

// guard refuses a request whose Host is not the loopback at the server's own port, then, under /api, one from a page of another origin and one without the token; it writes the refusal and reports false. No CORS header is ever written: a page of another origin cannot read a response.
func (s *Server) guard(w http.ResponseWriter, r *http.Request) bool {
	if !ownHost(r) {
		writeError(w, http.StatusForbidden, &domain.Error{Kind: domain.KindRefused, Code: CodeHostRefused, Message: "the server answers the loopback at its own port only"})
		return false
	}
	if !strings.HasPrefix(r.URL.Path, "/api/") {
		return true
	}

	if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+r.Host {
		writeError(w, http.StatusForbidden, &domain.Error{Kind: domain.KindRefused, Code: CodeOriginRefused, Message: "a page of another origin may not call the server"})
		return false
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get(HeaderToken)), []byte(s.params.Token)) != 1 {
		writeError(w, http.StatusUnauthorized, &domain.Error{Kind: domain.KindRefused, Code: CodeTokenInvalid, Message: "the request carries no token of this server, or another one: open the page wtm ui prints"})
		return false
	}
	return true
}

// ownHost reports whether r names the server as the loopback, by a name of loopbackNames and the port the connection reached.
func ownHost(r *http.Request) bool {
	name, port, err := net.SplitHostPort(r.Host)
	if err != nil || !slices.Contains(loopbackNames, name) {
		return false
	}

	local, ok := r.Context().Value(http.LocalAddrContextKey).(net.Addr)
	if !ok {
		return false
	}
	_, ownPort, err := net.SplitHostPort(local.String())
	return err == nil && port == ownPort
}
