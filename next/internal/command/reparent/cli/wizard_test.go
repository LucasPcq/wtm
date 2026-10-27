package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/reparent"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheWizardDrawsItsKeyScreens(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase()})
	d.looksLike("worktrees")

	d.Keys("down", "space", "enter")
	d.looksLike("to")

	d.Keys("enter")
	d.looksLike("recap")
}

func TestTheRequestIsWhatWasAnswered(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase()})

	d.Keys("down", "space", "enter", "enter", "enter")

	require.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Equal(t, reparent.Request{Worktrees: []string{featB}, To: "main"}, d.s.Request())
	assert.NotEmpty(t, d.s.Preview().Fingerprint)
}

func TestTheWorktreeTheCallerStandsInIsPrePicked(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{Cwd: pathOf(featC)}).usecase(), Request: reparent.Request{Caller: callerIn(featC)}})

	d.Keys("enter")

	assert.Equal(t, stepTo, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "feat/c is rebased onto feat/b")
}

func TestAPlanThatMovesNothingEndsWithoutAskingToConfirm(t *testing.T) {
	d := open(t, sessionParams{App: newWorld(seed{}).usecase(), Request: reparent.Request{Worktrees: []string{featA, featX}, To: "main"}})

	assert.Equal(t, form.Finished, d.Wizard.Ending())
	assert.NotContains(t, d.Screen(), "Yes, apply")
	assert.True(t, d.s.Preview().Empty())
}

func TestThePickerAndTheNewParentStepSayTheParentLeftIsGone(t *testing.T) {
	w := newWorld(seed{})
	_, err := w.store.Update(t.Context(), worktree.MetaUpdate{Branch: featC, Change: func(meta *worktree.Meta) error {
		meta.SourceBranch = "feat/deleted"
		return nil
	}})
	require.NoError(t, err)

	picker := open(t, sessionParams{App: w.usecase()})
	d := open(t, sessionParams{App: w.usecase(), Request: reparent.Request{Worktrees: []string{featC}}})

	assert.Contains(t, picker.Screen(), "feat/c  on feat/deleted (gone)")
	assert.Equal(t, stepTo, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "feat/c is rebased onto feat/deleted (gone)")
}
