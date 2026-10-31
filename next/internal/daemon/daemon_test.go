package daemon

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAServiceIsUpOnceStartedListedIndexedAndAnnounced(t *testing.T) {
	w := newWorld(t, nil)

	job, err := w.Start(service("web"), nil)

	require.NoError(t, err)
	want := run.Job{Name: "web", WorkDir: "/wt/a", Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn, PID: 1000, URL: "http://localhost:3100", Ports: map[string]int{"PORT": 3100}, StartedAt: now}
	assert.Equal(t, want, job)
	assert.Equal(t, []run.Job{want}, w.List(""))
	assert.Equal(t, []string{"/wt/a:web"}, w.index.names())
	assert.True(t, w.Busy())
	require.Len(t, w.bus.lines, 1)
	assert.Equal(t, domain.Line{V: domain.StreamVersion, TS: "2026-10-31T12:00:00Z", Repo: &domain.Repo{Root: "/repo", CommonDir: "/repo/.git"}, CorrelationID: "start-1",
		Event: domain.JobStarted{JobChange: domain.JobChange{Worktree: domain.JobWorktree{Branch: "feat/a", Path: "/wt/a"}, Job: domain.EventJob{Name: "web", Kind: project.Service, URL: "http://localhost:3100", Hold: run.HoldOwn}}}}, w.bus.lines[0])
}

func TestTheServiceRunsWithItsCommandWhereAndWithWhatItsSpecSays(t *testing.T) {
	w := newWorld(t, nil)
	spec := service("web")
	spec.Job.Cwd, spec.Unset = "apps/web", []string{"PORT"}

	_, err := w.Start(spec, nil)

	require.NoError(t, err)
	command := w.spawner.commands[0]
	assert.Equal(t, run.Command{Line: "serve web", Dir: "/wt/a/apps/web", Env: map[string]string{"PORT": "3100", "TERM": "xterm-256color", "COLORTERM": "truecolor", "FORCE_COLOR": "1"}, Unset: []string{"PORT"}, Terminal: true}, run.Command{Line: command.Line, Dir: command.Dir, Env: command.Env, Unset: command.Unset, Terminal: command.Terminal})
}

func TestAServiceThatEndsOnItsOwnCrashesWithItsLastLines(t *testing.T) {
	w := newWorld(t, map[string]script{"serve web": exits(1, "listening", "EADDRINUSE")})

	_, err := w.Start(service("web"), nil)
	require.NoError(t, err)
	job := w.settled(t, "web", run.Crashed)

	assert.False(t, job.Up)
	assert.Equal(t, 1, *job.ExitCode)
	assert.Empty(t, w.index.names())
	assert.False(t, w.Busy())
	require.Eventually(t, func() bool { return len(w.bus.events()) == 2 }, time.Second, time.Millisecond)
	crashed, ok := w.bus.events()[1].(domain.JobCrashed)
	require.True(t, ok)
	assert.Equal(t, []string{"listening", "EADDRINUSE"}, crashed.LastLines)
	assert.Equal(t, 1, *crashed.ExitCode)
}

func TestAStartTimeIsGivenOnlyWhileTheJobIsUp(t *testing.T) {
	w := newWorld(t, map[string]script{"serve web": exits(1)})

	_, err := w.Start(service("web"), nil)
	require.NoError(t, err)

	assert.Zero(t, w.settled(t, "web", run.Crashed).StartedAt, "a job that ended has no uptime")
}

func TestATaskHandsOverEachLineAndEndsExited(t *testing.T) {
	w := newWorld(t, map[string]script{"run migrate": exits(0, "\x1b[32m2 migrations\x1b[0m", "done")})
	var lines []string

	job, err := w.Start(task("migrate"), func(line run.Line) { lines = append(lines, line.Text) })

	require.NoError(t, err)
	assert.Equal(t, run.Exited, job.State)
	assert.Equal(t, []string{"2 migrations", "done"}, lines)
	assert.Empty(t, w.index.names(), "a task is never indexed")
	assert.IsType(t, domain.JobExited{}, w.bus.events()[1])
}

func TestATaskThatFailsFailsTheStartWithItsExitCode(t *testing.T) {
	w := newWorld(t, map[string]script{"run migrate": exits(3, "no database")})

	job, err := w.Start(task("migrate"), nil)

	var failure *run.Error
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, run.CodeTaskFailed, failure.Code)
	assert.Equal(t, 3, *failure.ExitCode)
	assert.Equal(t, run.Crashed, job.State)
}

func TestAJobAlreadyRunningInItsWorktreeIsRefused(t *testing.T) {
	w := newWorld(t, nil)
	_, err := w.Start(service("web"), nil)
	require.NoError(t, err)

	_, err = w.Start(service("web"), nil)

	var refusal *run.Error
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, run.CodeJobRunning, refusal.Code)
	other := service("web")
	other.WorkDir = "/wt/b"
	_, err = w.Start(other, nil)
	assert.NoError(t, err, "the same job runs in each worktree")
}

func TestAJobThatCannotStartIsRefused(t *testing.T) {
	cases := map[string]struct {
		spec run.Spec
		fail bool
		code string
	}{
		"empty_cmd":    {spec: run.Spec{Job: project.Job{Name: "web", Kind: project.Service, Cmd: "  "}, WorkDir: "/wt/a"}, code: run.CodeEmptyCommand},
		"spawn_failed": {spec: service("web"), fail: true, code: run.CodeLaunchFailed},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t, nil)
			w.spawner.fail = c.fail

			_, err := w.Start(c.spec, nil)

			var refusal *run.Error
			require.ErrorAs(t, err, &refusal)
			assert.Equal(t, c.code, refusal.Code)
			assert.Empty(t, w.List(""))
		})
	}
}

func TestStopStopsTheServicesGroupAndAnnouncesItForTheStoppingCommand(t *testing.T) {
	w := newWorld(t, nil)
	_, err := w.Start(service("web"), nil)
	require.NoError(t, err)

	outcome, err := w.Stop(run.Stop{Name: "web", WorkDir: "/wt/a", CorrelationID: "stop-1"})

	require.NoError(t, err)
	assert.Equal(t, run.OutcomeStopped, outcome)
	assert.Equal(t, run.Stopped, w.job(t, "web").State)
	assert.Equal(t, []time.Duration{time.Millisecond}, w.spawner.process("serve web").stops)
	assert.Empty(t, w.index.names())
	assert.Equal(t, "stop-1", w.bus.lines[1].CorrelationID)
	assert.IsType(t, domain.JobStopped{}, w.bus.lines[1].Event)
}

func TestStoppingAJobThatIsNotUpSaysNothingWasRunning(t *testing.T) {
	w := newWorld(t, map[string]script{"serve web": exits(1)})
	_, err := w.Start(service("web"), nil)
	require.NoError(t, err)
	w.settled(t, "web", run.Crashed)

	unknown, err := w.Stop(run.Stop{Name: "nope", WorkDir: "/x"})
	require.NoError(t, err)
	ended, err := w.Stop(run.Stop{Name: "web", WorkDir: "/wt/a"})
	require.NoError(t, err)

	assert.Equal(t, run.OutcomeNotRunning, unknown)
	assert.Equal(t, run.OutcomeNotRunning, ended)
	assert.Empty(t, w.List(""), "a job that ended leaves the listing once stopped")
}

func TestADetachedLauncherLeavesItsStackUpWithNoProcessOfTheDaemons(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0, "Container db Started")})
	var lines []string

	job, err := w.Start(stack("db"), func(line run.Line) { lines = append(lines, line.Text) })

	require.NoError(t, err)
	assert.Equal(t, run.Job{Name: "db", WorkDir: "/wt/a", Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldDetached, Ports: map[string]int{}, StartedAt: now}, job)
	assert.Equal(t, []string{"Container db Started"}, lines)
	assert.False(t, w.spawner.commands[0].Terminal)
	assert.False(t, w.Busy(), "a stack keeps no daemon up")
	assert.Equal(t, []string{"/wt/a:db"}, w.index.names())
}

func TestADetachedStackStopsWithItsStopCommand(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(0)})
	_, err := w.Start(stack("db"), nil)
	require.NoError(t, err)

	outcome, err := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})

	require.NoError(t, err)
	assert.Equal(t, run.OutcomeStopped, outcome)
	assert.Equal(t, []string{"up db", "down db"}, w.spawner.lines())
	assert.Equal(t, run.Stopped, w.job(t, "db").State)
	assert.Empty(t, w.index.names())
}

func TestAStopCommandThatFailsLeavesTheStackUp(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(2)})
	_, err := w.Start(stack("db"), nil)
	require.NoError(t, err)

	_, err = w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})

	var failure *run.Error
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, run.CodeStopFailed, failure.Code)
	assert.True(t, w.job(t, "db").Up)
}

func TestALauncherThatFailsGivesTheWorktreeBackTheStackItHad(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0)})
	_, err := w.Start(stack("db"), nil)
	require.NoError(t, err)
	w.spawner.scripts["up db"] = exits(1, "port is already allocated")

	_, err = w.Start(stack("db"), nil)

	var failure *run.Error
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, run.CodeLaunchFailed, failure.Code)
	assert.True(t, w.job(t, "db").Up)
	assert.Equal(t, []string{"/wt/a:db"}, w.index.names())
}

func TestStopAllStopsEveryJobUpOfTheWorktreeTogether(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(0)})
	for _, spec := range []run.Spec{service("web"), service("api"), stack("db")} {
		_, err := w.Start(spec, nil)
		require.NoError(t, err)
	}
	elsewhere := service("web")
	elsewhere.WorkDir = "/wt/b"
	_, err := w.Start(elsewhere, nil)
	require.NoError(t, err)

	stopped, err := w.StopAll(run.StopAll{WorkDir: "/wt/a"})

	require.NoError(t, err)
	var names []string
	for _, s := range stopped {
		assert.Equal(t, run.OutcomeStopped, s.Outcome)
		names = append(names, s.Job.Name)
	}
	assert.Equal(t, []string{"api", "db", "web"}, names)
	assert.Equal(t, []string{"/wt/b:web"}, w.index.names())
}

func TestShutdownStopsTheDaemonsProcessesAndLeavesStacksAndClaims(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0)})
	w.index.records = []run.Record{{Claim: true, SharedDir: "/repo", Spec: run.Spec{Job: stack("db").Job, WorkDir: "/wt/b"}}}
	w.Adopt()
	for _, spec := range []run.Spec{service("web"), stack("db")} {
		_, err := w.Start(spec, nil)
		require.NoError(t, err)
	}

	w.Shutdown()

	assert.Equal(t, run.Stopped, w.job(t, "web").State)
	assert.Equal(t, []string{"/wt/a:db", "/wt/b:db"}, w.index.names())
	_, err := w.Start(service("api"), nil)
	var refusal *run.Error
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, run.CodeStopping, refusal.Code)
}

func TestAJobStartedWithNoOriginPublishesNothing(t *testing.T) {
	w := newWorld(t, nil)
	spec := service("web")
	spec.Origin = nil

	_, err := w.Start(spec, nil)

	require.NoError(t, err)
	assert.Empty(t, w.bus.events())
}

func TestAdoptReapsOrphansListsCrashedAndKeepsStacksAndClaims(t *testing.T) {
	w := newWorld(t, nil)
	record := func(spec run.Spec, pgid int) run.Record {
		return run.Record{Spec: spec, StartedAt: now, PID: pgid, PGID: pgid}
	}
	gone := service("old")
	gone.WorkDir = "/wt/gone"
	w.gone["/wt/gone"] = true
	w.index.records = []run.Record{
		record(service("web"), 100),
		record(service("api"), 200),
		record(service("worker"), 300),
		record(gone, 400),
		record(stack("db"), 0),
		{Claim: true, SharedDir: "/repo", Spec: run.Spec{Job: stack("cache").Job, WorkDir: "/wt/a"}},
	}
	w.orphans.groups = map[int]run.Group{100: {Alive: true, Identified: true}, 300: {Alive: true}}

	w.Adopt()

	assert.Equal(t, []int{100}, w.orphans.reaped, "only a group identified as the job's is reaped")
	assert.Equal(t, []run.Job{
		{Name: "api", WorkDir: "/wt/a", Kind: project.Service, State: run.Crashed, Hold: run.HoldOwn, URL: "http://localhost:3100", Ports: map[string]int{"PORT": 3100}},
		{Name: "cache", WorkDir: "/wt/a", Up: true, Kind: project.Service, Hold: run.HoldClaim, Ports: map[string]int{}, SharedDir: "/repo"},
		{Name: "db", WorkDir: "/wt/a", Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldDetached, Ports: map[string]int{}, StartedAt: now},
		{Name: "web", WorkDir: "/wt/a", Kind: project.Service, State: run.Stopped, Hold: run.HoldOwn, Reaped: true, URL: "http://localhost:3100", Ports: map[string]int{"PORT": 3100}},
	}, w.List(""))
	assert.Equal(t, []string{"/wt/a:cache", "/wt/a:db"}, w.index.names())
}

func TestAStackStaysIndexedWhileItsLauncherRunsAgain(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0)})
	_, err := w.Start(stack("db"), nil)
	require.NoError(t, err)
	delete(w.spawner.scripts, "up db")

	go func() { _, _ = w.Start(stack("db"), nil) }()
	require.Eventually(t, func() bool { return len(w.spawner.lines()) == 2 }, time.Second, time.Millisecond)
	w.save()

	assert.Equal(t, []string{"/wt/a:db"}, w.index.names(), "a daemon killed now would lose the stack")
	w.spawner.process("up db").end(0)
}

func TestAShutdownDuringARelaunchHandsTheStackOver(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0)})
	_, err := w.Start(stack("db"), nil)
	require.NoError(t, err)
	delete(w.spawner.scripts, "up db")
	relaunched := make(chan error)
	go func() { _, err := w.Start(stack("db"), nil); relaunched <- err }()
	require.Eventually(t, func() bool { return len(w.spawner.lines()) == 2 }, time.Second, time.Millisecond)

	w.Shutdown()

	require.NoError(t, <-relaunched)
	assert.True(t, w.job(t, "db").Up)
	assert.Equal(t, []string{"/wt/a:db"}, w.index.names())
	assert.Equal(t, []string{"up db", "up db"}, w.spawner.lines(), "the stack is left to the next daemon, never stopped")
}

func TestAStopDuringARelaunchStopsTheStackItself(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(0)})
	_, err := w.Start(stack("db"), nil)
	require.NoError(t, err)
	delete(w.spawner.scripts, "up db")
	go func() { _, _ = w.Start(stack("db"), nil) }()
	require.Eventually(t, func() bool { return len(w.spawner.lines()) == 2 }, time.Second, time.Millisecond)

	outcome, err := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})

	require.NoError(t, err)
	assert.Equal(t, run.OutcomeStopped, outcome)
	assert.Equal(t, []string{"up db", "up db", "down db"}, w.spawner.lines())
	assert.Equal(t, run.Stopped, w.job(t, "db").State)
	assert.Empty(t, w.index.names())
}

func TestAStopCommandThatFailsDuringARelaunchLeavesTheStackUpAndIndexed(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(2)})
	_, err := w.Start(stack("db"), nil)
	require.NoError(t, err)
	delete(w.spawner.scripts, "up db")
	go func() { _, _ = w.Start(stack("db"), nil) }()
	require.Eventually(t, func() bool { return len(w.spawner.lines()) == 2 }, time.Second, time.Millisecond)

	outcome, err := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})

	var failure *run.Error
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, run.CodeStopFailed, failure.Code)
	assert.Empty(t, outcome)
	assert.True(t, w.job(t, "db").Up)
	assert.Equal(t, []string{"/wt/a:db"}, w.index.names())
	w.spawner.scripts["down db"] = exits(0)
	retried, err := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})
	require.NoError(t, err)
	assert.Equal(t, run.OutcomeStopped, retried, "a stop tried again still reaches the stack")
}

func TestAServiceStartedWhereAStackWasIsIndexedAsItself(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0)})
	_, err := w.Start(stack("db"), nil)
	require.NoError(t, err)
	foreground := stack("db")
	foreground.Job.Stop, foreground.Job.Cmd = "", "postgres"

	_, err = w.Start(foreground, nil)
	require.NoError(t, err)

	records := w.index.Load()
	require.Len(t, records, 1)
	assert.Equal(t, run.HoldOwn, records[0].Hold())
	assert.Equal(t, "postgres", records[0].Spec.Job.Cmd)
}

func TestAShutdownDuringAStacksFirstLaunchLeavesItIndexed(t *testing.T) {
	w := newWorld(t, nil)
	launched := make(chan error)
	go func() { _, err := w.Start(stack("db"), nil); launched <- err }()
	require.Eventually(t, func() bool { return len(w.spawner.lines()) == 1 }, time.Second, time.Millisecond)

	w.Shutdown()

	require.NoError(t, <-launched)
	assert.True(t, w.job(t, "db").Up, "the stack may be partly up: the next daemon's stop takes it down")
	assert.Equal(t, []string{"/wt/a:db"}, w.index.names())
}

func TestAStopWhoseStackOthersTookDownMeanwhileEndsQuietly(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(0)})
	first := w.relaunching(t)
	launcher := w.spawner.process("up db")
	launcher.afterStop = func() {
		w.given(t, first)
		_, err := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})
		require.NoError(t, err)
		_, err = w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})
		require.NoError(t, err)
	}

	outcome, err := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})

	require.NoError(t, err)
	assert.Equal(t, run.OutcomeStopped, outcome)
	assert.Equal(t, []string{"up db", "up db", "down db"}, w.spawner.lines(), "the stack is stopped once")
}

func TestAStopNeverRunsTheStopCommandOnALaunchStartedMeanwhile(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(0)})
	first := w.relaunching(t)
	launcher := w.spawner.process("up db")
	launcher.afterStop = func() {
		w.given(t, first)
		go func() { _, _ = w.Start(stack("db"), nil) }()
		require.Eventually(t, func() bool { return len(w.spawner.lines()) == 3 }, time.Second, time.Millisecond)
	}

	outcome, err := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})

	require.NoError(t, err)
	assert.Equal(t, run.OutcomeStopped, outcome)
	assert.Equal(t, []string{"up db", "up db", "up db"}, w.spawner.lines(), "the new launch is not stopped under it")
	w.spawner.process("up db").end(0)
}
