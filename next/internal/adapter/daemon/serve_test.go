package daemon_test

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/daemon"
	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheSocketIsTheOneWtmsDaemonListensOn(t *testing.T) {
	path := daemon.SocketPath()

	assert.Equal(t, "wtm.sock", filepath.Base(path))
	assert.Equal(t, "wtm", filepath.Base(filepath.Dir(path)))
}

func TestTheDaemonIdlesOutWithNoRequest(t *testing.T) {
	socket := filepath.Join(socketDir(t), "wtm.sock")

	served := startServe(t.Context(), daemon.ServeParams{SocketPath: socket, Idle: 100 * time.Millisecond})

	select {
	case err := <-served:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		require.FailNow(t, "the daemon did not idle out")
	}
	assert.NoFileExists(t, socket)
}

func TestADaemonBusyNeverIdlesOutAndStopsWhatItRunsBeforeItEnds(t *testing.T) {
	socket := filepath.Join(socketDir(t), "wtm.sock")
	var busy, stopped, ready atomic.Bool
	busy.Store(true)

	served := startServe(t.Context(), daemon.ServeParams{SocketPath: socket, Idle: 50 * time.Millisecond, Busy: busy.Load, Ready: func() { ready.Store(true) }, Stopping: func() { stopped.Store(true) }})
	require.Eventually(t, func() bool { return dials(socket) }, 2*time.Second, 10*time.Millisecond)
	time.Sleep(300 * time.Millisecond)

	require.True(t, ready.Load())
	require.Empty(t, served, "the daemon idled out while busy")
	busy.Store(false)
	select {
	case err := <-served:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		require.FailNow(t, "the daemon did not idle out once no longer busy")
	}
	assert.True(t, stopped.Load(), "what it runs was not stopped")
}

func TestOnlyItsOwnerMayConnectToTheDaemon(t *testing.T) {
	socket, _ := serving(t, daemon.ServeParams{Idle: time.Minute})

	info, err := os.Stat(socket)

	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o600), info.Mode().Perm())
}

func TestARequestInFlightKeepsTheDaemonUpAndItsEndLetsItIdleOut(t *testing.T) {
	held := make(chan struct{})
	socket, served := serving(t, daemon.ServeParams{Idle: 100 * time.Millisecond, Bus: func(_ context.Context, _ net.Conn, _ []byte) { <-held }})
	conn, err := net.Dial("unix", socket)
	require.NoError(t, err)
	defer conn.Close()
	_, err = conn.Write([]byte(`{"action":"subscribe"}` + "\n"))
	require.NoError(t, err)

	select {
	case <-served:
		require.FailNow(t, "the daemon idled out under a request")
	case <-time.After(400 * time.Millisecond):
	}
	close(held)
	select {
	case err := <-served:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		require.FailNow(t, "the daemon outlived its last request")
	}
}

func TestTheBusAnswersItsActionsAndAnswerEveryOther(t *testing.T) {
	bus, other := make(chan string, 4), make(chan string, 4)
	socket, _ := serving(t, daemon.ServeParams{
		Idle:   time.Minute,
		Bus:    func(_ context.Context, _ net.Conn, line []byte) { bus <- string(line) },
		Answer: func(_ net.Conn, line []byte, _ func()) { other <- string(line) },
	})

	for _, line := range []string{`{"action":"publish"}`, `{"action":"subscribe"}`, `{"action":"list"}`, `not json`} {
		conn, err := net.Dial("unix", socket)
		require.NoError(t, err)
		_, err = conn.Write([]byte(line + "\n"))
		require.NoError(t, err)
		_ = conn.Close()
	}

	assert.ElementsMatch(t, []string{`{"action":"publish"}` + "\n", `{"action":"subscribe"}` + "\n"}, drain(t, bus, 2))
	assert.ElementsMatch(t, []string{`{"action":"list"}` + "\n", "not json\n"}, drain(t, other, 2))
}

func TestOneDaemonAtATimeServesASocket(t *testing.T) {
	socket, _ := serving(t, daemon.ServeParams{Idle: time.Minute})

	err := daemon.Serve(t.Context(), daemon.ServeParams{SocketPath: socket, Idle: time.Minute})

	assert.ErrorIs(t, err, daemon.ErrRunning)
	assert.True(t, dials(socket), "the second daemon took the socket of the first")
}

// A client probing whether a daemon holds the lock takes it for an instant: a daemon starting then once gave up.
func TestADaemonStartingWaitsForALockHeldForAnInstant(t *testing.T) {
	socket := filepath.Join(socketDir(t), "wtm.sock")
	probe, err := statefile.TryLock(filepath.Join(filepath.Dir(socket), "wtm.lock"), domain.LockExclusive)
	require.NoError(t, err)
	time.AfterFunc(50*time.Millisecond, func() { _ = probe.Close() })

	ctx, stop := context.WithCancel(t.Context())
	served := startServe(ctx, daemon.ServeParams{SocketPath: socket, Idle: time.Minute})
	t.Cleanup(func() {
		stop()
		<-served
	})

	assert.Eventually(t, func() bool { return dials(socket) }, 2*time.Second, 10*time.Millisecond)
}
