package events_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAStreamSubscribesBeforeItReadsItsSnapshot(t *testing.T) {
	app := newRepo("/repo")
	log := &callLog{}
	app.Config.log = log
	bus := newBus()
	bus.log = log

	r, _ := watching(t, useCase(bus, &fakeRegistry{}, opening(app)), events.Request{Dir: "/repo"})
	r.nextType(t)

	// the first read refuses a repository with no config before anything subscribes
	assert.Equal(t, []string{"observe", "subscribe", "observe"}, log.all())
}

func TestAStreamOpensOnItsSnapshotThenReadyThenTheChangesOfItsRepository(t *testing.T) {
	app := newRepo("/repo", "feat/a")
	app.Meta["feat/a"] = worktree.Meta{SourceBranch: "main", CreatedAt: "2026-10-16T11:00:00Z", Ordinal: 1}
	bus := newBus()
	created := published(t, app.Repo, domain.WorktreeCreated{Worktree: worktree.Identity{Branch: "feat/b", Path: "/repo/.trees/feat/b"}})
	// published while the snapshot is read: it waits in the subscription made first
	bus.pending = []string{string(created)}

	r, _ := watching(t, useCase(bus, &fakeRegistry{}, opening(app)), events.Request{Dir: "/repo"})

	assert.Equal(t, `{"v":2,"type":"snapshot","ts":"2026-10-16T12:00:00Z","repo":{"root":"/repo","common_dir":"/repo/.git"},"worktrees":[{"branch":"main","path":"/repo","parent":"","ordinal":0,"isolation":"isolated","is_main":true,"created_at":""},{"branch":"feat/a","path":"/repo/.trees/feat/a","parent":"main","ordinal":1,"isolation":"isolated","is_main":false,"created_at":"2026-10-16T11:00:00Z"}]}`, r.next(t))
	assert.Equal(t, `{"v":2,"type":"ready","ts":"2026-10-16T12:00:00Z"}`, r.next(t))
	assert.Equal(t, string(created), r.next(t))
	assert.Equal(t, []string{"/repo/.git"}, bus.subscribed(t).Repos)
}

func TestWhenTheBusGoesAwayTheStreamOpensAgainOnAFreshSnapshot(t *testing.T) {
	bus := newBus()
	r, _ := watching(t, useCase(bus, &fakeRegistry{}, opening(newRepo("/repo"))), events.Request{Dir: "/repo"})
	first := bus.subscribed(t)
	r.nextType(t)
	r.nextType(t)

	close(first.Lines)

	bus.subscribed(t)
	assert.Equal(t, "snapshot", r.nextType(t))
	assert.Equal(t, "ready", r.nextType(t))
}

func TestABusOutOfReachIsAWarningAndTriedAgain(t *testing.T) {
	bus := newBus(errBusGone)

	r, _ := watching(t, useCase(bus, &fakeRegistry{}, opening(newRepo("/repo"))), events.Request{Dir: "/repo"})

	assert.ErrorIs(t, r.warning(t), errBusGone)
	assert.Equal(t, "snapshot", r.nextType(t))
}

func TestALineOfANewerSchemaEndsTheStream(t *testing.T) {
	bus := newBus()
	r, done := watching(t, useCase(bus, &fakeRegistry{}, opening(newRepo("/repo"))), events.Request{Dir: "/repo"})
	sub := bus.subscribed(t)
	r.nextType(t)
	r.nextType(t)

	sub.Lines <- []byte(`{"v":3,"type":"worktree.created","repo":{"root":"/repo","common_dir":"/repo/.git"}}`)

	err := ended(t, done)
	assert.Equal(t, events.CodeSchemaNewer, codeOf(t, err))
	assert.EqualError(t, err, "the event stream speaks a newer schema than this wtm: upgrade wtm")
}

// Legacy dropped a payload it could not decode without a word.
func TestALineThatCannotBeReadIsDroppedWithAWarning(t *testing.T) {
	app := newRepo("/repo")
	bus := newBus()
	r, _ := watching(t, useCase(bus, &fakeRegistry{}, opening(app)), events.Request{Dir: "/repo"})
	sub := bus.subscribed(t)
	r.nextType(t)
	r.nextType(t)

	sub.Lines <- []byte(`{"v":2,"type":"worktree.created","worktree":"not an identity"}`)
	sub.Lines <- published(t, app.Repo, domain.RepoAdded{})

	assert.ErrorContains(t, r.warning(t), "an event the bus carried could not be read, and was dropped")
	assert.Equal(t, "repo.added", r.nextType(t))
}

func TestALineOfATypeThisWtmDoesNotKnowIsRelayedAsItCame(t *testing.T) {
	bus := newBus()
	r, _ := watching(t, useCase(bus, &fakeRegistry{}, opening(newRepo("/repo"))), events.Request{Dir: "/repo"})
	sub := bus.subscribed(t)
	r.nextType(t)
	r.nextType(t)
	job := `{"v":2,"type":"job.started","ts":"t","repo":{"root":"/repo","common_dir":"/repo/.git"},"job":{"name":"web"}}`

	sub.Lines <- []byte(job)

	received := <-r.lines
	assert.Equal(t, job, string(received.JSON))
	assert.Nil(t, received.Line.Event)
}

// Legacy never noticed its repository going away and streamed on forever.
func TestAStreamEndsWhenItsRepositoryLosesItsConfig(t *testing.T) {
	app := newRepo("/repo")
	u := useCase(newBus(), &fakeRegistry{}, opening(app))
	u.Recheck = 20 * time.Millisecond
	r, done := watching(t, u, events.Request{Dir: "/repo"})
	r.nextType(t)
	r.nextType(t)

	app.Config.remove()

	assert.Equal(t, events.CodeConfigNotFound, codeOf(t, ended(t, done)))
}

func TestAStreamEndsWhenItsRepositoryIsAnnouncedRemovedAndItsConfigIsGone(t *testing.T) {
	app := newRepo("/repo")
	bus := newBus()
	r, done := watching(t, useCase(bus, &fakeRegistry{}, opening(app)), events.Request{Dir: "/repo"})
	sub := bus.subscribed(t)
	r.nextType(t)
	r.nextType(t)

	app.Config.remove()
	sub.Lines <- published(t, app.Repo, domain.RepoRemoved{})

	assert.Equal(t, "repo.removed", r.nextType(t))
	assert.Equal(t, events.CodeConfigNotFound, codeOf(t, ended(t, done)))
}

func TestARepositoryWithNoConfigIsRefusedBeforeTheStreamAndNotRegistered(t *testing.T) {
	app := newRepo("/repo")
	app.Config.remove()
	registry := &fakeRegistry{}

	err := useCase(newBus(), registry, opening(app)).Watch(t.Context(), events.Request{Dir: "/repo"}, newReader().out())

	assert.Equal(t, events.CodeConfigNotFound, codeOf(t, err))
	assert.Empty(t, registry.registers())
}

func TestAStreamRegistersTheRepositoryItWatches(t *testing.T) {
	app := newRepo("/repo")
	registry := &fakeRegistry{}

	r, _ := watching(t, useCase(newBus(), registry, opening(app)), events.Request{Dir: "/repo"})
	r.nextType(t)

	assert.Equal(t, []domain.Repo{app.Repo}, registry.registers())
}

// A stream relays only the lines of its own version: a legacy daemon holding the socket would hand it v1 lines.
func TestALineOfAnOlderSchemaIsDroppedWithAWarning(t *testing.T) {
	app := newRepo("/repo")
	bus := newBus()
	r, _ := watching(t, useCase(bus, &fakeRegistry{}, opening(app)), events.Request{Dir: "/repo"})
	sub := bus.subscribed(t)
	r.nextType(t)
	r.nextType(t)

	sub.Lines <- []byte(`{"v":1,"type":"worktree.created","ts":"t","repo":{"root":"/repo","common_dir":"/repo/.git"}}`)
	sub.Lines <- []byte(`{"v":1,"type":"job.started","ts":"t","repo":{"root":"/repo","common_dir":"/repo/.git"},"job":{"name":"web"}}`)
	sub.Lines <- published(t, app.Repo, domain.RepoAdded{})

	assert.ErrorContains(t, r.warning(t), "a line of the event stream v1 was dropped")
	assert.ErrorContains(t, r.warning(t), "a line of the event stream v1 was dropped")
	assert.Equal(t, "repo.added", r.nextType(t))
}

func TestARequestThatCannotBeWatchedIsRefused(t *testing.T) {
	for name, c := range map[string]struct {
		Request events.Request
		Code    string
	}{
		"repo_with_all":       {Request: events.Request{Repo: "/repo", All: true}, Code: domain.CodeInvalidRequest},
		"repo_not_directory":  {Request: events.Request{Repo: "/nowhere"}, Code: domain.CodeInvalidRequest},
		"repo_not_repository": {Request: events.Request{Repo: "/tmp"}, Code: domain.CodeNotARepository},
		"named_dir_outside":   {Request: events.Request{Dir: "/tmp", Named: true}, Code: domain.CodeNotARepository},
	} {
		t.Run(name, func(t *testing.T) {
			err := useCase(newBus(), &fakeRegistry{}, opening(newRepo("/repo"))).Watch(t.Context(), c.Request, newReader().out())

			assert.Equal(t, c.Code, codeOf(t, err))
		})
	}
}

func TestTheFieldsOfARefusedRequestNameWhatToChange(t *testing.T) {
	u := useCase(newBus(), &fakeRegistry{}, opening(newRepo("/repo")))

	both := u.Watch(t.Context(), events.Request{Repo: "/repo", All: true}, newReader().out())
	nowhere := u.Watch(t.Context(), events.Request{Repo: "/nowhere"}, newReader().out())

	var failure *domain.Error
	require.ErrorAs(t, both, &failure)
	assert.Equal(t, events.CodeRepoWithAll, failure.Fields[0].Code)
	require.ErrorAs(t, nowhere, &failure)
	assert.Equal(t, "repo /nowhere is not a directory", failure.Fields[0].Message("repo"))
}

// Legacy subscribed to the --repo repository with $WTM_PROJECT_DIR's metadata.
func TestTheRepositoryNamedByRepoWinsOverTheDirectory(t *testing.T) {
	named, current := newRepo("/named"), newRepo("/current")
	bus := newBus()

	r, _ := watching(t, useCase(bus, &fakeRegistry{}, opening(named, current)), events.Request{Repo: "/named", Dir: "/current", Named: true})

	assert.Equal(t, []string{"/named/.git"}, bus.subscribed(t).Repos)
	assert.Contains(t, r.next(t), `"root":"/named"`)
}

func TestAnUnnamedDirectoryInNoRepositoryWatchesEveryRepository(t *testing.T) {
	bus := newBus()

	watching(t, useCase(bus, &fakeRegistry{}, opening()), events.Request{Dir: "/tmp"})

	assert.Empty(t, bus.subscribed(t).Repos)
}

func TestAnInterruptBeforeTheStreamStartsIsAnInterruption(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	done := watchingWith(t, watchingParams{UseCase: useCase(newBus(errBusGone, errBusGone, errBusGone), &fakeRegistry{}, opening(newRepo("/repo"))), Request: events.Request{Dir: "/repo"}, Reader: newReader(), Context: ctx})

	cancel()

	assert.ErrorIs(t, ended(t, done), context.Canceled)
}

func TestAnInterruptOnceTheStreamStartedEndsItWell(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	r := newReader()
	done := watchingWith(t, watchingParams{UseCase: useCase(newBus(), &fakeRegistry{}, opening(newRepo("/repo"))), Request: events.Request{Dir: "/repo"}, Reader: r, Context: ctx})
	r.nextType(t)

	cancel()

	assert.NoError(t, ended(t, done))
}

func TestAReaderThatFailsEndsTheStreamWithItsError(t *testing.T) {
	r := newReader()
	r.fail = errors.New("broken pipe")

	done := watchingWith(t, watchingParams{UseCase: useCase(newBus(), &fakeRegistry{}, opening(newRepo("/repo"))), Request: events.Request{Dir: "/repo"}, Reader: r})

	assert.EqualError(t, ended(t, done), "broken pipe")
}

func TestTheGlobalStreamOpensOnASnapshotOfEachRepositoryThenOneReady(t *testing.T) {
	a, b := newRepo("/a"), newRepo("/b")
	bus := newBus()

	r, _ := watching(t, useCase(bus, &fakeRegistry{repos: []domain.Repo{a.Repo, b.Repo}}, opening(a, b)), events.Request{All: true})

	assert.Contains(t, r.next(t), `"root":"/a"`)
	assert.Contains(t, r.next(t), `"root":"/b"`)
	assert.Equal(t, "ready", r.nextType(t))
}

func TestARepositoryTheGlobalStreamCannotReadIsSkippedWithAWarning(t *testing.T) {
	a, broken := newRepo("/a"), newRepo("/broken")
	broken.Config.err = errors.New("permission denied")

	r, _ := watching(t, useCase(newBus(), &fakeRegistry{repos: []domain.Repo{broken.Repo, a.Repo}}, opening(a, broken)), events.Request{All: true})

	assert.EqualError(t, r.warning(t), "/broken: permission denied")
	assert.Contains(t, r.next(t), `"root":"/a"`)
	assert.Equal(t, "ready", r.nextType(t))
}

func TestARepositoryJoiningIsSentWithItsSnapshot(t *testing.T) {
	joining := newRepo("/joining")
	bus := newBus()
	r, _ := watching(t, useCase(bus, &fakeRegistry{}, opening(joining)), events.Request{All: true})
	sub := bus.subscribed(t)
	assert.Equal(t, "ready", r.nextType(t))

	sub.Lines <- published(t, joining.Repo, domain.RepoAdded{})

	assert.Equal(t, "repo.added", r.nextType(t))
	assert.Contains(t, r.next(t), `"type":"snapshot","ts":"2026-10-16T12:00:00Z","repo":{"root":"/joining"`)
}

// A repository joining with no wtm config once looped: the stream's join registered it again, whose prune dropped it, announcing it anew.
func TestARepositoryJoiningWithNoConfigIsOneLineAndIsNotRegistered(t *testing.T) {
	uninitialized := newRepo("/bare")
	uninitialized.Config.remove()
	bus, registry := newBus(), &fakeRegistry{}
	r, _ := watching(t, useCase(bus, registry, opening(uninitialized)), events.Request{All: true})
	sub := bus.subscribed(t)
	assert.Equal(t, "ready", r.nextType(t))

	sub.Lines <- published(t, uninitialized.Repo, domain.RepoAdded{})

	assert.Equal(t, "repo.added", r.nextType(t))
	assert.ErrorContains(t, r.warning(t), "/bare: no wtm config found")
	r.quiet(t)
	assert.Empty(t, registry.registers())
}

// Legacy sent a second snapshot of a repository registered between its subscription and its first prune.
func TestARepositoryAlreadySnapshottedIsNotSentAgainWhenItsJoiningArrives(t *testing.T) {
	a := newRepo("/a")
	bus := newBus()
	r, _ := watching(t, useCase(bus, &fakeRegistry{repos: []domain.Repo{a.Repo}}, opening(a)), events.Request{All: true})
	sub := bus.subscribed(t)
	r.nextType(t)
	r.nextType(t)

	sub.Lines <- published(t, a.Repo, domain.RepoAdded{})

	r.quiet(t)
}

// Legacy relayed the events of a repository it had sent no snapshot of.
func TestTheGlobalStreamDropsTheEventsOfARepositoryItSentNoSnapshotOf(t *testing.T) {
	a, unknown := newRepo("/a"), newRepo("/unknown")
	bus := newBus()
	r, _ := watching(t, useCase(bus, &fakeRegistry{repos: []domain.Repo{a.Repo}}, opening(a, unknown)), events.Request{All: true})
	sub := bus.subscribed(t)
	r.nextType(t)
	r.nextType(t)

	sub.Lines <- published(t, unknown.Repo, domain.WorktreeCreated{Worktree: worktree.Identity{Branch: "feat/x"}})
	sub.Lines <- published(t, unknown.Repo, domain.RepoRemoved{})
	sub.Lines <- published(t, a.Repo, domain.WorktreeCreated{Worktree: worktree.Identity{Branch: "feat/a"}})

	assert.Contains(t, r.next(t), `"branch":"feat/a"`)
}

func TestARepositoryThatLeftTheGlobalStreamHasItsEventsDropped(t *testing.T) {
	a := newRepo("/a")
	bus := newBus()
	r, _ := watching(t, useCase(bus, &fakeRegistry{repos: []domain.Repo{a.Repo}}, opening(a)), events.Request{All: true})
	sub := bus.subscribed(t)
	r.nextType(t)
	r.nextType(t)

	sub.Lines <- published(t, a.Repo, domain.RepoRemoved{})
	sub.Lines <- published(t, a.Repo, domain.WorktreeCreated{Worktree: worktree.Identity{Branch: "feat/a"}})

	assert.Equal(t, "repo.removed", r.nextType(t))
	r.quiet(t)
}

func TestTheGlobalStreamPrunesTheRegistryAsItRuns(t *testing.T) {
	registry := &fakeRegistry{}
	u := useCase(newBus(), registry, opening())
	u.Recheck = 20 * time.Millisecond

	watching(t, u, events.Request{All: true})

	assert.Eventually(t, func() bool { return registry.pruned() >= 3 }, 2*time.Second, 10*time.Millisecond)
}
