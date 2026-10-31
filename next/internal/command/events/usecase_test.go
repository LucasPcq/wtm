package events_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
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

	r, _ := watching(t, useCase(bus, &known{}, opening(app)), events.Request{Caller: domain.Caller{Dir: "/repo"}})
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

	r, _ := watching(t, useCase(bus, &known{}, opening(app)), events.Request{Caller: domain.Caller{Dir: "/repo"}})

	assert.Equal(t, `{"v":2,"type":"snapshot","ts":"2026-10-16T12:00:00Z","repo":{"root":"/repo","common_dir":"/repo/.git"},"worktrees":[{"branch":"main","path":"/repo","parent":"","ordinal":0,"isolation":"isolated","is_main":true,"created_at":""},{"branch":"feat/a","path":"/repo/.trees/feat/a","parent":"main","ordinal":1,"isolation":"isolated","is_main":false,"created_at":"2026-10-16T11:00:00Z"}]}`, r.next(t))
	assert.Equal(t, `{"v":2,"type":"ready","ts":"2026-10-16T12:00:00Z"}`, r.next(t))
	assert.Equal(t, string(created), r.next(t))
	assert.Equal(t, []string{"/repo/.git"}, bus.subscribed(t).Repos)
}

func TestWhenTheBusGoesAwayTheStreamOpensAgainOnAFreshSnapshot(t *testing.T) {
	bus := newBus()
	r, _ := watching(t, useCase(bus, &known{}, opening(newRepo("/repo"))), events.Request{Caller: domain.Caller{Dir: "/repo"}})
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

	r, _ := watching(t, useCase(bus, &known{}, opening(newRepo("/repo"))), events.Request{Caller: domain.Caller{Dir: "/repo"}})

	assert.ErrorIs(t, r.warning(t), errBusGone)
	assert.Equal(t, "snapshot", r.nextType(t))
}

func TestALineOfANewerSchemaEndsTheStream(t *testing.T) {
	bus := newBus()
	r, done := watching(t, useCase(bus, &known{}, opening(newRepo("/repo"))), events.Request{Caller: domain.Caller{Dir: "/repo"}})
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
	r, _ := watching(t, useCase(bus, &known{}, opening(app)), events.Request{Caller: domain.Caller{Dir: "/repo"}})
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
	r, _ := watching(t, useCase(bus, &known{}, opening(newRepo("/repo"))), events.Request{Caller: domain.Caller{Dir: "/repo"}})
	sub := bus.subscribed(t)
	r.nextType(t)
	r.nextType(t)
	job := `{"v":2,"type":"job.restarted","ts":"t","repo":{"root":"/repo","common_dir":"/repo/.git"},"job":{"name":"web"}}`

	sub.Lines <- []byte(job)

	received := <-r.lines
	assert.Equal(t, job, string(received.JSON))
	assert.Nil(t, received.Line.Event)
}

// Legacy never noticed its repository going away and streamed on forever.
func TestAStreamEndsWhenItsRepositoryLosesItsConfig(t *testing.T) {
	app := newRepo("/repo")
	u := useCase(newBus(), &known{}, opening(app))
	u.Recheck = 20 * time.Millisecond
	r, done := watching(t, u, events.Request{Caller: domain.Caller{Dir: "/repo"}})
	r.nextType(t)
	r.nextType(t)

	app.Config.Remove()

	assert.Equal(t, events.CodeConfigNotFound, codeOf(t, ended(t, done)))
}

func TestAStreamEndsWhenItsRepositoryIsAnnouncedRemovedAndItsConfigIsGone(t *testing.T) {
	app := newRepo("/repo")
	bus := newBus()
	r, done := watching(t, useCase(bus, &known{}, opening(app)), events.Request{Caller: domain.Caller{Dir: "/repo"}})
	sub := bus.subscribed(t)
	r.nextType(t)
	r.nextType(t)

	app.Config.Remove()
	sub.Lines <- published(t, app.Repo, domain.RepoRemoved{})

	assert.Equal(t, "repo.removed", r.nextType(t))
	assert.Equal(t, events.CodeConfigNotFound, codeOf(t, ended(t, done)))
}

func TestARepositoryWithNoConfigIsRefusedBeforeTheStreamAndNotRegistered(t *testing.T) {
	app := newRepo("/repo")
	app.Config.Remove()
	k := &known{}

	err := useCase(newBus(), k, opening(app)).Watch(t.Context(), events.Request{Caller: domain.Caller{Dir: "/repo"}}, newReader().out())

	assert.Equal(t, events.CodeConfigNotFound, codeOf(t, err))
	assert.Empty(t, k.registers())
}

func TestAStreamRegistersTheRepositoryItWatches(t *testing.T) {
	app := newRepo("/repo")
	k := &known{}

	r, _ := watching(t, useCase(newBus(), k, opening(app)), events.Request{Caller: domain.Caller{Dir: "/repo"}})
	r.nextType(t)

	assert.Equal(t, []domain.Repo{app.Repo}, k.registers())
}

// A stream relays only the lines of its own version: a legacy daemon holding the socket would hand it v1 lines.
func TestALineOfAnOlderSchemaIsDroppedWithAWarning(t *testing.T) {
	app := newRepo("/repo")
	bus := newBus()
	r, _ := watching(t, useCase(bus, &known{}, opening(app)), events.Request{Caller: domain.Caller{Dir: "/repo"}})
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
		"repo_with_all":         {Request: events.Request{Repo: "/repo", All: true}, Code: domain.CodeInvalidRequest},
		"repo_not_directory":    {Request: events.Request{Repo: "/nowhere"}, Code: domain.CodeInvalidRequest},
		"repo_not_repository":   {Request: events.Request{Repo: "/tmp"}, Code: domain.CodeNotARepository},
		"named_dir_outside":     {Request: events.Request{Caller: domain.Caller{Dir: "/tmp", Named: true}}, Code: domain.CodeNotARepository},
		"relative_from_nowhere": {Request: events.Request{Repo: "repo"}, Code: domain.CodeInvalidRequest},
	} {
		t.Run(name, func(t *testing.T) {
			err := useCase(newBus(), &known{}, opening(newRepo("/repo"))).Watch(t.Context(), c.Request, newReader().out())

			assert.Equal(t, c.Code, codeOf(t, err))
		})
	}
}

func TestTheFieldsOfARefusedRequestNameWhatToChange(t *testing.T) {
	u := useCase(newBus(), &known{}, opening(newRepo("/repo")))

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

	r, _ := watching(t, useCase(bus, &known{}, opening(named, current)), events.Request{Repo: "/named", Caller: domain.Caller{Dir: "/current", Named: true}})

	assert.Equal(t, []string{"/named/.git"}, bus.subscribed(t).Repos)
	assert.Contains(t, r.next(t), `"root":"/named"`)
}

// The command line read a relative --repo from its working directory, the API from the call's directory: the use case reads it from where the caller stands, whatever directory it runs on ($WTM_PROJECT_DIR).
func TestARelativeRepoIsReadFromWhereTheCallerStands(t *testing.T) {
	bus := newBus()

	r, _ := watching(t, useCase(bus, &known{}, opening(newRepo("/code/app"), newRepo("/elsewhere/app"))), events.Request{Repo: "app", Caller: domain.Caller{Dir: "/elsewhere", Named: true, Cwd: "/code"}})

	assert.Equal(t, []string{"/code/app/.git"}, bus.subscribed(t).Repos)
	assert.Contains(t, r.next(t), `"root":"/code/app"`)
}

func TestACallerStandingNowhereWatchesEveryRepository(t *testing.T) {
	bus := newBus()

	watching(t, useCase(bus, &known{}, opening()), events.Request{})

	assert.Empty(t, bus.subscribed(t).Repos)
}

func TestAnUnnamedDirectoryInNoRepositoryWatchesEveryRepository(t *testing.T) {
	bus := newBus()

	watching(t, useCase(bus, &known{}, opening()), events.Request{Caller: domain.Caller{Dir: "/tmp"}})

	assert.Empty(t, bus.subscribed(t).Repos)
}

func TestAnInterruptBeforeTheStreamStartsIsAnInterruption(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	done := watchingWith(t, watchingParams{UseCase: useCase(newBus(errBusGone, errBusGone, errBusGone), &known{}, opening(newRepo("/repo"))), Request: events.Request{Caller: domain.Caller{Dir: "/repo"}}, Reader: newReader(), Context: ctx})

	cancel()

	assert.ErrorIs(t, ended(t, done), context.Canceled)
}

func TestAnInterruptOnceTheStreamStartedEndsItWell(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	r := newReader()
	done := watchingWith(t, watchingParams{UseCase: useCase(newBus(), &known{}, opening(newRepo("/repo"))), Request: events.Request{Caller: domain.Caller{Dir: "/repo"}}, Reader: r, Context: ctx})
	r.nextType(t)

	cancel()

	assert.NoError(t, ended(t, done))
}

func TestAReaderThatFailsEndsTheStreamWithItsError(t *testing.T) {
	r := newReader()
	r.fail = errors.New("broken pipe")

	done := watchingWith(t, watchingParams{UseCase: useCase(newBus(), &known{}, opening(newRepo("/repo"))), Request: events.Request{Caller: domain.Caller{Dir: "/repo"}}, Reader: r})

	assert.EqualError(t, ended(t, done), "broken pipe")
}

func TestTheGlobalStreamOpensOnASnapshotOfEachRepositoryThenOneReady(t *testing.T) {
	a, b := newRepo("/a"), newRepo("/b")
	bus := newBus()

	r, _ := watching(t, useCase(bus, &known{repos: []domain.Repo{a.Repo, b.Repo}}, opening(a, b)), events.Request{All: true})

	assert.Contains(t, r.next(t), `"root":"/a"`)
	assert.Contains(t, r.next(t), `"root":"/b"`)
	assert.Equal(t, "ready", r.nextType(t))
}

func TestARepositoryTheGlobalStreamCannotReadIsSkippedWithAWarning(t *testing.T) {
	a, broken := newRepo("/a"), newRepo("/broken")
	broken.Config = newConfig(map[string]error{"Load": errors.New("permission denied")})

	r, _ := watching(t, useCase(newBus(), &known{repos: []domain.Repo{broken.Repo, a.Repo}}, opening(a, broken)), events.Request{All: true})

	assert.EqualError(t, r.warning(t), "/broken: permission denied")
	assert.Contains(t, r.next(t), `"root":"/a"`)
	assert.Equal(t, "ready", r.nextType(t))
}

func TestARepositoryJoiningIsSentWithItsSnapshot(t *testing.T) {
	joining := newRepo("/joining")
	bus := newBus()
	r, _ := watching(t, useCase(bus, &known{}, opening(joining)), events.Request{All: true})
	sub := bus.subscribed(t)
	assert.Equal(t, "ready", r.nextType(t))

	sub.Lines <- published(t, joining.Repo, domain.RepoAdded{})

	assert.Equal(t, "repo.added", r.nextType(t))
	assert.Contains(t, r.next(t), `"type":"snapshot","ts":"2026-10-16T12:00:00Z","repo":{"root":"/joining"`)
}

// $WTM_STATE_DIR moves the state, not the bus: a repository another process announces would be read with the moved state.
func TestARepositoryJoiningIsIgnoredWhileTheStateIsMoved(t *testing.T) {
	joining := newRepo("/joining")
	bus := newBus()
	r, _ := watching(t, useCase(bus, &known{moved: true}, opening(joining)), events.Request{All: true})
	sub := bus.subscribed(t)
	assert.Equal(t, "ready", r.nextType(t))

	sub.Lines <- published(t, joining.Repo, domain.RepoAdded{})

	r.quiet(t)
}

// A repository joining with no wtm config once looped: the stream's join registered it again, whose prune dropped it, announcing it anew.
func TestARepositoryJoiningWithNoConfigIsOneLineAndIsNotRegistered(t *testing.T) {
	uninitialized := newRepo("/bare")
	uninitialized.Config.Remove()
	bus, k := newBus(), &known{}
	r, _ := watching(t, useCase(bus, k, opening(uninitialized)), events.Request{All: true})
	sub := bus.subscribed(t)
	assert.Equal(t, "ready", r.nextType(t))

	sub.Lines <- published(t, uninitialized.Repo, domain.RepoAdded{})

	assert.Equal(t, "repo.added", r.nextType(t))
	assert.ErrorContains(t, r.warning(t), "/bare: no wtm config found")
	r.quiet(t)
	assert.Empty(t, k.registers())
}

// Legacy sent a second snapshot of a repository registered between its subscription and its first prune.
func TestARepositoryAlreadySnapshottedIsNotSentAgainWhenItsJoiningArrives(t *testing.T) {
	a := newRepo("/a")
	bus := newBus()
	r, _ := watching(t, useCase(bus, &known{repos: []domain.Repo{a.Repo}}, opening(a)), events.Request{All: true})
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
	r, _ := watching(t, useCase(bus, &known{repos: []domain.Repo{a.Repo}}, opening(a, unknown)), events.Request{All: true})
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
	r, _ := watching(t, useCase(bus, &known{repos: []domain.Repo{a.Repo}}, opening(a)), events.Request{All: true})
	sub := bus.subscribed(t)
	r.nextType(t)
	r.nextType(t)

	sub.Lines <- published(t, a.Repo, domain.RepoRemoved{})
	sub.Lines <- published(t, a.Repo, domain.WorktreeCreated{Worktree: worktree.Identity{Branch: "feat/a"}})

	assert.Equal(t, "repo.removed", r.nextType(t))
	r.quiet(t)
}

func TestTheGlobalStreamPrunesTheRegistryAsItRuns(t *testing.T) {
	k := &known{}
	u := useCase(newBus(), k, opening())
	u.Recheck = 20 * time.Millisecond

	watching(t, u, events.Request{All: true})

	assert.Eventually(t, func() bool { return k.pruned() >= 3 }, 2*time.Second, 10*time.Millisecond)
}

// The legacy dashboard saw a commit made from a terminal: the stream polls what no operation of wtm announces.
func TestACommitFromATerminalIsSentAsTheWorktreeChanged(t *testing.T) {
	app := newRepo("/repo", "feat/a")
	app.Git.Local = []string{"feat/ahead"}
	app.Git.Commits = map[worktree.Ref][]string{worktree.LocalBranch{Name: "feat/ahead"}: {"c1"}}
	app, git := app.living()
	u := useCase(newBus(), &known{}, opening(app))
	u.Recheck = 20 * time.Millisecond
	r, _ := watching(t, u, events.Request{Caller: domain.Caller{Dir: "/repo"}})
	r.nextType(t)
	r.nextType(t)
	r.quiet(t)

	require.NoError(t, git.MergeFFOnly(t.Context(), worktree.Merge{Path: "/repo/.trees/feat/a", Onto: worktree.LocalBranch{Name: "feat/ahead"}}))

	assert.Equal(t, `{"v":2,"type":"worktree.changed","ts":"2026-10-16T12:00:00Z","repo":{"root":"/repo","common_dir":"/repo/.git"},"worktree":{"branch":"feat/a","path":"/repo/.trees/feat/a","parent":"","ordinal":null,"isolation":"isolated","is_main":false,"created_at":""},"changed":["head"]}`, r.next(t))
	r.quiet(t)
}

func TestAJobThatStartsOrStopsIsSentAsTheWorktreeChanged(t *testing.T) {
	jobs := &machine{}
	u := useCase(newBus(), &known{}, opening(newRepo("/repo", "feat/a")))
	u.Jobs, u.Recheck = jobs.Jobs, 20*time.Millisecond
	r, _ := watching(t, u, events.Request{Caller: domain.Caller{Dir: "/repo"}})
	r.nextType(t)
	r.nextType(t)

	jobs.set([]run.Job{{Name: "web", WorkDir: "/repo/.trees/feat/a", Up: true, State: run.Running}}, nil)

	assert.Contains(t, r.next(t), `"branch":"feat/a"`)
	r.quiet(t)
}

// A run daemon that stops answering for a while says nothing about the jobs: nothing changed until it answers again.
func TestJobsTheDaemonCannotTellChangeNothing(t *testing.T) {
	jobs := &machine{jobs: []run.Job{{Name: "web", WorkDir: "/repo/.trees/feat/a", Up: true, State: run.Running}}}
	u := useCase(newBus(), &known{}, opening(newRepo("/repo", "feat/a")))
	u.Jobs, u.Recheck = jobs.Jobs, 20*time.Millisecond
	r, _ := watching(t, u, events.Request{Caller: domain.Caller{Dir: "/repo"}})
	r.nextType(t)
	r.nextType(t)

	jobs.set(nil, run.ErrUnknown)
	r.quiet(t)
	jobs.set([]run.Job{{Name: "web", WorkDir: "/repo/.trees/feat/a", Up: true, State: run.Running}}, nil)

	r.quiet(t)
}

func TestAWorktreeTurningDirtyThenCleanIsSentAsTheWorktreeChanged(t *testing.T) {
	app, git := newRepo("/repo", "feat/a").living()
	u := useCase(newBus(), &known{}, opening(app))
	u.Recheck = 20 * time.Millisecond
	r, _ := watching(t, u, events.Request{Caller: domain.Caller{Dir: "/repo"}})
	r.nextType(t)
	r.nextType(t)
	r.quiet(t)

	git.SetDirty("/repo/.trees/feat/a", true)
	changed := r.next(t)
	assert.Contains(t, changed, `"branch":"feat/a"`)
	assert.Contains(t, changed, `"changed":["dirty"]`)

	git.SetDirty("/repo/.trees/feat/a", false)
	assert.Contains(t, r.next(t), `"changed":["dirty"]`)
	r.quiet(t)
}

// A git status per worktree is costly: the poll reads it every fourth time, the legacy dashboard's 20 seconds; a poll that does not read it changes nothing.
func TestAPollReadsWhetherAWorktreeIsDirtyEveryFourthPollOnly(t *testing.T) {
	log := &callLog{}
	app, git := newRepo("/repo", "feat/a").living()
	u := useCase(newBus(), &known{}, openingWith(app, recorded{Fake: git, log: log}))
	u.Recheck = 20 * time.Millisecond
	r, _ := watching(t, u, events.Request{Caller: domain.Caller{Dir: "/repo"}})
	r.nextType(t)
	r.nextType(t)

	git.SetDirty("/repo/.trees/feat/a", true)
	assert.Contains(t, r.next(t), `"changed":["dirty"]`)
	r.quiet(t)

	polls := pollsIn(log.all())
	require.Greater(t, len(polls), 5)
	// the last poll may still be under way
	for i, calls := range polls[:len(polls)-1] {
		dirty := slices.ContainsFunc(calls, func(call string) bool { return strings.HasPrefix(call, "Dirty") })
		assert.Equal(t, (i+1)%4 == 0, dirty, "poll %d: %v", i+1, calls)
	}
}

// A worktree created between two reads of the dirtiness would start unknown, and its first change be taken for its first read.
func TestAWorktreeNewToThePollHasItsDirtinessReadAtOnce(t *testing.T) {
	log := &callLog{}
	app, git := newRepo("/repo").living()
	u := useCase(newBus(), &known{}, openingWith(app, recorded{Fake: git, log: log}))
	u.Recheck = 20 * time.Millisecond
	r, _ := watching(t, u, events.Request{Caller: domain.Caller{Dir: "/repo"}})
	r.nextType(t)
	r.nextType(t)

	require.NoError(t, git.AddWorktree(t.Context(), worktree.Addition{Path: "/repo/.trees/feat/b", Checkout: worktree.NewBranch{Name: "feat/b", From: worktree.LocalBranch{Name: "main"}}}))
	assert.Contains(t, r.next(t), `"changed":["head","checkout","jobs","dirty"]`)
	git.SetDirty("/repo/.trees/feat/b", true)

	changed := r.next(t)
	assert.Contains(t, changed, `"branch":"feat/b"`)
	assert.Contains(t, changed, `"changed":["dirty"]`)
	// the poll that found feat/b read it with the branch tips, not on a dirty pass that happened to follow
	readsB := func(call string) bool { return strings.HasPrefix(call, "Dirty ") && strings.HasSuffix(call, "feat/b") }
	for _, calls := range pollsIn(log.all()) {
		if i := slices.IndexFunc(calls, readsB); i >= 0 {
			assert.Equal(t, 1, i, "the poll that found feat/b: %v", calls)
			break
		}
	}
}

// A dirty read that fails is unknown, never clean: the worktree dirty at the snapshot, cleaned while git cannot tell, sends nothing.
func TestAWorktreeWhoseDirtyReadFailsChangesNothing(t *testing.T) {
	app := newRepo("/repo", "feat/a")
	app.Git.Dirty = []string{"/repo/.trees/feat/a"}
	app, git := app.living()
	failing := &atomic.Bool{}
	u := useCase(newBus(), &known{}, openingWith(app, recorded{Fake: git, failing: failing}))
	u.Recheck = 20 * time.Millisecond
	r, _ := watching(t, u, events.Request{Caller: domain.Caller{Dir: "/repo"}})
	r.nextType(t)
	r.nextType(t)

	failing.Store(true)
	git.SetDirty("/repo/.trees/feat/a", false)

	r.quiet(t)
}

// The dirty reads of a poll take a while: one may catch an operation of wtm halfway, a stash of sync dirty until it pops, and what the operation changed is its own events' even once it finished.
func TestAnOperationStartedDuringTheDirtyReadsDropsThem(t *testing.T) {
	app, git := newRepo("/repo", "feat/a", "feat/c").living()
	bus := newBus()
	var armed atomic.Bool
	var sub subscription
	before := func(path string) {
		switch {
		case !armed.Load():
		case strings.HasSuffix(path, "feat/a"):
			git.SetDirty("/repo/.trees/feat/a", true)
		case strings.HasSuffix(path, "feat/c"):
			armed.Store(false)
			git.SetDirty("/repo/.trees/feat/a", false)
			sub.Lines <- published(t, app.Repo, domain.OperationStarted{Command: "sync", Surface: "cli"})
			sub.Lines <- published(t, app.Repo, domain.UnitFinished{Subject: "feat/a", Status: domain.StatusDone})
			sub.Lines <- published(t, app.Repo, domain.OperationFinished{Command: "sync", Surface: "cli", OK: true})
		}
	}
	u := useCase(bus, &known{}, openingWith(app, recorded{Fake: git, log: &callLog{}, before: before}))
	u.Recheck = 20 * time.Millisecond
	r, _ := watching(t, u, events.Request{Caller: domain.Caller{Dir: "/repo"}})
	sub = bus.subscribed(t)
	r.nextType(t)
	r.nextType(t)

	armed.Store(true)

	assert.Equal(t, "operation.started", r.nextType(t))
	assert.Equal(t, "unit.finished", r.nextType(t))
	assert.Equal(t, "operation.finished", r.nextType(t))
	r.quiet(t)
}

func TestTheDirtinessAnOperationLeftOnABranchItNamedIsNeverSentAsPolled(t *testing.T) {
	app, git := newRepo("/repo", "feat/a").living()
	bus := newBus()
	u := useCase(bus, &known{}, opening(app))
	u.Recheck = 20 * time.Millisecond
	r, _ := watching(t, u, events.Request{Caller: domain.Caller{Dir: "/repo"}})
	sub := bus.subscribed(t)
	r.nextType(t)
	r.nextType(t)

	sub.Lines <- published(t, app.Repo, domain.OperationStarted{Command: "sync", Surface: "cli"})
	assert.Equal(t, "operation.started", r.nextType(t))
	git.SetDirty("/repo/.trees/feat/a", true)
	sub.Lines <- published(t, app.Repo, domain.UnitFinished{Subject: "feat/a", Status: domain.StatusFailed})
	sub.Lines <- published(t, app.Repo, domain.OperationFinished{Command: "sync", Surface: "cli"})

	assert.Equal(t, "unit.finished", r.nextType(t))
	assert.Equal(t, "operation.finished", r.nextType(t))
	r.quiet(t)

	git.SetDirty("/repo/.trees/feat/a", false)
	assert.Contains(t, r.next(t), `"changed":["dirty"]`)
}

func TestAWorktreeRemovedFromATerminalResetsTheStreamOnAFreshSnapshot(t *testing.T) {
	app, git := newRepo("/repo", "feat/a").living()
	u := useCase(newBus(), &known{}, opening(app))
	u.Recheck = 20 * time.Millisecond
	r, _ := watching(t, u, events.Request{Caller: domain.Caller{Dir: "/repo"}})
	r.nextType(t)
	r.nextType(t)

	require.NoError(t, git.RemoveWorktree(t.Context(), "/repo/.trees/feat/a"))

	snapshot := r.next(t)
	assert.Contains(t, snapshot, `"type":"snapshot"`)
	assert.NotContains(t, snapshot, "feat/a")
}

func TestTheGlobalStreamPollsEachRepositoryItWatches(t *testing.T) {
	a, git := newRepo("/a").living()
	u := useCase(newBus(), &known{repos: []domain.Repo{a.Repo}}, opening(a))
	u.Recheck = 20 * time.Millisecond
	r, _ := watching(t, u, events.Request{All: true})
	r.nextType(t)
	r.nextType(t)

	require.NoError(t, git.AddWorktree(t.Context(), worktree.Addition{Path: "/a/.trees/feat/b", Checkout: worktree.NewBranch{Name: "feat/b", From: worktree.LocalBranch{Name: "main"}}}))

	assert.Contains(t, r.next(t), `"type":"worktree.changed","ts":"2026-10-16T12:00:00Z","repo":{"root":"/a","common_dir":"/a/.git"},"worktree":{"branch":"feat/b"`)
}

// The poll sent worktree.changed after wtm's own operations: a create was followed by a changed of its worktree, every field.
func TestWhatAnOperationOfWtmChangedIsNeverSentAsPolled(t *testing.T) {
	app, git := newRepo("/repo").living()
	bus := newBus()
	u := useCase(bus, &known{}, opening(app))
	u.Recheck = 10 * time.Millisecond
	r, _ := watching(t, u, events.Request{Caller: domain.Caller{Dir: "/repo"}})
	sub := bus.subscribed(t)
	r.nextType(t)
	r.nextType(t)

	sub.Lines <- published(t, app.Repo, domain.OperationStarted{Command: "create", Surface: "cli"})
	assert.Equal(t, "operation.started", r.nextType(t))
	require.NoError(t, git.AddWorktree(t.Context(), worktree.Addition{Path: "/repo/.trees/feat/b", Checkout: worktree.NewBranch{Name: "feat/b", From: worktree.LocalBranch{Name: "main"}}}))
	time.Sleep(50 * time.Millisecond)
	sub.Lines <- published(t, app.Repo, domain.WorktreeCreated{Worktree: worktree.Identity{Branch: "feat/b", Path: "/repo/.trees/feat/b"}})
	sub.Lines <- published(t, app.Repo, domain.OperationFinished{Command: "create", Surface: "cli", OK: true})

	assert.Equal(t, "worktree.created", r.nextType(t))
	assert.Equal(t, "operation.finished", r.nextType(t))
	r.quiet(t)
}

// The pause of the poll during an operation of wtm covered the whole repository: a commit in another worktree, made from a terminal meanwhile, was never sent.
func TestWhatAnOperationDidNotNameIsSentOnceItFinishes(t *testing.T) {
	app := newRepo("/repo", "feat/a")
	app.Git.Local = []string{"feat/ahead"}
	app.Git.Commits = map[worktree.Ref][]string{worktree.LocalBranch{Name: "feat/ahead"}: {"c1"}}
	app, git := app.living()
	bus := newBus()
	u := useCase(bus, &known{}, opening(app))
	u.Recheck = 10 * time.Millisecond
	r, _ := watching(t, u, events.Request{Caller: domain.Caller{Dir: "/repo"}})
	sub := bus.subscribed(t)
	r.nextType(t)
	r.nextType(t)

	sub.Lines <- published(t, app.Repo, domain.OperationStarted{Command: "create", Surface: "cli"})
	assert.Equal(t, "operation.started", r.nextType(t))
	require.NoError(t, git.AddWorktree(t.Context(), worktree.Addition{Path: "/repo/.trees/feat/b", Checkout: worktree.NewBranch{Name: "feat/b", From: worktree.LocalBranch{Name: "main"}}}))
	require.NoError(t, git.MergeFFOnly(t.Context(), worktree.Merge{Path: "/repo/.trees/feat/a", Onto: worktree.LocalBranch{Name: "feat/ahead"}}))
	sub.Lines <- published(t, app.Repo, domain.WorktreeCreated{Worktree: worktree.Identity{Branch: "feat/b", Path: "/repo/.trees/feat/b"}})
	sub.Lines <- published(t, app.Repo, domain.OperationFinished{Command: "create", Surface: "cli", OK: true})

	assert.Equal(t, "worktree.created", r.nextType(t))
	assert.Equal(t, "operation.finished", r.nextType(t))
	changed := r.next(t)
	assert.Contains(t, changed, `"type":"worktree.changed"`)
	assert.Contains(t, changed, `"branch":"feat/a"`)
	assert.Contains(t, changed, `"changed":["head"]`)
	r.quiet(t)
}

// An operation whose process died never says it finished: past its limit, the poll runs again.
func TestAnOperationThatNeverFinishesHoldsThePollBackTenMinutesAtMost(t *testing.T) {
	app := newRepo("/repo", "feat/a")
	app.Git.Local = []string{"feat/ahead"}
	app.Git.Commits = map[worktree.Ref][]string{worktree.LocalBranch{Name: "feat/ahead"}: {"c1"}}
	app, git := app.living()
	bus := newBus()
	u := useCase(bus, &known{}, opening(app))
	clock := &ticking{now: fixedNow}
	u.Now, u.Recheck = clock.Now, 10*time.Millisecond
	r, _ := watching(t, u, events.Request{Caller: domain.Caller{Dir: "/repo"}})
	sub := bus.subscribed(t)
	r.nextType(t)
	r.nextType(t)
	sub.Lines <- published(t, app.Repo, domain.OperationStarted{Command: "create", Surface: "cli"})
	assert.Equal(t, "operation.started", r.nextType(t))

	require.NoError(t, git.MergeFFOnly(t.Context(), worktree.Merge{Path: "/repo/.trees/feat/a", Onto: worktree.LocalBranch{Name: "feat/ahead"}}))
	r.quiet(t)
	clock.advance(11 * time.Minute)

	assert.Contains(t, r.next(t), `"type":"worktree.changed"`)
}

// No event of an operation reports a removal: a worktree it named that went away meanwhile was removed by someone else, and its removal was never sent.
func TestAWorktreeRemovedDuringAnOperationThatNamedItIsSent(t *testing.T) {
	app, git := newRepo("/repo", "feat/a").living()
	bus := newBus()
	u := useCase(bus, &known{}, opening(app))
	u.Recheck = 10 * time.Millisecond
	r, _ := watching(t, u, events.Request{Caller: domain.Caller{Dir: "/repo"}})
	sub := bus.subscribed(t)
	r.nextType(t)
	r.nextType(t)

	sub.Lines <- published(t, app.Repo, domain.OperationStarted{Command: "extract", Surface: "cli"})
	sub.Lines <- published(t, app.Repo, domain.WorktreeUpdated{Worktree: worktree.Identity{Branch: "feat/a", Path: "/repo/.trees/feat/a"}, Changed: []worktree.IdentityField{worktree.IdentityOrdinal}})
	assert.Equal(t, "operation.started", r.nextType(t))
	assert.Equal(t, "worktree.updated", r.nextType(t))
	require.NoError(t, git.RemoveWorktree(t.Context(), "/repo/.trees/feat/a"))
	sub.Lines <- published(t, app.Repo, domain.OperationFinished{Command: "extract", Surface: "cli", OK: true})

	assert.Equal(t, "operation.finished", r.nextType(t))
	assert.Equal(t, "snapshot", r.nextType(t))
}
