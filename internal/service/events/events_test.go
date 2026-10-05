package events

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/process/processtest"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
	"github.com/LucasPcq/wtm/internal/testutil/socktest"
)

type watchFixture struct {
	projectDir string
	stateDir   string
	socket     string
	stop       func()
}

func newWatchFixture(t *testing.T) watchFixture {
	t.Helper()
	dir := gittest.InitRepo(t)
	gittest.Git(t, dir, "worktree", "add", "-b", "feat/a", filepath.Join(t.TempDir(), "feat-a"))
	processtest.Home(t)
	socket := socktest.Path(t)
	stop := processtest.RealDaemon(t, socket)
	return watchFixture{projectDir: dir, stateDir: filepath.Join(dir, ".git", "wtm"), socket: socket, stop: stop}
}

type watching struct {
	events chan Received
	done   chan error
	cancel context.CancelFunc
}

func (f watchFixture) watch(t *testing.T) watching {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	w := watching{events: make(chan Received, 32), done: make(chan error, 1), cancel: cancel}
	t.Cleanup(cancel)
	go func() {
		w.done <- Watch(ctx, WatchParams{
			ProjectDir: f.projectDir,
			StateDir:   f.stateDir,
			SocketPath: f.socket,
			OnEvent: func(r Received) error {
				w.events <- r
				return nil
			},
		})
	}()
	return w
}

func (w watching) next(t *testing.T) domain.Event {
	t.Helper()
	return w.nextReceived(t).Event
}

func (w watching) nextReceived(t *testing.T) Received {
	t.Helper()
	select {
	case r := <-w.events:
		return r
	case <-time.After(5 * time.Second):
		t.Fatal("no event")
		return Received{}
	}
}

func noDaemonSpawn(t *testing.T) {
	t.Helper()
	previous := ensureDaemon
	ensureDaemon = func(params process.DaemonParams) error {
		if !process.IsDaemonRunning(params.SocketPath) {
			return errors.New("no daemon")
		}
		return nil
	}
	t.Cleanup(func() { ensureDaemon = previous })
}

func TestWatchSendsASnapshotThenReady(t *testing.T) {
	noDaemonSpawn(t)
	f := newWatchFixture(t)
	w := f.watch(t)

	snapshot := w.next(t)
	if snapshot.Type != domain.EventSnapshot || snapshot.V != domain.EventsSchemaVersion || snapshot.Repo == nil || len(snapshot.Worktrees) != 2 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
	if ready := w.next(t); ready.Type != domain.EventReady || ready.Repo != nil {
		t.Fatalf("ready = %+v", ready)
	}
}

func TestAPublishedEventReachesTheWatcherStamped(t *testing.T) {
	noDaemonSpawn(t)
	f := newWatchFixture(t)
	w := f.watch(t)
	w.next(t)
	w.next(t)

	NewPublisher(PublisherParams{ProjectDir: f.projectDir, SocketPath: f.socket}).Publish(t.Context(), domain.Event{Type: domain.EventWorktreeCreated, Worktree: &domain.WorktreeIdentity{Branch: "feat/a"}})

	got := w.next(t)
	if got.Type != domain.EventWorktreeCreated || got.V != domain.EventsSchemaVersion || got.TS == "" || got.Repo == nil || got.Repo.Root != f.projectDir {
		t.Fatalf("event = %+v", got)
	}
}

func TestAPublisherStampsItsCorrelationID(t *testing.T) {
	noDaemonSpawn(t)
	f := newWatchFixture(t)
	w := f.watch(t)
	w.next(t)
	w.next(t)
	created := domain.Event{Type: domain.EventWorktreeCreated, Worktree: &domain.WorktreeIdentity{Branch: "feat/a"}}

	NewPublisher(PublisherParams{ProjectDir: f.projectDir, SocketPath: f.socket, CorrelationID: "popup-1"}).Publish(t.Context(), created)
	NewPublisher(PublisherParams{ProjectDir: f.projectDir, SocketPath: f.socket}).Publish(t.Context(), created)

	if got := w.next(t); got.CorrelationID != "popup-1" {
		t.Fatalf("correlated = %+v", got)
	}
	if got := w.nextReceived(t); got.Event.CorrelationID != "" || bytes.Contains(got.Raw, []byte("correlation_id")) {
		t.Fatalf("uncorrelated = %s", got.Raw)
	}
}

func TestASnapshotNeverCarriesACorrelationID(t *testing.T) {
	noDaemonSpawn(t)
	t.Setenv(domain.EnvCorrelationID, "popup-1")
	f := newWatchFixture(t)
	w := f.watch(t)
	for range 2 {
		if got := w.nextReceived(t); bytes.Contains(got.Raw, []byte("correlation_id")) {
			t.Fatalf("%s carries a correlation id: %s", got.Event.Type, got.Raw)
		}
	}
}

func TestAnEventPublishedDuringTheSnapshotArrivesAfterReady(t *testing.T) {
	noDaemonSpawn(t)
	f := newWatchFixture(t)
	previous := identities
	identities = func(ctx context.Context, params worktree.IdentitiesParams) ([]domain.WorktreeIdentity, error) {
		NewPublisher(PublisherParams{ProjectDir: f.projectDir, SocketPath: f.socket}).Publish(t.Context(), domain.Event{Type: domain.EventWorktreeCreated, Worktree: &domain.WorktreeIdentity{Branch: "feat/b"}})
		return previous(t.Context(), params)
	}
	t.Cleanup(func() { identities = previous })
	w := f.watch(t)

	for _, want := range []domain.EventType{domain.EventSnapshot, domain.EventReady, domain.EventWorktreeCreated} {
		if got := w.next(t); got.Type != want {
			t.Fatalf("got %s, want %s", got.Type, want)
		}
	}
}

func TestWatchReconnectsAfterTheDaemonDies(t *testing.T) {
	noDaemonSpawn(t)
	f := newWatchFixture(t)
	w := f.watch(t)
	w.next(t)
	w.next(t)

	f.stop()
	processtest.RealDaemon(t, f.socket)

	for _, want := range []domain.EventType{domain.EventSnapshot, domain.EventReady} {
		if got := w.next(t); got.Type != want {
			t.Fatalf("after the restart got %s, want %s", got.Type, want)
		}
	}
}

func TestANewerSchemaEndsWatch(t *testing.T) {
	noDaemonSpawn(t)
	f := newWatchFixture(t)
	w := f.watch(t)
	w.next(t)
	w.next(t)
	repo, err := worktree.RepoOf(t.Context(), worktree.RepoOfParams{ProjectDir: f.projectDir})
	if err != nil {
		t.Fatal(err)
	}

	payload, _ := json.Marshal(map[string]any{"v": domain.EventsSchemaVersion + 1, "type": "worktree.created", "ts": "2026-10-03T10:00:00Z"})
	if err := process.Publish(process.PublishParams{SocketPath: f.socket, Repo: repo.CommonDir, Payload: payload}); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-w.done:
		if !errors.Is(err, domain.ErrEventsSchemaNewer) {
			t.Fatalf("Watch = %v, want ErrEventsSchemaNewer", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Watch kept going on a newer schema")
	}
}

func TestWatchReturnsNilWhenCancelled(t *testing.T) {
	noDaemonSpawn(t)
	f := newWatchFixture(t)
	w := f.watch(t)
	w.next(t)
	w.cancel()

	select {
	case err := <-w.done:
		if err != nil {
			t.Fatalf("Watch = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Watch ignored the cancellation")
	}
}

func TestAConsumerThatCannotWriteEndsWatch(t *testing.T) {
	noDaemonSpawn(t)
	f := newWatchFixture(t)
	broken := errors.New("stdout closed")

	err := Watch(context.Background(), WatchParams{
		ProjectDir: f.projectDir,
		StateDir:   f.stateDir,
		SocketPath: f.socket,
		OnEvent:    func(Received) error { return broken },
	})

	if !errors.Is(err, broken) {
		t.Fatalf("Watch = %v, want the consumer's error", err)
	}
}

func TestTheDaemonAWatcherStartsServesTheProxy(t *testing.T) {
	noDaemonSpawn(t)
	f := newWatchFixture(t)
	var asked process.DaemonParams
	previous := ensureDaemon
	ensureDaemon = func(params process.DaemonParams) error {
		asked = params
		return previous(params)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = Watch(ctx, WatchParams{
		ProjectDir: f.projectDir,
		StateDir:   f.stateDir,
		SocketPath: f.socket,
		ProxyPort:  11080,
		OnEvent: func(Received) error {
			cancel()
			return nil
		},
	})

	if asked.ProxyPort != 11080 || asked.SocketPath != f.socket {
		t.Fatalf("ensured %+v, want the proxy port passed on", asked)
	}
}

func TestOnlyADaemonThatPredatesSubscribeIsReplaced(t *testing.T) {
	processtest.Home(t)
	dir := gittest.InitRepo(t)
	socket := socktest.Path(t)
	socktest.Serve(t, socket, func(json.RawMessage) any {
		return process.Response{Status: process.StatusError, Message: domain.DaemonUnknownActionPrefix + ": subscribe"}
	})
	previousEnsure, previousReplace := ensureDaemon, replaceDaemon
	t.Cleanup(func() { ensureDaemon, replaceDaemon = previousEnsure, previousReplace })
	ensureDaemon = func(process.DaemonParams) error { return nil }
	replaced := make(chan struct{}, 8)
	replaceDaemon = func(process.DaemonParams) error {
		replaced <- struct{}{}
		return nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_ = Watch(ctx, WatchParams{ProjectDir: dir, StateDir: filepath.Join(dir, ".git", "wtm"), SocketPath: socket, OnEvent: func(Received) error { return nil }})
	}()

	select {
	case <-replaced:
	case <-time.After(3 * time.Second):
		t.Fatal("a daemon that cannot serve a subscription was left in place")
	}
}

// An older wtm relays a newer publisher's events: what it does not know must
// reach the consumer untouched, or "unknown fields are ignored" is a promise
// only the newest build can keep.
func TestARelayedEventKeepsTheFieldsThisBuildDoesNotKnow(t *testing.T) {
	noDaemonSpawn(t)
	f := newWatchFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	raws := make(chan json.RawMessage, 8)
	go func() {
		_ = Watch(ctx, WatchParams{ProjectDir: f.projectDir, StateDir: f.stateDir, SocketPath: f.socket, OnEvent: func(r Received) error {
			raws <- r.Raw
			return nil
		}})
	}()
	<-raws
	<-raws
	repo, err := worktree.RepoOf(ctx, worktree.RepoOfParams{ProjectDir: f.projectDir})
	if err != nil {
		t.Fatal(err)
	}

	if err := process.Publish(process.PublishParams{SocketPath: f.socket, Repo: repo.CommonDir, Payload: json.RawMessage(`{"v":1,"type":"worktree.created","ts":"t","extra":1}`)}); err != nil {
		t.Fatal(err)
	}

	select {
	case raw := <-raws:
		if !strings.Contains(string(raw), `"extra":1`) {
			t.Fatalf("relayed %s, want the unknown field kept", raw)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no event")
	}
}

func TestASnapshotOfNothingIsAnEmptyList(t *testing.T) {
	previous := identities
	identities = func(context.Context, worktree.IdentitiesParams) ([]domain.WorktreeIdentity, error) { return nil, nil }
	t.Cleanup(func() { identities = previous })

	received, err := snapshotOf(t.Context(), snapshotParams{Repo: domain.EventRepo{Root: "/r", CommonDir: "/r/.git"}})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(received.Raw), `"worktrees":[]`) {
		t.Fatalf("snapshot = %s, want an empty list rather than no field", received.Raw)
	}
}

func TestThePublisherListensOnlyWhileTheDaemonRuns(t *testing.T) {
	f := newWatchFixture(t)
	publisher := NewPublisher(PublisherParams{ProjectDir: f.projectDir, SocketPath: f.socket})
	if !publisher.Listening() {
		t.Fatal("not listening with the daemon up")
	}
	f.stop()
	if publisher.Listening() {
		t.Fatal("listening with no daemon")
	}
}

// repo.* is the global stream's business: a per-repository stream that relays
// a repo.added gets no snapshot after it, and would leave its reader empty.
func TestAPerRepoStreamDoesNotRelayRepoEvents(t *testing.T) {
	noDaemonSpawn(t)
	f := newWatchFixture(t)
	w := f.watch(t)
	w.next(t)
	w.next(t)
	repo, err := worktree.RepoOf(t.Context(), worktree.RepoOfParams{ProjectDir: f.projectDir})
	if err != nil {
		t.Fatal(err)
	}

	announce(announceParams{Bus: busParams{SocketPath: f.socket}, Type: domain.EventRepoRemoved, Repo: repo})
	NewPublisher(PublisherParams{ProjectDir: f.projectDir, SocketPath: f.socket}).Publish(t.Context(), domain.Event{Type: domain.EventWorktreeCreated, Worktree: &domain.WorktreeIdentity{Branch: "feat/a"}})

	if got := w.next(t); got.Type != domain.EventWorktreeCreated {
		t.Fatalf("got %s, want the repo event skipped", got.Type)
	}
}
