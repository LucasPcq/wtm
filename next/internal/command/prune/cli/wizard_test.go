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

func TestNothingToPruneSkipsTheWorktreesStep(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{NoPRs: true}).usecase(), Request: prune.Request{Merged: true}})

	assert.Equal(t, stepRecap, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "Worktrees — no worktree is done with")
}

// The default pick confirmed untouched read as worktrees named: feat/dirty, left out of it, vanished from the plan with its warning and its --force line.
func TestTheDefaultPickConfirmedLeavesTheChoiceToTheCommand(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase()})

	d.Keys("enter", "enter", "enter")

	require.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Nil(t, d.s.Request().Worktrees, "no worktree named: the command chose")
	require.NotNil(t, d.s.Preview())
	kept := d.s.Preview().Plan.Worktrees[0]
	assert.Equal(t, dirty, kept.Worktree)
	assert.Empty(t, kept.Phases)
	assert.Contains(t, d.Screen()+d.s.Preview().Warnings[0].Code, prune.CodeKept)
}
