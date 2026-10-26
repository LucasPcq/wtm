package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/clean"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheWizardDrawsItsKeyScreens(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase()})
	d.looksLike("worktrees")

	d.Keys("down", "space", "down", "space", "enter")
	d.looksLike("reparent")

	d.Keys("enter")
	d.looksLike("force")

	d.Keys("down", "enter")
	d.looksLike("recap")
}

func TestTheRequestIsWhatWasAnswered(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase()})

	d.Keys("down", "space", "down", "space", "enter", "down", "enter", "down", "enter", "enter")

	require.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Equal(t, clean.Request{Worktrees: []string{dirty, parent}, ReparentChildren: true, Force: true}, d.s.Request())
	assert.NotEmpty(t, d.s.Preview().Fingerprint)
}

func TestASafeRemovalAsksNeitherReparentNorForce(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: clean.Request{Worktrees: []string{plain}}})

	assert.Equal(t, stepRecap, d.Wizard.Current())
	screen := d.Screen()
	assert.Contains(t, screen, "✓ Worktrees: feat/plain")
	assert.Contains(t, screen, "· Reparent children — no child worktree is left behind")
	assert.Contains(t, screen, "· Force — every worktree is safe to remove")
}

func TestForceGivenStandsInTheTrailWithItsFlag(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: clean.Request{Worktrees: []string{dirty}, Force: true}})

	assert.Equal(t, stepRecap, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "✓ Force: forced · --force")
}

func TestNotForcingAnUnsafeWorktreeShowsTheRefusalAtTheRecap(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: clean.Request{Worktrees: []string{dirty}}})

	assert.Equal(t, stepForce, d.Wizard.Current())
	d.Keys("enter")

	assert.Equal(t, stepRecap, d.Wizard.Current())
	d.looksLike("refused")
}

func TestTheWorktreesStepRefusesAnEmptySelection(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase()})

	d.Keys("enter")

	assert.Equal(t, stepWorktrees, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "✗ Select at least one worktree")
}
