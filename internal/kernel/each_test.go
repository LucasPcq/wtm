package kernel_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/kernel"
)

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
			unit.Before = []kernel.Prep{{Name: "stop jobs", Run: func(context.Context, kernel.Reporter) error { return nil }}}
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
