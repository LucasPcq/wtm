package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/exec"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheWizardDrawsItsKeyScreens(t *testing.T) {
	d := open(t, sessionParams{App: newWorld().usecase(), Request: exec.Request{Caller: newWorld().env().Caller()}})
	assert.Equal(t, stepWorktrees, d.Wizard.Current())
	d.looksLike("worktrees")

	d.Keys("down", "down", "space", "enter")
	d.looksLike("command")

	d.Keys("pnpm test")
	d.Keys("enter")
	assert.Equal(t, stepRecap, d.Wizard.Current())
	d.looksLike("recap")
}

func TestTheRequestIsWhatWasAnswered(t *testing.T) {
	d := open(t, sessionParams{App: newWorld().usecase(), Request: exec.Request{Caller: newWorld().env().Caller()}})

	d.Keys("down", "down", "space", "enter")
	d.Keys("pnpm test")
	d.Keys("enter", "enter")

	require.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Equal(t, []string{featA, featB}, d.s.Request().Worktrees)
	assert.Equal(t, "pnpm test", d.s.Request().Command)
}

func TestACommandTheShellRefusesIsSaidAtItsStep(t *testing.T) {
	d := open(t, sessionParams{App: newWorld().usecase(), Request: exec.Request{Worktrees: []string{featA}}})

	assert.Equal(t, stepCommand, d.Wizard.Current())
	d.Keys("if then fi")
	d.Keys("enter")

	assert.Equal(t, stepCommand, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "syntax error near unexpected token")
}
