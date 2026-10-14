package kernel_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/kernel"
)

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
			return kernel.Unit[string]{Before: []kernel.Prep{{Name: "check", Run: func(context.Context, kernel.Reporter) error { return refused }}}}
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
	assert.Equal(t, []string{"a:failed/incomplete"}, outcomes(items), "a's saga committed, a phase did not finish")
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
