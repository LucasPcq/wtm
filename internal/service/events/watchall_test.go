package events

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process/processtest"
	"github.com/LucasPcq/wtm/internal/service/worktree"
	"github.com/LucasPcq/wtm/internal/testutil/socktest"
)

type globalWatching struct {
	watching
	warnings chan error
}

func watchAll(t *testing.T, socket string) globalWatching {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	w := globalWatching{
		watching: watching{events: make(chan Received, 32), done: make(chan error, 1), cancel: cancel},
		warnings: make(chan error, 8),
	}
	go func() {
		w.done <- WatchAll(ctx, WatchAllParams{
			SocketPath: socket,
			OnEvent: func(r Received) error {
				w.events <- r
				return nil
			},
			OnWarning: func(err error) { w.warnings <- err },
		})
	}()
	return w
}

func globalFixture(t *testing.T) string {
	t.Helper()
	noDaemonSpawn(t)
	processtest.Home(t)
	socket := socktest.Path(t)
	processtest.RealDaemon(t, socket)
	return socket
}

func register(t *testing.T, dir string) {
	t.Helper()
	if err := Register(RegisterParams{Root: dir, StateDir: stateOf(dir), SocketPath: "/nonexistent"}); err != nil {
		t.Fatal(err)
	}
}

func rootOf(t *testing.T, dir string) string {
	t.Helper()
	repo, err := worktree.RepoOf(worktree.RepoOfParams{ProjectDir: dir})
	if err != nil {
		t.Fatal(err)
	}
	return repo.Root
}

func TestWatchAllSendsOneSnapshotPerRepoThenOneReady(t *testing.T) {
	socket := globalFixture(t)
	a, b := initializedRepo(t), initializedRepo(t)
	register(t, a)
	register(t, b)

	w := watchAll(t, socket)

	var roots []string
	for range 2 {
		got := w.next(t)
		if got.Type != domain.EventSnapshot || got.Repo == nil {
			t.Fatalf("got %+v, want a snapshot", got)
		}
		roots = append(roots, got.Repo.Root)
	}
	if !slices.Contains(roots, rootOf(t, a)) || !slices.Contains(roots, rootOf(t, b)) {
		t.Fatalf("snapshots of %v", roots)
	}
	if got := w.next(t); got.Type != domain.EventReady {
		t.Fatalf("got %s, want ready", got.Type)
	}
}

func TestWatchAllWithAnEmptyRegistrySendsReadyAlone(t *testing.T) {
	socket := globalFixture(t)

	w := watchAll(t, socket)

	if got := w.next(t); got.Type != domain.EventReady {
		t.Fatalf("got %s, want ready", got.Type)
	}
}

func TestARepoAddedIsFollowedByItsSnapshot(t *testing.T) {
	socket := globalFixture(t)
	w := watchAll(t, socket)
	w.next(t)
	dir := initializedRepo(t)

	if err := Register(RegisterParams{Root: dir, StateDir: stateOf(dir), SocketPath: socket}); err != nil {
		t.Fatal(err)
	}

	if got := w.next(t); got.Type != domain.EventRepoAdded {
		t.Fatalf("got %s, want repo.added", got.Type)
	}
	if got := w.next(t); got.Type != domain.EventSnapshot || got.Repo == nil || got.Repo.Root != rootOf(t, dir) {
		t.Fatalf("got %+v, want the new repository's snapshot", got)
	}
}

func TestARepoWhoseSnapshotFailsIsSkippedWithAWarning(t *testing.T) {
	socket := globalFixture(t)
	good, bad := initializedRepo(t), initializedRepo(t)
	register(t, good)
	register(t, bad)
	badRoot := rootOf(t, bad)
	previous := identities
	identities = func(params worktree.IdentitiesParams) ([]domain.WorktreeIdentity, error) {
		if params.ProjectDir == badRoot {
			return nil, errors.New("unreadable")
		}
		return previous(params)
	}
	t.Cleanup(func() { identities = previous })

	w := watchAll(t, socket)

	if got := w.next(t); got.Type != domain.EventSnapshot || got.Repo.Root != rootOf(t, good) {
		t.Fatalf("got %+v, want the good repository's snapshot", got)
	}
	if got := w.next(t); got.Type != domain.EventReady {
		t.Fatalf("got %s, want ready", got.Type)
	}
	select {
	case <-w.warnings:
	case <-time.After(time.Second):
		t.Fatal("no warning for the skipped repository")
	}
}

func TestWatchAllPrunesAGoneRepoBeforeItsSnapshots(t *testing.T) {
	socket := globalFixture(t)
	kept, gone := initializedRepo(t), initializedRepo(t)
	register(t, kept)
	register(t, gone)
	removeConfig(t, gone)

	w := watchAll(t, socket)

	if got := w.next(t); got.Type != domain.EventSnapshot || got.Repo.Root != rootOf(t, kept) {
		t.Fatalf("got %+v, want only the kept repository", got)
	}
	if got := w.next(t); got.Type != domain.EventReady {
		t.Fatalf("got %s, want ready", got.Type)
	}
}
