package portprobe_test

import (
	"net"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/portprobe"

	"github.com/stretchr/testify/assert"
)

func TestAPortSomethingListensOnAnswersAndAPortHeldUnlistenedDoesNot(t *testing.T) {
	listener := listening(t, "tcp4", "127.0.0.1:0")
	listened := listener.Addr().(*net.TCPAddr).Port
	held, _ := heldUnlistened(t)

	got := portprobe.Listening(t.Context(), []int{listened, held}, 300*time.Millisecond)

	assert.Equal(t, map[int]bool{listened: true}, got)
}

// A dev server on localhost may listen on ::1 alone, as Vite and Node do on macOS.
func TestAPortListenedOnTheIPv6LoopbackAloneAnswers(t *testing.T) {
	listener, err := net.Listen("tcp6", "[::1]:0")
	if err != nil {
		t.Skip("no IPv6 loopback:", err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	port := listener.Addr().(*net.TCPAddr).Port

	got := portprobe.Listening(t.Context(), []int{port}, time.Second)

	assert.Equal(t, map[int]bool{port: true}, got)
}

func TestAPortListenedOnWhileItWaitsAnswers(t *testing.T) {
	port, listen := heldUnlistened(t)
	listened := make(chan error, 1)
	go func() {
		time.Sleep(300 * time.Millisecond)
		listened <- listen()
	}()

	got := portprobe.Listening(t.Context(), []int{port}, 3*time.Second)

	assert.NoError(t, <-listened)
	assert.Equal(t, map[int]bool{port: true}, got)
}
