package portprobe_test

import (
	"net"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/portprobe"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAPortSomethingListensOnAnswersAndAFreeOneDoesNot(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { _ = listener.Close() })
	bound := listener.Addr().(*net.TCPAddr).Port
	free := freePort(t)

	got := portprobe.Listening(t.Context(), []int{bound, free}, 300*time.Millisecond)

	assert.Equal(t, map[int]bool{bound: true}, got)
}

func TestAPortBoundWhileItWaitsAnswers(t *testing.T) {
	port := freePort(t)
	go func() {
		time.Sleep(300 * time.Millisecond)
		listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", itoa(port)))
		if err == nil {
			t.Cleanup(func() { _ = listener.Close() })
		}
	}()

	got := portprobe.Listening(t.Context(), []int{port}, 3*time.Second)

	assert.Equal(t, map[int]bool{port: true}, got)
}
