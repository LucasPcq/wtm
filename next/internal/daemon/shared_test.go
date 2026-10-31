package daemon

import (
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestASharedJobRunsOnceInItsMainCheckoutAndEachWorktreeClaimsIt(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0)})

	a, err := w.Start(shared("db", "/wt/a", "down db"), nil)
	require.NoError(t, err)
	b, err := w.Start(shared("db", "/wt/b", "down db"), nil)
	require.NoError(t, err)

	assert.Equal(t, run.HoldClaim, a.Hold)
	assert.Equal(t, "/repo", a.SharedDir)
	assert.True(t, b.Up)
	assert.Equal(t, []string{"up db"}, w.spawner.lines(), "it runs once")
	service := w.List("/repo")
	require.Len(t, service, 1)
	assert.Equal(t, run.HoldDetached, service[0].Hold)
	assert.Equal(t, []string{"/repo:db", "/wt/a:db", "/wt/b:db"}, w.index.names())
}

func TestASharedJobStopsOnceItsLastHolderLetsGo(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(0)})
	for _, at := range []string{"/wt/a", "/wt/b"} {
		_, err := w.Start(shared("db", at, "down db"), nil)
		require.NoError(t, err)
	}

	first, err := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})
	require.NoError(t, err)
	last, err := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/b"})
	require.NoError(t, err)

	assert.Equal(t, run.OutcomeReleased, first)
	assert.Equal(t, run.OutcomeStopped, last)
	assert.Equal(t, []string{"up db", "down db"}, w.spawner.lines())
	assert.Empty(t, w.index.names())
}

func TestTheMainCheckoutHoldingASharedJobKeepsItUpPastItsLastClaim(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(0)})
	_, err := w.Start(shared("db", "/wt/a", "down db"), nil)
	require.NoError(t, err)

	_, err = w.Start(shared("db", "/repo", "down db"), nil)
	require.NoError(t, err)
	released, err := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})
	require.NoError(t, err)

	assert.Equal(t, run.OutcomeReleased, released, "main joined the job a worktree started, and holds it")
	assert.Equal(t, []string{"up db"}, w.spawner.lines())
	stopped, err := w.Stop(run.Stop{Name: "db", WorkDir: "/repo"})
	require.NoError(t, err)
	assert.Equal(t, run.OutcomeStopped, stopped)
	assert.Equal(t, []string{"up db", "down db"}, w.spawner.lines())
}

func TestASharedJobAlreadyHeldInAWorktreeIsRefused(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0)})
	_, err := w.Start(shared("db", "/wt/a", "down db"), nil)
	require.NoError(t, err)

	_, err = w.Start(shared("db", "/wt/a", "down db"), nil)

	var refusal *run.Error
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, run.CodeJobRunning, refusal.Code)
}

func TestASharedJobWithNoMainCheckoutIsRefused(t *testing.T) {
	w := newWorld(t, nil)
	spec := shared("db", "/wt/a", "down db")
	spec.Shared = nil

	_, err := w.Start(spec, nil)

	var refusal *run.Error
	require.ErrorAs(t, err, &refusal)
	assert.Equal(t, run.CodeNoMainCheckout, refusal.Code)
	assert.Empty(t, w.spawner.lines(), "never one instance per worktree")
}

func TestEachWorktreesNamespaceIsCreatedRetriedWhileTheServiceComesUpAndNoted(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": prints("listening"), "createdb": failsFirst(2, "the database system is starting up")})
	spec := shared("db", "/wt/a", "")
	spec.Job.Namespace = &project.Namespace{Name: "app_{worktree}", Create: "createdb"}

	_, err := w.Start(spec, nil)

	require.NoError(t, err)
	assert.Equal(t, []string{"up db", "createdb", "createdb", "createdb"}, w.spawner.lines())
	create := w.spawner.commands[1]
	assert.Equal(t, "/wt/a", create.Dir)
	assert.Equal(t, "app_a", create.Env["WTM_NAMESPACE"])
	var lines []string
	_, err = w.Lines(t.Context(), run.Tail{Name: "db", WorkDir: "/wt/a"}, func(line run.Line) { lines = append(lines, line.Text) })
	require.NoError(t, err)
	assert.Contains(t, lines, "[wtm] namespace app_a ready for a", "a claim's lines are its service's")
}

func TestANamespaceThatCannotBeMadeTakesTheHoldBackAndTheJobWithIt(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(0), "createdb": failsFirst(1000, "connection refused")})
	spec := shared("db", "/wt/a", "down db")
	spec.Job.Namespace = &project.Namespace{Name: "app_{worktree}", Create: "createdb"}

	_, err := w.Start(spec, nil)

	var failure *run.Error
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, run.CodeNamespace, failure.Code)
	assert.Contains(t, failure.Message, "connection refused")
	assert.Contains(t, w.spawner.lines(), "down db", "nobody else held it")
	assert.Empty(t, w.index.names())
}

func TestTheEventsOfASharedJobNameTheWorktreesHoldingIt(t *testing.T) {
	w := newWorld(t, nil)
	for _, at := range []string{"/wt/a", "/wt/b"} {
		_, err := w.Start(shared("db", at, ""), nil)
		require.NoError(t, err)
	}

	w.spawner.process("up db").end(1)

	require.Eventually(t, func() bool {
		for _, event := range w.bus.events() {
			if _, ok := event.(domain.JobCrashed); ok {
				return true
			}
		}
		return false
	}, time.Second, time.Millisecond)
	var crashed domain.JobCrashed
	var claims int
	for _, event := range w.bus.events() {
		switch e := event.(type) {
		case domain.JobCrashed:
			crashed = e
		case domain.JobStarted:
			if e.Job.Hold == run.HoldClaim {
				claims++
			}
		}
	}
	assert.Equal(t, 2, claims, "each claim is announced")
	assert.Equal(t, []domain.JobWorktree{{Branch: "feat/a", Path: "/wt/a"}, {Branch: "feat/b", Path: "/wt/b"}}, crashed.HeldBy)
	assert.Equal(t, domain.JobWorktree{Branch: "main", Path: "/repo"}, crashed.Worktree)
}

func TestASharedJobThatEndsTakesItsClaimsAndIsStartedAgainFromAWorktree(t *testing.T) {
	w := newWorld(t, nil)
	for _, at := range []string{"/wt/a", "/wt/b"} {
		_, err := w.Start(shared("db", at, ""), nil)
		require.NoError(t, err)
	}

	w.spawner.process("up db").end(3)
	require.Eventually(t, func() bool { return len(w.List("/wt/a")) == 0 }, time.Second, time.Millisecond)
	again, err := w.Start(shared("db", "/wt/a", ""), nil)

	require.NoError(t, err, "no claim left refuses the start")
	assert.Equal(t, run.HoldClaim, again.Hold)
	assert.Equal(t, []string{"up db", "up db"}, w.spawner.lines())
}

func TestAHoldANamespaceTookBackIsNeverAnnounced(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(0), "createdb": failsFirst(1000, "connection refused")})
	spec := shared("db", "/wt/a", "down db")
	spec.Job.Namespace = &project.Namespace{Name: "app_{worktree}", Create: "createdb"}

	_, err := w.Start(spec, nil)
	require.Error(t, err)

	for _, event := range w.bus.events() {
		if stopped, ok := event.(domain.JobStopped); ok {
			assert.NotEqual(t, run.HoldClaim, stopped.Job.Hold, "a claim never announced is never announced gone")
		}
	}
}

func TestAClaimDuringTheLastHoldsReleaseWaitsAndStartsTheJobAgain(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(0)})
	waited := w.waits(t)
	_, err := w.Start(shared("db", "/wt/a", "down db"), nil)
	require.NoError(t, err)
	resume := w.index.stalling()
	stopped := make(chan run.Outcome, 1)
	go func() {
		outcome, _ := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})
		stopped <- outcome
	}()
	// Held between the release that found nobody holding the job and its stop.
	require.Eventually(t, func() bool { return w.index.holding() == 1 }, time.Second, time.Millisecond)

	claim := w.claimed("/wt/b")
	waited()
	assert.Empty(t, w.List("/wt/b"), "no claim lands on a job being stopped")
	resume()

	assert.Equal(t, run.OutcomeStopped, <-stopped)
	require.NoError(t, <-claim)
	assert.Equal(t, []string{"up db", "down db", "up db"}, w.spawner.lines())
	require.Len(t, w.List("/wt/b"), 1)
	assert.True(t, w.List("/wt/b")[0].Up)
}

func TestAClaimDuringAFailingFirstLaunchWaitsAndLaunchesItAgain(t *testing.T) {
	up := make(chan struct{})
	w := newWorld(t, map[string]script{"up db": holds(up, 1)})
	waited := w.waits(t)
	first := w.claimed("/wt/a")
	require.Eventually(t, func() bool { return len(w.spawner.lines()) == 1 }, time.Second, time.Millisecond)
	w.spawner.mu.Lock()
	w.spawner.scripts["up db"] = exits(0)
	w.spawner.mu.Unlock()

	second := w.claimed("/wt/b")
	waited()
	assert.Empty(t, w.List("/wt/b"), "no claim lands on a launch that may fail")
	close(up)

	var refusal *run.Error
	require.ErrorAs(t, <-first, &refusal)
	assert.Equal(t, run.CodeLaunchFailed, refusal.Code)
	require.NoError(t, <-second)
	assert.Equal(t, []string{"up db", "up db"}, w.spawner.lines())
	assert.Empty(t, w.List("/wt/a"))
	require.Len(t, w.List("/wt/b"), 1)
	assert.True(t, w.List("/wt/b")[0].Up)
}

func TestTwoFirstClaimsAtOnceLaunchTheJobOnce(t *testing.T) {
	up := make(chan struct{})
	w := newWorld(t, map[string]script{"up db": holds(up, 0)})
	waited := w.waits(t)

	a, b := w.claimed("/wt/a"), w.claimed("/wt/b")
	waited()
	close(up)

	require.NoError(t, <-a)
	require.NoError(t, <-b)
	assert.Equal(t, []string{"up db"}, w.spawner.lines())
	assert.Equal(t, []string{"/repo:db", "/wt/a:db", "/wt/b:db"}, w.index.names())
}

func TestAHoldLetGoOfTwiceIsLetGoOfOnce(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(0)})
	for _, at := range []string{"/wt/a", "/wt/b"} {
		_, err := w.Start(shared("db", at, "down db"), nil)
		require.NoError(t, err)
	}
	a, b := w.jobOf(key{workDir: "/wt/a", name: "db"}), w.jobOf(key{workDir: "/wt/b", name: "db"})

	released, err := w.release(a, "")
	require.NoError(t, err)
	assert.Equal(t, run.OutcomeReleased, released, "the other claim still holds it")
	again, err := w.release(a, "")
	require.NoError(t, err)
	assert.Equal(t, run.OutcomeNotRunning, again)
	stops := 0
	for _, event := range w.bus.events() {
		if stopped, ok := event.(domain.JobStopped); ok && stopped.Job.Hold == run.HoldClaim {
			stops++
		}
	}
	assert.Equal(t, 1, stops, "a claim is announced gone once")
	resume := w.index.stalling()
	var releasing sync.WaitGroup
	release := func() {
		releasing.Go(func() {
			_, err := w.release(b, "")
			assert.NoError(t, err)
		})
	}
	release()
	// Held between the release that found nobody holding the job and its stop: a second release finds nothing to stop.
	require.Eventually(t, func() bool { return w.index.holding() == 1 }, time.Second, time.Millisecond)
	w.mu.Lock()
	assert.Nil(t, w.realLocked(key{workDir: "/repo", name: "db"}))
	w.mu.Unlock()
	release()
	resume()
	releasing.Wait()

	assert.Equal(t, []string{"up db", "down db"}, w.spawner.lines(), "stopped once")
}

func TestAStaleReleaseLeavesTheWorktreesNewerClaim(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(0)})
	for _, at := range []string{"/wt/a", "/wt/b"} {
		_, err := w.Start(shared("db", at, "down db"), nil)
		require.NoError(t, err)
	}
	old := w.jobOf(key{workDir: "/wt/a", name: "db"})
	_, err := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})
	require.NoError(t, err)
	_, err = w.Start(shared("db", "/wt/a", "down db"), nil)
	require.NoError(t, err)

	_, err = w.release(old, "")

	require.NoError(t, err)
	require.Len(t, w.List("/wt/a"), 1)
	assert.True(t, w.List("/wt/a")[0].Up)
}

func TestANamespaceCreateStillRunningAfterFiveMinutesIsStopped(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(0)})
	w.params.NamespaceBudget = time.Nanosecond
	var limits []time.Duration
	w.params.After = func(limit time.Duration) <-chan time.Time {
		limits = append(limits, limit)
		if len(limits) > 1 {
			return nil
		}
		fell := make(chan time.Time, 1)
		fell <- now
		return fell
	}
	spec := shared("db", "/wt/a", "down db")
	spec.Job.Namespace = &project.Namespace{Name: "app_{worktree}", Create: "createdb"}

	_, err := w.Start(spec, nil)

	var failure *run.Error
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, run.CodeNamespace, failure.Code)
	assert.Equal(t, 5*time.Minute, limits[0], "an attempt runs up to five minutes")
	assert.Contains(t, failure.Message, "still running after 5m0s, stopped")
	assert.Equal(t, []string{"up db", "createdb", "down db"}, w.spawner.lines())
	assert.NotEmpty(t, w.spawner.process("createdb").stops)
}

func TestANamespaceCreateOutlastingItsBudgetIsWaitedFor(t *testing.T) {
	made := make(chan struct{})
	w := newWorld(t, map[string]script{"up db": exits(0), "createdb": holds(made, 0)})
	w.params.NamespaceBudget = time.Nanosecond
	spec := shared("db", "/wt/a", "down db")
	spec.Job.Namespace = &project.Namespace{Name: "app_{worktree}", Create: "createdb"}
	result := make(chan error, 1)
	go func() {
		_, err := w.Start(spec, nil)
		result <- err
	}()
	require.Eventually(t, func() bool { return slices.Contains(w.spawner.lines(), "createdb") }, time.Second, time.Millisecond)

	close(made)

	require.NoError(t, <-result, "the budget bounds when an attempt starts, not how long it runs")
	assert.Empty(t, w.spawner.process("createdb").stops)
}

func TestAShutdownStopsANamespaceCreateThatHangs(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": prints("listening")})
	w.params.NamespaceBudget = time.Minute
	spec := shared("db", "/wt/a", "")
	spec.Job.Namespace = &project.Namespace{Name: "app_{worktree}", Create: "createdb"}
	result := make(chan error, 1)
	go func() {
		_, err := w.Start(spec, nil)
		result <- err
	}()
	require.Eventually(t, func() bool { return slices.Contains(w.spawner.lines(), "createdb") }, time.Second, time.Millisecond)

	w.Shutdown()

	select {
	case err := <-result:
		var refusal *run.Error
		require.ErrorAs(t, err, &refusal)
		assert.Equal(t, run.CodeStopping, refusal.Code)
		assert.NotEmpty(t, w.spawner.process("createdb").stops)
	case <-time.After(2 * time.Second):
		require.FailNow(t, "the create still holds its start")
	}
}

func TestAStartWaitingOnAStacksStopPollsIt(t *testing.T) {
	down := make(chan struct{})
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": holds(down, 0)})
	var waits atomic.Int64
	w.waited = func() { waits.Add(1) }
	_, err := w.Start(shared("db", "/wt/a", "down db"), nil)
	require.NoError(t, err)
	go func() { _, _ = w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"}) }()
	require.Eventually(t, func() bool { return slices.Contains(w.spawner.lines(), "down db") }, time.Second, time.Millisecond)

	claim := w.claimed("/wt/b")
	require.Eventually(t, func() bool { return waits.Load() > 0 }, time.Second, time.Millisecond)
	time.Sleep(10 * pendingPoll)
	close(down)

	require.NoError(t, <-claim)
	assert.Less(t, waits.Load(), int64(30), "a stack's stop has no channel: it is polled, never spun on")
}

func TestAFailedNamespaceLetsGoOfItsOwnHoldNeverTheWorktreesNewerOne(t *testing.T) {
	first := make(chan struct{})
	var mu sync.Mutex
	calls := 0
	createdb := func(p *fakeProcess, _ func(string)) {
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()
		switch call {
		case 1:
			<-first
		case 2:
			p.end(0)
			return
		}
		p.end(1)
	}
	w := newWorld(t, map[string]script{"createdb": createdb})
	w.params.NamespaceBudget = time.Second
	spec := shared("db", "/wt/b", "")
	spec.Job.Namespace = &project.Namespace{Name: "app_{worktree}", Create: "createdb"}
	stale := make(chan error, 1)
	go func() {
		_, err := w.Start(spec, nil)
		stale <- err
	}()
	require.Eventually(t, func() bool { return len(w.spawner.lines()) == 2 }, time.Second, time.Millisecond)
	w.spawner.process("up db").end(1)
	require.Eventually(t, func() bool { return len(w.List("/wt/b")) == 0 }, time.Second, time.Millisecond)
	_, err := w.Start(spec, nil)
	require.NoError(t, err)

	close(first)

	var failure *run.Error
	require.ErrorAs(t, <-stale, &failure)
	assert.Equal(t, run.CodeNamespace, failure.Code)
	require.Len(t, w.List("/wt/b"), 1)
	assert.True(t, w.List("/wt/b")[0].Up)
	require.Len(t, w.List("/repo"), 1)
	assert.True(t, w.List("/repo")[0].Up, "the newer claim still holds it")
}

func TestAHoldGoneWhileItsNamespaceIsMadeFailsTheStart(t *testing.T) {
	for _, gone := range []struct {
		name string
		do   func(t *testing.T, w *world)
	}{
		{"stopped", func(_ *testing.T, w *world) { _, _ = w.Stop(run.Stop{Name: "db", WorkDir: "/wt/b"}) }},
		{"crashed", func(t *testing.T, w *world) {
			w.spawner.process("up db").end(1)
			require.Eventually(t, func() bool { return len(w.List("/wt/b")) == 0 }, time.Second, time.Millisecond)
		}},
	} {
		t.Run(gone.name, func(t *testing.T) {
			create := make(chan struct{})
			w := newWorld(t, map[string]script{"up db": prints("listening"), "createdb": holds(create, 0)})
			w.params.NamespaceBudget = time.Minute
			spec := shared("db", "/wt/b", "")
			spec.Job.Namespace = &project.Namespace{Name: "app_{worktree}", Create: "createdb"}
			result := make(chan error, 1)
			go func() {
				_, err := w.Start(spec, nil)
				result <- err
			}()
			require.Eventually(t, func() bool { return slices.Contains(w.spawner.lines(), "createdb") }, time.Second, time.Millisecond)

			gone.do(t, w)
			close(create)

			var failure *run.Error
			require.ErrorAs(t, <-result, &failure)
			assert.Equal(t, run.CodeLaunchFailed, failure.Code)
			assert.Empty(t, w.List("/wt/b"))
			for _, event := range w.bus.events() {
				if stopped, ok := event.(domain.JobStopped); ok {
					assert.NotEqual(t, run.HoldClaim, stopped.Job.Hold, "a claim never announced is never announced gone")
				}
			}
		})
	}
}

func TestASharedTaskThatEndsFailsItsStartOnce(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0)})
	spec := shared("db", "/wt/a", "")
	spec.Job.Kind = project.Task

	_, err := w.Start(spec, nil)

	var failure *run.Error
	require.ErrorAs(t, err, &failure)
	assert.Equal(t, run.CodeLaunchFailed, failure.Code)
	assert.Equal(t, []string{"up db"}, w.spawner.lines(), "never run again")
}

func TestAFailedNamespaceOfTheMainCheckoutLeavesItsNewerHold(t *testing.T) {
	first := make(chan struct{})
	var mu sync.Mutex
	calls := 0
	createdb := func(p *fakeProcess, _ func(string)) {
		mu.Lock()
		calls++
		call := calls
		mu.Unlock()
		switch call {
		case 1:
			<-first
		case 2:
			p.end(0)
			return
		}
		p.end(1)
	}
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(0), "createdb": createdb})
	w.params.NamespaceBudget = time.Second
	_, err := w.Start(shared("db", "/wt/a", "down db"), nil)
	require.NoError(t, err)
	main := shared("db", "/repo", "down db")
	main.Job.Namespace = &project.Namespace{Name: "app_{worktree}", Create: "createdb"}
	stale := make(chan error, 1)
	go func() {
		_, err := w.Start(main, nil)
		stale <- err
	}()
	require.Eventually(t, func() bool { return slices.Contains(w.spawner.lines(), "createdb") }, time.Second, time.Millisecond)
	released, err := w.Stop(run.Stop{Name: "db", WorkDir: "/repo"})
	require.NoError(t, err)
	require.Equal(t, run.OutcomeReleased, released)
	_, err = w.Start(main, nil)
	require.NoError(t, err)

	close(first)

	require.Error(t, <-stale)
	last, err := w.Stop(run.Stop{Name: "db", WorkDir: "/wt/a"})
	require.NoError(t, err)
	assert.Equal(t, run.OutcomeReleased, last, "the main checkout's newer hold stands")
	assert.NotContains(t, w.spawner.lines(), "down db")
}

func TestAShutdownDuringANamespaceCreateLeavesTheStackToTheIndex(t *testing.T) {
	w := newWorld(t, map[string]script{"up db": exits(0), "down db": exits(0)})
	w.params.NamespaceBudget = time.Minute
	spec := shared("db", "/wt/a", "down db")
	spec.Job.Namespace = &project.Namespace{Name: "app_{worktree}", Create: "createdb"}
	result := make(chan error, 1)
	go func() {
		_, err := w.Start(spec, nil)
		result <- err
	}()
	require.Eventually(t, func() bool { return slices.Contains(w.spawner.lines(), "createdb") }, time.Second, time.Millisecond)

	w.Shutdown()

	require.Error(t, <-result)
	assert.NotContains(t, w.spawner.lines(), "down db", "halted, never released")
	assert.Contains(t, w.index.names(), "/repo:db")
}
