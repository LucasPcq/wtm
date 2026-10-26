package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/fastforward"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheWizardDrawsItsKeyScreens(t *testing.T) {
	d := open(t, sessionParams{App: newWorld().usecase()})
	d.looksLike("worktrees")

	d.Keys("enter")
	d.looksLike("recap")
}

func TestTheDefaultPickConfirmedAsItWasIsEveryWorktree(t *testing.T) {
	d := open(t, sessionParams{App: newWorld().usecase()})

	d.Keys("enter", "enter")

	require.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Equal(t, fastforward.Request{All: true}, d.s.Request())
}

func TestAPickTheUserChangedNamesItsWorktrees(t *testing.T) {
	d := open(t, sessionParams{App: newWorld().usecase()})

	d.Keys("space", "enter")

	assert.Equal(t, fastforward.Request{Worktrees: []string{behind}}, d.s.Request())
	assert.Contains(t, d.Screen(), "· All — only --all asks every worktree")
}

func TestANamedDirtyWorktreeAsksToForceItAsTheUnattendedRunRefuses(t *testing.T) {
	d := open(t, sessionParams{App: newWorld().usecase(), Request: fastforward.Request{Worktrees: []string{behind, dirty}}})

	assert.Equal(t, stepForce, d.Wizard.Current())
	d.looksLike("force")

	d.Keys("enter")

	assert.Equal(t, stepRecap, d.Wizard.Current())
	d.looksLike("refused")
}

func TestASafeRunDoesNotAskForce(t *testing.T) {
	d := open(t, sessionParams{App: newWorld().usecase(), Request: fastforward.Request{Worktrees: []string{behind}}})

	assert.Equal(t, stepRecap, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "· Force — no worktree named behind origin has uncommitted changes")
}
