package kernel_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/kernel"
)

// — A run with nothing in its way —

func TestEachRunsOneUnitPerItemInOrder(t *testing.T) {
	d := &disk{}
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a", "b"},
		Unit:  func(branch string) kernel.Unit[string] { return worktreeUnit(d, branch) },
	})
	assert.Equal(t, []string{"a:done/", "b:done/"}, outcomes(items))
	assert.Equal(t, []string{"a", "a/meta", "b", "b/meta"}, d.list())
}

func TestEachWithNoItemsDoesNothing(t *testing.T) {
	items := each(context.Background(), kernel.EachParams[string, string]{
		Unit: func(string) kernel.Unit[string] { t.Fatal("a unit ran"); return kernel.Unit[string]{} },
	})
	assert.Empty(t, items)
}

func TestTheCommitIsTheItemsDetailAndEachPhaseCompletesIt(t *testing.T) {
	ports := phase("ports", func(_ context.Context, path string) (string, error) { return path + " ports=3000", nil })
	hooks := phase("hooks", func(_ context.Context, path string) (string, error) { return path + " hooks=ok", nil })
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit:  func(branch string) kernel.Unit[string] { return worktreeUnit(&disk{}, branch, ports, hooks) },
	})
	assert.Equal(t, "/wt/a ports=3000 hooks=ok", items[0].Detail)
}

func TestEachUnitSaysWhatItIsDoingAsItGoes(t *testing.T) {
	progress := &recorder{}
	each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Emit:  progress,
		Unit: func(branch string) kernel.Unit[string] {
			unit := worktreeUnit(&disk{}, branch, phase("hooks", func(_ context.Context, path string) (string, error) { return path, nil }))
			unit.Before = []kernel.Prep{{Name: "stop jobs", Run: func(context.Context) error { return nil }}}
			return unit
		},
	})
	assert.Equal(t, []string{
		"a unit.started",
		"a phase.started stop jobs", "a phase.finished stop jobs",
		"a phase.started worktree", "a phase.finished worktree",
		"a phase.started meta", "a phase.finished meta",
		"a phase.started hooks", "a phase.finished hooks",
		"a unit.finished done",
	}, progress.lines())
}

// — The saga: all or nothing —

func TestAFailedSagaIsUndoneInReverseAndLeavesNothing(t *testing.T) {
	d := &disk{}
	var undone []string
	track := func(name string, undo func(context.Context) error) func(context.Context) error {
		return func(ctx context.Context) error { undone = append(undone, name); return undo(ctx) }
	}
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit: func(string) kernel.Unit[string] {
			return kernel.Unit[string]{Saga: kernel.Saga[string]{Steps: []kernel.SagaStep{
				{Name: "worktree", Do: d.add("a"), Undo: track("worktree", d.remove("a"))},
				{Name: "meta", Do: d.add("a/meta"), Undo: track("meta", d.remove("a/meta"))},
				{Name: "publish", Do: fail},
			}}}
		},
	})
	assert.Equal(t, []string{"a:failed/"}, outcomes(items))
	assert.Equal(t, kernel.KindInternal, items[0].Error.Kind())
	assert.Equal(t, kernel.Problem{Code: kernel.CodeStepFailed, Params: kernel.Params{kernel.ParamStep: "publish"}, Cause: errBoom}, *items[0].Error.Base())
	assert.Equal(t, []string{"meta", "worktree"}, undone)
	assert.Empty(t, d.list())
}

func TestAStepWithNoUndoLeavesItToAnEarlierStep(t *testing.T) {
	d := &disk{}
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit: func(string) kernel.Unit[string] {
			return kernel.Unit[string]{Saga: kernel.Saga[string]{Steps: []kernel.SagaStep{
				{Name: "worktree", Do: d.add("a"), Undo: d.remove("a")},
				{Name: "env", Do: d.add("a/.env")},
				{Name: "meta", Do: fail},
			}}}
		},
	})
	assert.Equal(t, []string{"a:failed/"}, outcomes(items))
	assert.Equal(t, []string{"a/.env"}, d.list(), "here nothing removes a/.env: in create, removing the worktree does")
}

func TestACommitThatFailsUndoesEveryStep(t *testing.T) {
	d := &disk{}
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit: func(branch string) kernel.Unit[string] {
			unit := worktreeUnit(d, branch)
			unit.Saga.Commit = func(context.Context) (string, error) { return "", errBoom }
			return unit
		},
	})
	assert.Equal(t, []string{"a:failed/"}, outcomes(items))
	assert.Equal(t, "commit", items[0].Error.Base().Params[kernel.ParamStep])
	assert.Empty(t, d.list())
}

func TestAnUndoThatFailsNamesWhatIsLeftAndProposesTheFollowUp(t *testing.T) {
	d := &disk{}
	cleanIt := func(step string) *kernel.FollowUp {
		return &kernel.FollowUp{Command: "clean", Code: "test.left_behind", Params: kernel.Params{kernel.ParamStep: step}}
	}
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit: func(string) kernel.Unit[string] {
			return kernel.Unit[string]{Saga: kernel.Saga[string]{
				Steps: []kernel.SagaStep{
					{Name: "worktree", Do: d.add("a"), Undo: d.remove("a")},
					{Name: "meta", Do: d.add("a/meta"), Undo: fail},
					{Name: "publish", Do: fail},
				},
				LeftBehind: cleanIt,
			}}
		},
	})
	require.Equal(t, []string{"a:failed/"}, outcomes(items))
	problem := items[0].Error.Base()
	assert.Equal(t, kernel.CodeUndoFailed, problem.Code)
	assert.Equal(t, kernel.Params{kernel.ParamStep: "meta", kernel.ParamLeft: "worktree,meta"}, problem.Params)
	assert.Equal(t, cleanIt("meta"), problem.FollowUp)
	assert.ErrorIs(t, items[0].Error, errBoom)
	assert.Equal(t, []string{"a", "a/meta"}, d.list(), "the walk back stops at the undo that failed")
}

func TestAnUndoThatFailsWithNothingToProposeHasNoFollowUp(t *testing.T) {
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit: func(string) kernel.Unit[string] {
			return kernel.Unit[string]{Saga: kernel.Saga[string]{Steps: []kernel.SagaStep{
				{Name: "worktree", Do: func(context.Context) error { return nil }, Undo: fail},
				{Name: "meta", Do: fail},
			}}}
		},
	})
	assert.Equal(t, kernel.CodeUndoFailed, items[0].Error.Base().Code)
	assert.Nil(t, items[0].Error.Base().FollowUp)
}

// — Interruptions: the regression tests of LUC-257, on Each —

func TestAnInterruptedBatchFinishesTheUnitUnderWayAndStopsThere(t *testing.T) {
	d := &disk{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	interrupt := func(context.Context) error { cancel(); return nil }
	items := each(ctx, kernel.EachParams[string, string]{
		Items: []string{"a", "b", "c"},
		Unit: func(branch string) kernel.Unit[string] {
			unit := worktreeUnit(d, branch)
			unit.Saga.Steps = append([]kernel.SagaStep{{Name: "ctrl-c", Do: interrupt}}, unit.Saga.Steps...)
			return unit
		},
	})
	assert.Equal(t, []string{"a:done/", "b:skipped/interrupted", "c:skipped/interrupted"}, outcomes(items))
	assert.Equal(t, []string{"a", "a/meta"}, d.list(), "the unit under way finishes whole, the next never starts")
}

func TestTheSagaRunsUnderTheShieldItIsHanded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var taken, released int
	shield := func(ctx context.Context) (context.Context, func()) {
		taken++
		return context.WithoutCancel(ctx), func() { released++ }
	}
	var stepSawTheCancel bool
	items := each(ctx, kernel.EachParams[string, string]{
		Items:  []string{"a"},
		Shield: shield,
		Unit: func(string) kernel.Unit[string] {
			return kernel.Unit[string]{Saga: kernel.Saga[string]{Steps: []kernel.SagaStep{
				{Name: "worktree", Do: func(context.Context) error { cancel(); return nil }},
				{Name: "meta", Do: func(ctx context.Context) error { stepSawTheCancel = ctx.Err() != nil; return nil }},
			}}}
		},
	})
	assert.Equal(t, []string{"a:done/"}, outcomes(items))
	assert.False(t, stepSawTheCancel, "the next step runs as if nothing was cancelled")
	assert.Equal(t, 1, taken)
	assert.Equal(t, 1, released)
}

func TestAnInterruptInThePrepLeavesTheUnitUntouched(t *testing.T) {
	d := &disk{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	stopJobs := kernel.Prep{Name: "stop jobs", Run: func(context.Context) error { cancel(); return context.Canceled }}
	items := each(ctx, kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit: func(branch string) kernel.Unit[string] {
			unit := worktreeUnit(d, branch)
			unit.Before = []kernel.Prep{stopJobs}
			return unit
		},
	})
	assert.Equal(t, []string{"a:cancelled/interrupted"}, outcomes(items))
	assert.Equal(t, kernel.KindCancelled, items[0].Error.Kind())
	assert.Equal(t, "stop jobs", items[0].Error.Base().Params[kernel.ParamPhase], "the error names the prep it stopped in")
	assert.Empty(t, d.list())
}

func TestAnInterruptBetweenThePrepAndTheSagaStartsNoSaga(t *testing.T) {
	d := &disk{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	items := each(ctx, kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit: func(branch string) kernel.Unit[string] {
			unit := worktreeUnit(d, branch)
			unit.Before = []kernel.Prep{{Name: "check", Run: func(context.Context) error { cancel(); return nil }}}
			return unit
		},
	})
	assert.Equal(t, []string{"a:cancelled/interrupted"}, outcomes(items))
	assert.Empty(t, d.list())
}

func TestAFailureTheInterruptCausedCountsAsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hookKilled := func(context.Context) error { cancel(); return errBoom }
	items := each(ctx, kernel.EachParams[string, string]{
		Items:         []string{"a", "b", "c"},
		StopOnFailure: true,
		Unit: func(branch string) kernel.Unit[string] {
			unit := worktreeUnit(&disk{}, branch)
			unit.Before = []kernel.Prep{{Name: "on_clean", Run: hookKilled}}
			return unit
		},
	})
	assert.Equal(t, []string{"a:cancelled/interrupted", "b:skipped/interrupted", "c:skipped/interrupted"}, outcomes(items))
}

func TestAnInterruptDuringThenKeepsTheCommitAndNamesWhatWasNotDone(t *testing.T) {
	d := &disk{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var hooksRan bool
	ports := phase("ports", func(_ context.Context, path string) (string, error) { cancel(); return path + " ports=3000", nil })
	hooks := phase("hooks", func(_ context.Context, path string) (string, error) { hooksRan = true; return path, nil })
	items := each(ctx, kernel.EachParams[string, string]{
		Items: []string{"a", "b"},
		Unit:  func(branch string) kernel.Unit[string] { return worktreeUnit(d, branch, ports, hooks) },
	})
	assert.Equal(t, []string{"a:cancelled/interrupted", "b:skipped/interrupted"}, outcomes(items))
	assert.False(t, hooksRan, "the phases after the interrupt never run")
	assert.Equal(t, []string{"a", "a/meta"}, d.list(), "the saga stays")
	assert.Equal(t, "/wt/a ports=3000", items[0].Detail, "the item keeps what was done")
	assert.Equal(t, "hooks", items[0].Error.Base().Params[kernel.ParamPhase], "and names the phase not run")
}

// — Failures —

func TestAPrepThatFailsStopsItsUnitBeforeTheSaga(t *testing.T) {
	d := &disk{}
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a", "b"},
		Unit: func(branch string) kernel.Unit[string] {
			unit := worktreeUnit(d, branch)
			if branch == "a" {
				unit.Before = []kernel.Prep{{Name: "on_clean", Run: fail}}
			}
			return unit
		},
	})
	assert.Equal(t, []string{"a:failed/", "b:done/"}, outcomes(items))
	assert.Equal(t, kernel.Problem{Code: kernel.CodePhaseFailed, Params: kernel.Params{kernel.ParamPhase: "on_clean"}, Cause: errBoom}, *items[0].Error.Base())
	assert.Equal(t, []string{"b", "b/meta"}, d.list())
}

func TestARefusalInThePrepKeepsItsOwnType(t *testing.T) {
	refused := &kernel.RefusedError{Problem: kernel.Problem{Code: "test.dirty"}, Blockers: []kernel.Blocker{{Code: "test.dirty", Field: "force"}}}
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit: func(string) kernel.Unit[string] {
			return kernel.Unit[string]{Before: []kernel.Prep{{Name: "check", Run: func(context.Context) error { return refused }}}}
		},
	})
	assert.Equal(t, kernel.Error(refused), items[0].Error)
}

func TestAPhaseThatFailsKeepsTheCommitAndStopsTheNextPhases(t *testing.T) {
	d := &disk{}
	var hooksRan bool
	ports := phase("ports", func(context.Context, string) (string, error) { return "", errBoom })
	hooks := phase("hooks", func(_ context.Context, path string) (string, error) { hooksRan = true; return path, nil })
	items := each(context.Background(), kernel.EachParams[string, string]{
		Items: []string{"a"},
		Unit:  func(branch string) kernel.Unit[string] { return worktreeUnit(d, branch, ports, hooks) },
	})
	assert.Equal(t, []string{"a:failed/"}, outcomes(items))
	assert.Equal(t, "/wt/a", items[0].Detail)
	assert.False(t, hooksRan)
	assert.Equal(t, []string{"a", "a/meta"}, d.list())
	assert.Equal(t, "ports", items[0].Error.Base().Params[kernel.ParamPhase])
}

func TestAFailureStopsTheBatchOnlyWhenAskedTo(t *testing.T) {
	failingA := func(branch string) kernel.Unit[string] {
		unit := worktreeUnit(&disk{}, branch)
		if branch == "a" {
			unit.Saga.Steps[1].Do = fail
		}
		return unit
	}
	stopped := each(context.Background(), kernel.EachParams[string, string]{Items: []string{"a", "b"}, StopOnFailure: true, Unit: failingA})
	assert.Equal(t, []string{"a:failed/", "b:skipped/not_reached"}, outcomes(stopped))
	carried := each(context.Background(), kernel.EachParams[string, string]{Items: []string{"a", "b"}, Unit: failingA})
	assert.Equal(t, []string{"a:failed/", "b:done/"}, outcomes(carried))
}
