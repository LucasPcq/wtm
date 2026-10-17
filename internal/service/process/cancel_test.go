package process

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

const promptly = time.Second

func cancelledSoon(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(100*time.Millisecond, cancel)
	return ctx
}

// An interrupt while the daemon is coming up ends the wait there: the daemon
// itself is left to start, and only this command gives up on it.
func TestEnsureDaemonGivesUpWhenCancelled(t *testing.T) {
	socket := skewSocket(t)
	previous := spawnDaemon
	spawnDaemon = func(DaemonParams) error { return nil }
	t.Cleanup(func() { spawnDaemon = previous })

	begin := time.Now()
	err := EnsureDaemon(cancelledSoon(t), DaemonParams{SocketPath: socket})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("EnsureDaemon = %v, want the cancellation", err)
	}
	if elapsed := time.Since(begin); elapsed > promptly {
		t.Fatalf("EnsureDaemon gave up after %v", elapsed)
	}
}

func TestAwaitDaemonStoppedGivesUpWhenCancelled(t *testing.T) {
	socket := skewSocket(t)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	begin := time.Now()
	err = AwaitDaemonStopped(cancelledSoon(t), socket)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("AwaitDaemonStopped = %v, want the cancellation", err)
	}
	if elapsed := time.Since(begin); elapsed > promptly {
		t.Fatalf("AwaitDaemonStopped gave up after %v", elapsed)
	}
}

// A request in flight ends on its own socket: the daemon is never signalled,
// and the job it is running is untouched.
func TestSendGivesUpWhenCancelled(t *testing.T) {
	socket := skewSocket(t)
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		<-t.Context().Done()
	}()

	begin := time.Now()
	_, err = NewClient(socket).Send(cancelledSoon(t), Request{Action: ActionList})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Send = %v, want the cancellation", err)
	}
	if elapsed := time.Since(begin); elapsed > promptly {
		t.Fatalf("Send gave up after %v", elapsed)
	}
}
