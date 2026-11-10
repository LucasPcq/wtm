package cli

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	envcmd "github.com/LucasPcq/wtm/next/internal/command/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
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

func TestThePickOfferedFromMainNamesTheWorktreesOutOfStepItHolds(t *testing.T) {
	w := newWorld()
	d := open(t, sessionParams{App: w.usecase(), Request: envcmd.Request{Caller: callerIn("main")}})
	d.looksLike("worktrees_from_main")

	d.Keys("enter")

	assert.Equal(t, stepPrune, d.Wizard.Current(), "the questions after the worktrees are those of the pick: feat/billing holds a key in no source")
	assert.Equal(t, []string{"main", billing}, d.s.Request().Worktrees, "unnamed, the run would write main alone")
}

func TestTheWorktreesStepShowsThePickAgainWhenTheUserComesBack(t *testing.T) {
	d := open(t, sessionParams{App: newWorld().usecase(), Request: envcmd.Request{Caller: callerIn("main")}})

	d.Keys("space", "enter")
	d.Keys("esc")

	assert.Equal(t, stepWorktrees, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "[ ] main")
	assert.Contains(t, d.Screen(), "[x] feat/billing")
	assert.Equal(t, []string{billing}, d.s.Request().Worktrees)
}

func TestTheWizardNeverShowsAValueWtmDoesNotWrite(t *testing.T) {
	// legacy: the resolve step and the recap printed the secrets the report withheld
	w := newWorld()
	w.store = storetest.NewFake(storetest.FakeParams{Meta: map[string]worktree.Meta{billing: {SourceBranch: "main", EnvStrategy: project.EnvMain, Ordinal: 2, Isolation: project.Isolated}}, Files: map[string]string{
		mainPath + "/" + apiEnv:        "PORT=4000\nJWT_SECRET=s3cret\n",
		mainPath + "/" + webEnv:        "PORT=3000\n",
		pathOf(billing) + "/" + apiEnv: "PORT=4020\nJWT_SECRET=hunter2\n",
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
		assert.NotContains(t, screen, "hunter2")
	}
	assert.Contains(t, strings.Join(screens, "\n"), "JWT_SECRET")
}
