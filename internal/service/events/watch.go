package events

import (
	"context"
	"encoding/json"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

var (
	identities   = worktree.Identities
	ensureDaemon = process.EnsureCurrentDaemon
)

type WatchParams struct {
	ProjectDir string
	StateDir   string
	// SocketPath is the daemon's; empty is the one every command talks to.
	SocketPath string
	// OnEvent receives the stream in order; an error ends Watch with it.
	OnEvent   func(domain.Event) error
	OnWarning func(error)
}

// Watch streams a repository's events until ctx is done: a snapshot, ready,
// then every change. A daemon that goes away is waited for and the stream
// starts over from a fresh snapshot — nothing is replayed, so a consumer
// treats every event as an upsert and every snapshot as a reset.
func Watch(ctx context.Context, params WatchParams) error {
	socket := params.SocketPath
	if socket == "" {
		socket = process.SocketPath()
	}
	repo, err := worktree.RepoOf(worktree.RepoOfParams{ProjectDir: params.ProjectDir})
	if err != nil {
		return err
	}

	backoff := domain.EventsReconnectMin
	for {
		result := watchOnce(ctx, watchOnceParams{WatchParams: params, Socket: socket, Repo: repo})
		if ctx.Err() != nil {
			return nil
		}
		if result.fatal != nil {
			return result.fatal
		}
		if result.transient != nil && params.OnWarning != nil {
			params.OnWarning(result.transient)
		}
		backoff = nextBackoff(nextBackoffParams{Current: backoff, Reset: result.reachedReady})
		if !sleep(ctx, backoff) {
			return nil
		}
	}
}

// watchResult says how one subscription ended: fatal ends Watch (a newer
// schema, a consumer that can no longer write), transient is worth a warning
// before reconnecting, and neither is a daemon that simply went away.
type watchResult struct {
	fatal        error
	transient    error
	reachedReady bool
}

type watchOnceParams struct {
	WatchParams
	Socket string
	Repo   domain.EventRepo
}

// watchOnce subscribes before it reads the snapshot: a change made while the
// snapshot is read waits in the subscription and arrives after ready, where
// replaying it is harmless, rather than falling between the two.
func watchOnce(ctx context.Context, params watchOnceParams) watchResult {
	if err := ensureDaemon(process.DaemonParams{SocketPath: params.Socket}); err != nil {
		return watchResult{transient: err}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	deliveries, err := process.Subscribe(ctx, process.SubscribeParams{SocketPath: params.Socket, Repos: []string{params.Repo.CommonDir}})
	if err != nil {
		return watchResult{transient: err}
	}
	snapshot, err := snapshotEvent(snapshotParams{ProjectDir: params.ProjectDir, StateDir: params.StateDir, Repo: params.Repo})
	if err != nil {
		return watchResult{transient: err}
	}
	if err := params.OnEvent(snapshot); err != nil {
		return watchResult{fatal: err}
	}
	if err := params.OnEvent(readyEvent()); err != nil {
		return watchResult{fatal: err}
	}
	for delivery := range deliveries {
		var event domain.Event
		if json.Unmarshal(delivery.Payload, &event) != nil {
			continue
		}
		if event.V > domain.EventsSchemaVersion {
			return watchResult{fatal: domain.ErrEventsSchemaNewer, reachedReady: true}
		}
		if err := params.OnEvent(event); err != nil {
			return watchResult{fatal: err, reachedReady: true}
		}
	}
	return watchResult{reachedReady: true}
}

type snapshotParams struct {
	ProjectDir string
	StateDir   string
	Repo       domain.EventRepo
}

func snapshotEvent(params snapshotParams) (domain.Event, error) {
	list, err := identities(worktree.IdentitiesParams{ProjectDir: params.ProjectDir, StateDir: params.StateDir})
	if err != nil {
		return domain.Event{}, err
	}
	return stamp(stampParams{Event: domain.Event{Type: domain.EventSnapshot, Worktrees: list}, Repo: params.Repo}), nil
}

func readyEvent() domain.Event {
	return domain.Event{V: domain.EventsSchemaVersion, Type: domain.EventReady, TS: now()}
}

type nextBackoffParams struct {
	Current time.Duration
	Reset   bool
}

func nextBackoff(params nextBackoffParams) time.Duration {
	if params.Reset {
		return domain.EventsReconnectMin
	}
	return min(params.Current*2, domain.EventsReconnectMax)
}

func sleep(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
