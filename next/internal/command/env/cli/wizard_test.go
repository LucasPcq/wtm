package cli

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	envcmd "github.com/LucasPcq/wtm/next/internal/command/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
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

func TestTheWizardNeverShowsAValueWtmDoesNotWrite(t *testing.T) {
	// legacy: the resolve step and the recap printed the secrets the report withheld
	w := newWorld()
	w.store = storetest.NewFake(storetest.FakeParams{Meta: map[string]worktree.Meta{billing: {SourceBranch: "main", EnvStrategy: project.EnvMain, Ordinal: 2, Isolation: project.Isolated}}, Files: map[string]string{
		mainPath + "/" + apiEnv:        "PORT=4000\nJWT_SECRET=s3cret\n",
		mainPath + "/" + webEnv:        "PORT=3000\n",
		pathOf(billing) + "/" + apiEnv: "PORT=4020\nJWT_SECRET=mine\n",
		pathOf(billing) + "/" + webEnv: "PORT=3020\n",
	}})
	d := open(t, sessionParams{App: w.usecase(), Request: envcmd.Request{Worktrees: []string{billing}}})

	screens := []string{d.Screen()}
	for range 3 {
		d.Keys("down", "enter")
		screens = append(screens, d.Screen())
	}

	for _, screen := range screens {
		assert.NotContains(t, screen, "s3cret")
		assert.NotContains(t, screen, "mine")
	}
	assert.Contains(t, strings.Join(screens, "\n"), "JWT_SECRET")
}
