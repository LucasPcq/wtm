package events

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

var (
	identities    = worktree.Identities
	ensureDaemon  = process.EnsureDaemon
	replaceDaemon = process.EnsureCurrentDaemon
)

type WatchParams struct {
	ProjectDir string
	StateDir   string
	// SocketPath is the daemon's; empty is the one every command talks to.
	SocketPath string
	// ProxyPort is what a daemon this watcher starts serves names on, as for
	// every other command that starts one: it outlives the watcher's own needs.
	ProxyPort int
	// OnEvent receives the stream in order; an error ends Watch with it.
	OnEvent   func(Received) error
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

// Received is one line of the stream, decoded for a reader that acts on it and
// as it was sent for one that relays it: a newer publisher's fields survive an
// older wtm on their way through.
type Received struct {
	Event domain.Event
	Raw   json.RawMessage
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
	daemon := process.DaemonParams{SocketPath: params.Socket, ProxyPort: params.ProxyPort}
	if err := ensureDaemon(daemon); err != nil {
		return watchResult{transient: err}
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	deliveries, err := process.Subscribe(ctx, process.SubscribeParams{SocketPath: params.Socket, Repos: []string{params.Repo.CommonDir}})
	if errors.Is(err, domain.ErrDaemonNoSubscribe) {
		// Never a daemon another watcher could need: it cannot serve one.
		return watchResult{transient: errors.Join(err, replaceDaemon(daemon))}
	}
	if err != nil {
		return watchResult{transient: err}
	}
	snapshot, err := snapshotOf(snapshotParams{ProjectDir: params.ProjectDir, StateDir: params.StateDir, Repo: params.Repo})
	if err != nil {
		return watchResult{transient: err}
	}
	if err := params.OnEvent(snapshot); err != nil {
		return watchResult{fatal: err}
	}
	ready, err := readyOf()
	if err != nil {
		return watchResult{transient: err}
	}
	if err := params.OnEvent(ready); err != nil {
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
		if err := params.OnEvent(Received{Event: event, Raw: delivery.Payload}); err != nil {
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

// snapshotLine shadows the event's worktrees so a snapshot always carries the
// list, empty included: a consumer resets its state from it.
type snapshotLine struct {
	domain.Event
	Worktrees []domain.WorktreeIdentity `json:"worktrees"`
}

func snapshotOf(params snapshotParams) (Received, error) {
	list, err := identities(worktree.IdentitiesParams{ProjectDir: params.ProjectDir, StateDir: params.StateDir})
	if err != nil {
		return Received{}, err
	}
	if list == nil {
		list = []domain.WorktreeIdentity{}
	}
	event := stamp(stampParams{Event: domain.Event{Type: domain.EventSnapshot, Worktrees: list}, Repo: params.Repo})
	raw, err := json.Marshal(snapshotLine{Event: event, Worktrees: list})
	if err != nil {
		return Received{}, err
	}
	return Received{Event: event, Raw: raw}, nil
}

func readyOf() (Received, error) {
	event := domain.Event{V: domain.EventsSchemaVersion, Type: domain.EventReady, TS: now()}
	raw, err := json.Marshal(event)
	if err != nil {
		return Received{}, err
	}
	return Received{Event: event, Raw: raw}, nil
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
