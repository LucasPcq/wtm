package events

import (
	"context"
	"encoding/json"
	"net"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnEventReachesTheSubscribersOfItsRepositoryAndOfEveryRepository(t *testing.T) {
	socket := serving(t)
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

func TestARelayClosingEndsEverySubscription(t *testing.T) {
	socket := filepath.Join(socketDir(t), "wtm.sock")
	ctx, stop := context.WithCancel(t.Context())
	served := startRelay(ctx, socket)
	require.Eventually(t, Bus{SocketPath: socket}.answers, 2*time.Second, 10*time.Millisecond)
	lines := subscribed(t, socket)

	stop()

	assert.True(t, closed(lines))
	require.NoError(t, <-served)
}

func TestAnUnknownActionIsRefused(t *testing.T) {
	socket := serving(t)
	conn, err := net.Dial("unix", socket)
	require.NoError(t, err)
	defer conn.Close()

	require.NoError(t, json.NewEncoder(conn).Encode(request{Action: "list"}))
	var reply response
	require.NoError(t, json.NewDecoder(conn).Decode(&reply))

	assert.Equal(t, response{Status: statusError, Message: "unknown action: list"}, reply)
}

// A daemon once waited forever on a subscriber that stopped reading, holding its lock while every client waited for it to go.
func TestASubscriberThatStoppedReadingDoesNotHoldTheRelayClosing(t *testing.T) {
	socket := filepath.Join(socketDir(t), "wtm.sock")
	ctx, stop := context.WithCancel(t.Context())
	served := startRelay(ctx, socket)
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
		require.FailNow(t, "the relay did not close")
	}
}
