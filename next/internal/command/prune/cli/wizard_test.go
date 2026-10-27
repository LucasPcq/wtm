package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/prune"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheWizardDrawsItsKeyScreens(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase()})
	d.looksLike("worktrees")

	d.Keys("space", "enter")
	d.looksLike("reparent")

	d.Keys("enter")
	d.looksLike("force")

	d.Keys("down", "enter")
	d.looksLike("recap")
}

// B3: legacy let an unsafe worktree be checked, counted it in the confirmation, then skipped it after the fact.
func TestCheckingAnUnsafeWorktreeAsksForceAndTheRequestIsWhatWasAnswered(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase()})

	d.Keys("space", "enter", "down", "enter", "down", "enter", "enter")

	require.Equal(t, form.Finished, d.Wizard.Ending())
	assert.ElementsMatch(t, []string{merged, dirty, gone}, d.s.Request().Worktrees)
	assert.True(t, d.s.Request().ReparentChildren)
	assert.True(t, d.s.Request().Force)
	assert.NotEmpty(t, d.s.Preview().Fingerprint)
}

func TestNothingToPruneEndsWithoutAskingToConfirm(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{NoPRs: true}).usecase(), Request: prune.Request{Merged: true}})

	assert.Equal(t, form.Finished, d.Wizard.Ending())
	assert.NotContains(t, d.Screen(), "Yes, apply")
	assert.True(t, d.s.Preview().Empty())
}

// The default pick confirmed untouched read as worktrees named: feat/dirty, left out of it, vanished from the plan with its warning and its --force line.
func TestTheDefaultPickConfirmedLeavesTheChoiceToTheCommand(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase()})

	d.Keys("enter", "enter")
	assert.Contains(t, d.Screen(), "feat/dirty is kept: unsafe to remove (dirty)", "the recap names what is left as it is")
	d.Keys("enter")

	require.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Nil(t, d.s.Request().Worktrees, "no worktree named: the command chose")
	require.NotNil(t, d.s.Preview())
	kept := d.s.Preview().Plan.Worktrees[0]
	assert.Equal(t, dirty, kept.Worktree)
	assert.Empty(t, kept.Phases)
	assert.Contains(t, codes(d.s.Preview().Warnings), prune.CodeKept)
}

// The pick changed, then Esc back to the picker: the candidates not picked still show what makes them unsafe.
func TestBackInThePickerEachCandidateStillShowsItsBlockers(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase()})

	d.Keys("down", "space", "enter", "esc")

	assert.Equal(t, stepWorktrees, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "feat/dirty — merged #14 · uncommitted changes")
}
