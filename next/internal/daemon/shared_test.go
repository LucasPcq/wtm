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
