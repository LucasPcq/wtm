package proxy

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func portOf(t *testing.T, addr string) int {
	t.Helper()
	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		t.Fatal(err)
	}
	n, err := strconv.Atoi(port)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestProbeSeesOurOwnProxy(t *testing.T) {
	addr := serve(t)
	port := portOf(t, addr)

	if got := Probe(ProbeParams{Port: port}); got != port {
		t.Errorf("got %d, want %d", got, port)
	}
}

func TestProbeIgnoresAnotherServer(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	t.Cleanup(other.Close)

	if got := Probe(ProbeParams{Port: portOf(t, other.Listener.Addr().String())}); got != 0 {
		t.Errorf("un serveur sans l'en-tête %s n'est pas le nôtre : got %d", domain.ProxyProbeHeader, got)
	}
}

func TestProbeOnAClosedPortReturnsZero(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := portOf(t, listener.Addr().String())
	listener.Close()

	if got := Probe(ProbeParams{Port: port}); got != 0 {
		t.Errorf("got %d, want 0", got)
	}
}
