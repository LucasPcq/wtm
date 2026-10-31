package portprobe_test

import (
	"net"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"
)

// freePort is a port nothing listens on once it returns.
func freePort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

func itoa(port int) string { return strconv.Itoa(port) }
