package events

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnEventReachesTheSubscribersOfItsRepositoryAndOfEveryRepository(t *testing.T) {
	socket, _ := serving(t, time.Minute)
	ofA, ofB, ofAll := subscribed(t, socket, "/a/.git"), subscribed(t, socket, "/b/.git"), subscribed(t, socket)

	publish(t, socket, "/a/.git", `{"n":1}`)
	publish(t, socket, "/b/.git", `{"n":2}`)

	assert.JSONEq(t, `{"n":1}`, received(t, ofA))
	assert.JSONEq(t, `{"n":2}`, received(t, ofB))
	assert.JSONEq(t, `{"n":1}`, received(t, ofAll))
	assert.JSONEq(t, `{"n":2}`, received(t, ofAll))
}

func TestASubscriberWhoseQueueIsFullIsCutOffWithoutHoldingThePublisher(t *testing.T) {
	h := newHub(2)
	slow, _ := h.subscribe(nil)
	fast, _ := h.subscribe(nil)

	done := make(chan struct{})
	go func() {
		for range 3 {
			h.publish(delivery{repo: "/a/.git", payload: json.RawMessage(`{}`)})
			<-fast
		}
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		require.FailNow(t, "the publisher waited on a full queue")
	}
	assert.Len(t, slow, 2)
	<-slow
	<-slow
	_, open := <-slow
	assert.False(t, open, "the slow subscriber is still subscribed")
}

func TestTheDaemonIdlesOutWithNoSubscriberAndNoRequest(t *testing.T) {
	socket := filepath.Join(socketDir(t), "wtm.sock")

	served := startServe(t.Context(), ServeParams{SocketPath: socket, Idle: 100 * time.Millisecond})

	select {
	case err := <-served:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		require.FailNow(t, "the daemon did not idle out")
	}
	assert.NoFileExists(t, socket)
}

func TestADaemonBusyNeverIdlesOutAndStopsWhatItHostsBeforeItEnds(t *testing.T) {
	socket := filepath.Join(socketDir(t), "wtm.sock")
	var busy, stopped, ready atomic.Bool
	busy.Store(true)

	served := startServe(t.Context(), ServeParams{SocketPath: socket, Idle: 50 * time.Millisecond, Busy: busy.Load, Ready: func() { ready.Store(true) }, Stopping: func() { stopped.Store(true) }})
	require.Eventually(t, Bus{SocketPath: socket}.answers, 2*time.Second, 10*time.Millisecond)
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
	assert.True(t, stopped.Load(), "what it hosts was not stopped")
}

func TestASubscriberKeepsTheDaemonUp(t *testing.T) {
	socket, served := serving(t, 100*time.Millisecond)
	subscribed(t, socket)

	select {
	case <-served:
		require.FailNow(t, "the daemon idled out under a subscriber")
	case <-time.After(400 * time.Millisecond):
	}
}

func TestStoppingTheDaemonEndsEverySubscription(t *testing.T) {
	socket := filepath.Join(socketDir(t), "wtm.sock")
	ctx, stop := context.WithCancel(t.Context())
	served := startServe(ctx, ServeParams{SocketPath: socket, Idle: time.Minute})
	require.Eventually(t, Bus{SocketPath: socket}.answers, 2*time.Second, 10*time.Millisecond)
	lines := subscribed(t, socket)

	stop()

	assert.True(t, closed(lines))
	require.NoError(t, <-served)
}

func TestOneDaemonAtATimeServesASocket(t *testing.T) {
	socket, _ := serving(t, time.Minute)

	err := Serve(t.Context(), ServeParams{SocketPath: socket, Idle: time.Minute})

	assert.ErrorIs(t, err, ErrDaemonRunning)
	assert.True(t, Bus{SocketPath: socket}.answers(), "the second daemon took the socket of the first")
}

func TestAnUnknownActionIsRefused(t *testing.T) {
	socket, _ := serving(t, time.Minute)
	conn, err := net.Dial("unix", socket)
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, json.NewEncoder(conn).Encode(request{Action: "list"}))
	var reply response
	require.NoError(t, json.NewDecoder(conn).Decode(&reply))

	assert.Equal(t, response{Status: statusError, Message: "unknown action: list"}, reply)
}

func TestASubscriberLeavingLetsTheDaemonIdleOut(t *testing.T) {
	socket := filepath.Join(socketDir(t), "wtm.sock")
	served := startServe(t.Context(), ServeParams{SocketPath: socket, Idle: 100 * time.Millisecond})
	require.Eventually(t, Bus{SocketPath: socket}.answers, 2*time.Second, 10*time.Millisecond)
	ctx, leave := context.WithCancel(t.Context())
	_, err := Bus{SocketPath: socket}.Subscribe(ctx, nil)
	require.NoError(t, err)

	leave()

	select {
	case err := <-served:
		require.NoError(t, err)
	case <-time.After(2 * time.Second):
		require.FailNow(t, "the daemon outlived its last subscriber")
	}
}

// A daemon once waited forever on a subscriber that stopped reading, holding its lock while every client waited for it to go.
func TestASubscriberThatStoppedReadingDoesNotHoldTheDaemonStopping(t *testing.T) {
	socket := filepath.Join(socketDir(t), "wtm.sock")
	ctx, stop := context.WithCancel(t.Context())
	served := startServe(ctx, ServeParams{SocketPath: socket, Idle: time.Minute})
	require.Eventually(t, Bus{SocketPath: socket}.answers, 2*time.Second, 10*time.Millisecond)
	stalled, err := net.Dial("unix", socket)
	require.NoError(t, err)
	defer stalled.Close()
	require.NoError(t, json.NewEncoder(stalled).Encode(request{Action: actionSubscribe}))
	var ack response
	require.NoError(t, json.NewDecoder(stalled).Decode(&ack))
	payload := `"` + strings.Repeat("x", 8192) + `"`
	for range 64 {
		publish(t, socket, "/a/.git", payload)
	}

	stop()

	select {
	case err := <-served:
		require.NoError(t, err)
	case <-time.After(5 * time.Second):
		require.FailNow(t, "the daemon did not stop")
	}
}

// A client probing whether a daemon holds the lock takes it for an instant: a daemon starting then once gave up.
func TestADaemonStartingWaitsForALockHeldForAnInstant(t *testing.T) {
	socket := filepath.Join(socketDir(t), "wtm.sock")
	probe, err := statefile.TryLock(lockPath(socket), domain.LockExclusive)
	require.NoError(t, err)
	time.AfterFunc(50*time.Millisecond, func() { _ = probe.Close() })

	ctx, stop := context.WithCancel(t.Context())
	served := startServe(ctx, ServeParams{SocketPath: socket, Idle: time.Minute})
	t.Cleanup(func() {
		stop()
		<-served
	})

	assert.Eventually(t, Bus{SocketPath: socket}.answers, 2*time.Second, 10*time.Millisecond)
}
