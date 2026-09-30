package portprobe

import (
	"context"
	"net"
	"testing"
	"time"
)

func listenOnAFreePort(t *testing.T) (port int, close func()) {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	return l.Addr().(*net.TCPAddr).Port, func() { l.Close() }
}

func freePort(t *testing.T) int {
	t.Helper()
	port, close := listenOnAFreePort(t)
	close()
	return port
}

func TestPollFindsAListener(t *testing.T) {
	port, closeListener := listenOnAFreePort(t)
	defer closeListener()

	listening := Poll(context.Background(), PollParams{
		Ports:   []int{port},
		Budget:  2 * time.Second,
		Settled: func(map[int]bool) bool { return false },
	})

	if !listening[port] {
		t.Errorf("expected %d to answer", port)
	}
}

func TestPollReportsASilentPort(t *testing.T) {
	port := freePort(t)

	start := time.Now()
	listening := Poll(context.Background(), PollParams{
		Ports:   []int{port},
		Budget:  300 * time.Millisecond,
		Settled: func(map[int]bool) bool { return false },
	})

	if listening[port] {
		t.Errorf("nothing listens on %d, it must not answer", port)
	}
	if elapsed := time.Since(start); elapsed < 300*time.Millisecond {
		t.Errorf("a silent port must consume the budget, returned after %s", elapsed)
	}
}

func TestPollStopsEarlyWhenSettled(t *testing.T) {
	port, closeListener := listenOnAFreePort(t)
	defer closeListener()

	start := time.Now()
	Poll(context.Background(), PollParams{
		Ports:  []int{port},
		Budget: 10 * time.Second,
		// A healthy stack must not spend its whole budget: this is what keeps a
		// generous default free.
		Settled: func(listening map[int]bool) bool { return listening[port] },
	})

	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Errorf("expected an early return once settled, took %s", elapsed)
	}
}

func TestPollSeparatesListeningFromSilent(t *testing.T) {
	up, closeListener := listenOnAFreePort(t)
	defer closeListener()
	down := freePort(t)

	listening := Poll(context.Background(), PollParams{
		Ports:   []int{up, down},
		Budget:  300 * time.Millisecond,
		Settled: func(map[int]bool) bool { return false },
	})

	if !listening[up] {
		t.Errorf("%d listens and must be reported", up)
	}
	if listening[down] {
		t.Errorf("%d is free and must not be reported", down)
	}
}

func TestPollHonoursCancellation(t *testing.T) {
	port := freePort(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	Poll(ctx, PollParams{
		Ports:   []int{port},
		Budget:  10 * time.Second,
		Settled: func(map[int]bool) bool { return false },
	})

	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("a cancelled probe must return at once, took %s", elapsed)
	}
}

func TestPollWithNoPortsAsksNothing(t *testing.T) {
	listening := Poll(context.Background(), PollParams{
		Ports:   nil,
		Budget:  10 * time.Second,
		Settled: func(map[int]bool) bool { return false },
	})
	if len(listening) != 0 {
		t.Errorf("nothing to probe, got %v", listening)
	}
}
