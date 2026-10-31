package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/sync"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheWizardDrawsItsKeyScreens(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase()})
	d.looksLike("worktrees")

	d.Keys("enter")
	assert.Equal(t, stepKeepConflict, d.Wizard.Current())
	d.looksLike("keep_conflict")

	d.Keys("enter")
	assert.Equal(t, stepFFParents, d.Wizard.Current(), "main is on origin: the fetch may bring it commits")
	d.Keys("enter")
	assert.Equal(t, stepPush, d.Wizard.Current())
	d.looksLike("push")

	d.Keys("enter")
	assert.Equal(t, stepRecap, d.Wizard.Current())
	d.looksLike("recap")
}

func TestAWorktreeNamedWithWorktreesUnderItAsksWhetherTheyComeToo(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: sync.Request{Worktrees: []string{featA}}})

	assert.Equal(t, stepDescendants, d.Wizard.Current())
	d.looksLike("descendants")
	assert.Contains(t, d.Screen(), "Sync the 2 worktrees under feat/a too?")

	d.Keys("down", "enter")

	assert.Equal(t, sync.Request{Worktrees: []string{featA}, Descendants: new(false)}, d.s.Request())
}

func TestTheDefaultPickConfirmedAsItWasNamesNone(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase()})

	d.Keys("enter")

	assert.Equal(t, sync.Request{}, d.s.Request())
}

func TestNothingToSyncEndsWithoutAskingToConfirm(t *testing.T) {
	w := newWorld(seed{})
	d := open(t, sessionParams{App: w.usecase(), Request: sync.Request{Worktrees: []string{featC}, Descendants: new(false)}})

	// on origin as last fetched, feat/c and its parent may still move once fetched: each switch is asked first
	d.Keys("enter", "enter", "enter")

	require.Equal(t, form.Finished, d.Wizard.Ending())
	assert.NotContains(t, d.Screen(), "Yes, sync")
	assert.True(t, d.s.Preview().Empty())
}
