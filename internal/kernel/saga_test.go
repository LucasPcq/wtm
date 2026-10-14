package kernel_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/kernel"
)

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
					{Name: "meta", Do: d.add("a/meta"), Undo: failUndo},
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
				{Name: "worktree", Do: func(context.Context, kernel.Reporter) error { return nil }, Undo: failUndo},
				{Name: "meta", Do: fail},
			}}}
		},
	})
	assert.Equal(t, kernel.CodeUndoFailed, items[0].Error.Base().Code)
	assert.Nil(t, items[0].Error.Base().FollowUp)
}
