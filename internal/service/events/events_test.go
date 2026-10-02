package events

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
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
	events chan domain.Event
	done   chan error
	cancel context.CancelFunc
}

func (f watchFixture) watch(t *testing.T) watching {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	w := watching{events: make(chan domain.Event, 32), done: make(chan error, 1), cancel: cancel}
	t.Cleanup(cancel)
	go func() {
		w.done <- Watch(ctx, WatchParams{
			ProjectDir: f.projectDir,
			StateDir:   f.stateDir,
			SocketPath: f.socket,
			OnEvent: func(e domain.Event) error {
				w.events <- e
				return nil
			},
		})
	}()
	return w
}

func (w watching) next(t *testing.T) domain.Event {
	t.Helper()
	select {
	case e := <-w.events:
		return e
	case <-time.After(5 * time.Second):
		t.Fatal("no event")
		return domain.Event{}
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

	NewPublisher(PublisherParams{ProjectDir: f.projectDir, SocketPath: f.socket}).Publish(domain.Event{Type: domain.EventWorktreeCreated, Worktree: &domain.WorktreeIdentity{Branch: "feat/a"}})

	got := w.next(t)
	if got.Type != domain.EventWorktreeCreated || got.V != domain.EventsSchemaVersion || got.TS == "" || got.Repo == nil || got.Repo.Root != f.projectDir {
		t.Fatalf("event = %+v", got)
	}
}

func TestAnEventPublishedDuringTheSnapshotArrivesAfterReady(t *testing.T) {
	noDaemonSpawn(t)
	f := newWatchFixture(t)
	previous := identities
	identities = func(params worktree.IdentitiesParams) ([]domain.WorktreeIdentity, error) {
		NewPublisher(PublisherParams{ProjectDir: f.projectDir, SocketPath: f.socket}).Publish(domain.Event{Type: domain.EventWorktreeCreated, Worktree: &domain.WorktreeIdentity{Branch: "feat/b"}})
		return previous(params)
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
	repo, err := worktree.RepoOf(worktree.RepoOfParams{ProjectDir: f.projectDir})
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
		OnEvent:    func(domain.Event) error { return broken },
	})

	if !errors.Is(err, broken) {
		t.Fatalf("Watch = %v, want the consumer's error", err)
	}
}
