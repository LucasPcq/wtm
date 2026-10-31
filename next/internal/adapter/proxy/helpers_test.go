package proxy

import (
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/require"
)

// serve starts a proxy on any free port serving addresses, and returns it.
func serve(t *testing.T, addresses ...run.Address) *Server {
	t.Helper()
	server := New()
	require.NoError(t, server.Listen(0))
	t.Cleanup(func() { _ = server.Close() })
	for _, address := range addresses {
		server.Add(address)
	}
	return server
}

// get asks server for host, as a browser resolving *.localhost to the loopback does, and returns the answer and its body.
func get(t *testing.T, server *Server, host string) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1:"+strconv.Itoa(server.Port())+"/", nil)
	require.NoError(t, err)
	req.Host = host
	resp, err := http.DefaultTransport.RoundTrip(req)
	require.NoError(t, err)
	t.Cleanup(func() { _ = resp.Body.Close() })
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, string(body)
}

// backend starts an HTTP server answering with handler, and returns the port it answers on.
func backend(t *testing.T, handler http.HandlerFunc) int {
	t.Helper()
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return portOf(t, server.Listener.Addr())
}

func portOf(t *testing.T, addr net.Addr) int {
	t.Helper()
	tcp, ok := addr.(*net.TCPAddr)
	require.True(t, ok)
	return tcp.Port
}

// taken holds a port on the loopback for the test, and returns it.
func taken(t *testing.T) int {
	t.Helper()
	busy, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = busy.Close() })
	return portOf(t, busy.Addr())
}

// closed returns a port on the loopback nothing listens on.
func closed(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := portOf(t, listener.Addr())
	require.NoError(t, listener.Close())
	return port
}
