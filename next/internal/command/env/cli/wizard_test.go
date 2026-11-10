package cli

import (
	"testing"

	envcmd "github.com/LucasPcq/wtm/next/internal/command/env"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheWizardDrawsItsKeyScreens(t *testing.T) {
	d := open(t, sessionParams{App: newWorld().usecase(), Request: envcmd.Request{Caller: callerIn(billing)}})
	assert.Equal(t, stepWorktrees, d.Wizard.Current())
	d.looksLike("worktrees")

	d.Keys("enter")
	assert.Equal(t, stepPrune, d.Wizard.Current(), "no key differs from its source: overwrite is not asked")
	d.looksLike("prune")

	d.Keys("enter")
	assert.Equal(t, stepRecap, d.Wizard.Current())
	d.looksLike("recap")
}

func TestTheDefaultPickConfirmedReconcilesTheWorktreesOutOfStep(t *testing.T) {
	w := newWorld()
	d := open(t, sessionParams{App: w.usecase(), Request: envcmd.Request{Caller: callerIn(billing)}})

	d.Keys("enter", "enter", "enter")

	require.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Equal(t, envcmd.Request{Caller: callerIn(billing)}, d.s.Request(), "the default pick names none")
}
