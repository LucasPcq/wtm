package kernel_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestAnInterruptedBatchFinishesTheUnitUnderWayAndStopsThere(t *testing.T) {
	d := &disk{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	interrupt := func(context.Context, kernel.Reporter) error { cancel(); return nil }
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
				{Name: "worktree", Do: func(context.Context, kernel.Reporter) error { cancel(); return nil }},
				{Name: "meta", Do: func(ctx context.Context, _ kernel.Reporter) error { stepSawTheCancel = ctx.Err() != nil; return nil }},
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
	stopJobs := kernel.Prep{Name: "stop jobs", Run: func(context.Context, kernel.Reporter) error { cancel(); return context.Canceled }}
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
			unit.Before = []kernel.Prep{{Name: "check", Run: func(context.Context, kernel.Reporter) error { cancel(); return nil }}}
			return unit
		},
	})
	assert.Equal(t, []string{"a:cancelled/interrupted"}, outcomes(items))
	assert.Empty(t, d.list())
}

func TestAFailureTheInterruptCausedCountsAsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	hookKilled := func(context.Context, kernel.Reporter) error { cancel(); return errBoom }
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
	assert.Equal(t, []string{"a:cancelled/incomplete", "b:skipped/interrupted"}, outcomes(items), "a's saga committed, a phase did not finish")
	assert.False(t, hooksRan, "the phases after the interrupt never run")
	assert.Equal(t, []string{"a", "a/meta"}, d.list(), "the saga stays")
	assert.Equal(t, "/wt/a ports=3000", items[0].Detail, "the item keeps what was done")
	assert.Equal(t, "hooks", items[0].Error.Base().Params[kernel.ParamPhase], "and names the phase not run")
}
