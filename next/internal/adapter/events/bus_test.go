package events

import (
	"context"
	"errors"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubscribingStartsTheDaemonWhenNoneAnswers(t *testing.T) {
	socket := filepath.Join(socketDir(t), "wtm.sock")
	ctx, cancel := context.WithCancel(context.Background())
	var served <-chan error
	t.Cleanup(func() {
		cancel()
		<-served
	})
	bus := Bus{SocketPath: socket, Start: func() error {
		served = startServe(ctx, ServeParams{SocketPath: socket, Idle: time.Minute})
		return nil
	}}

	lines, err := bus.Subscribe(t.Context(), nil)
	require.NoError(t, err)
	NewPublisher(PublisherParams{Root: "/repo", CommonDir: "/repo/.git", SocketPath: socket, Now: fixedNow}).Publish(domain.RepoAdded{})

	assert.JSONEq(t, `{"v":2,"type":"repo.added","ts":"2026-10-16T12:00:00Z","repo":{"root":"/repo","common_dir":"/repo/.git"}}`, received(t, lines))
}

func TestSubscribingWithNoDaemonAndNoWayToStartOneFails(t *testing.T) {
	_, err := Bus{SocketPath: filepath.Join(socketDir(t), "wtm.sock")}.Subscribe(t.Context(), nil)

	assert.ErrorContains(t, err, "no event bus answers")
}

func TestADaemonThatFailsToStartFailsTheSubscription(t *testing.T) {
	bus := Bus{SocketPath: filepath.Join(socketDir(t), "wtm.sock"), Start: func() error { return errors.New("no binary") }}

	_, err := bus.Subscribe(t.Context(), nil)

	assert.ErrorContains(t, err, "start the event bus: no binary")
}

func TestASubscriptionEndsWithItsContext(t *testing.T) {
	socket, _ := serving(t, time.Minute)
	ctx, cancel := context.WithCancel(t.Context())
	lines, err := Bus{SocketPath: socket}.Subscribe(ctx, nil)
	require.NoError(t, err)

	cancel()

	assert.True(t, closed(lines))
}

func TestSubscribingWaitsForADaemonThatHoldsTheLockWithoutAnsweringToBeGone(t *testing.T) {
	socket := filepath.Join(socketDir(t), "wtm.sock")
	stopping, err := statefile.TryLock(lockPath(socket), domain.LockExclusive)
	require.NoError(t, err)
	var gone atomic.Bool
	time.AfterFunc(300*time.Millisecond, func() {
		gone.Store(true)
		_ = stopping.Close()
	})
	ctx, cancel := context.WithCancel(context.Background())
	var served <-chan error
	t.Cleanup(func() {
		cancel()
		if served != nil {
			<-served
		}
	})
	startedAfterIt := false
	bus := Bus{SocketPath: socket, Start: func() error {
		startedAfterIt = gone.Load()
		served = startServe(ctx, ServeParams{SocketPath: socket, Idle: time.Minute})
		return nil
	}}

	_, err = bus.Subscribe(t.Context(), nil)

	require.NoError(t, err)
	assert.True(t, startedAfterIt, "a daemon was started while the stopping one still held the lock")
}
