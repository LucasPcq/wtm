package kernel_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestMilestonesFollowTheUnitAsItGoes(t *testing.T) {
	var said []string
	milestones := kernel.Milestones[string]{
		Committed: func(m kernel.Committed[string]) { said = append(said, "committed "+m.Subject+" "+m.Detail) },
		PhaseDone: func(m kernel.PhaseDone[string]) { said = append(said, "done "+m.Phase+" "+m.Detail) },
		Finished:  func(item kernel.Item[string]) { said = append(said, "finished "+item.Subject+" "+string(item.Status)) },
	}
	hooks := phase("hooks", func(_ context.Context, path string) (string, error) {
		said = append(said, "hooks run")
		return path + " hooked", nil
	})
	each(context.Background(), kernel.EachParams[string, string]{
		Items:      []string{"a"},
		Milestones: milestones,
		Unit:       func(branch string) kernel.Unit[string] { return worktreeUnit(&disk{}, branch, hooks) },
	})
	assert.Equal(t, []string{"committed a /wt/a", "hooks run", "done hooks /wt/a hooked", "finished a done"}, said)
}

func TestAFailedSagaIsNeverCommittedButItsUnitFinishes(t *testing.T) {
	var said []string
	milestones := kernel.Milestones[string]{
		Committed: func(kernel.Committed[string]) { said = append(said, "committed") },
		Finished:  func(item kernel.Item[string]) { said = append(said, "finished "+string(item.Status)) },
	}
	each(context.Background(), kernel.EachParams[string, string]{
		Items:      []string{"a"},
		Milestones: milestones,
		Unit: func(string) kernel.Unit[string] {
			return kernel.Unit[string]{Saga: kernel.Saga[string]{Steps: []kernel.SagaStep{{Name: "worktree", Do: fail}}}}
		},
	})
	assert.Equal(t, []string{"finished failed"}, said)
}

func TestAUnitNeverReachedHasNoMilestone(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	each(ctx, kernel.EachParams[string, string]{
		Items:      []string{"a"},
		Milestones: kernel.Milestones[string]{Finished: func(kernel.Item[string]) { t.Fatal("a unit never reached finished") }},
		Unit:       func(branch string) kernel.Unit[string] { return worktreeUnit(&disk{}, branch) },
	})
}

func TestTheUnitFinishesBeforeItSaysSo(t *testing.T) {
	var said []string
	progress := kernel.EmitFunc(func(p kernel.Progress) {
		if _, finished := p.(*kernel.UnitFinished); finished {
			said = append(said, "unit.finished")
		}
	})
	each(context.Background(), kernel.EachParams[string, string]{
		Items:      []string{"a"},
		Emit:       progress,
		Milestones: kernel.Milestones[string]{Finished: func(kernel.Item[string]) { said = append(said, "finished") }},
		Unit:       func(branch string) kernel.Unit[string] { return worktreeUnit(&disk{}, branch) },
	})
	assert.Equal(t, []string{"finished", "unit.finished"}, said)
}
