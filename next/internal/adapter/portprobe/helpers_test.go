package portprobe_test

import (
	"net"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

func listening(t *testing.T, network, address string) net.Listener {
	t.Helper()
	listener, err := net.Listen(network, address)
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	return listener
}

// heldUnlistened binds a port of the IPv4 loopback without listening on it, so nothing answers there and nobody else takes it, until listen; both closed once the test ends.
func heldUnlistened(t *testing.T) (port int, listen func() error) {
	t.Helper()
	fd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	require.NoError(t, err)
	t.Cleanup(func() { _ = syscall.Close(fd) })
	require.NoError(t, syscall.Bind(fd, &syscall.SockaddrInet4{Addr: [4]byte{127, 0, 0, 1}}))
	bound, err := syscall.Getsockname(fd)
	require.NoError(t, err)
	inet, ok := bound.(*syscall.SockaddrInet4)
	require.True(t, ok)
	return inet.Port, func() error { return syscall.Listen(fd, 1) }
}
