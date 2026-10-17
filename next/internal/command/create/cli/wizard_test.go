package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheFormDrawsEachStepAsTheEngineDid(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{})})
	d.looksLike("1_branches")

	d.keys("feat/a", "tab", "feature", "tab")
	d.looksLike("2_branches_entries")

	d.keys("enter")
	d.looksLike("3_source")

	d.keys("enter")
	d.looksLike("4_env")

	d.keys("down", "tab", "enter")
	d.looksLike("5_recap")

	d.keys("esc")
	d.looksLike("6_back_to_env")
}

func TestTheRequestIsWhatWasAnswered(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{})})

	d.keys("feat/a", "enter", "enter", "down", "tab", "enter", "enter")

	require.Equal(t, form.Finished, d.w.Ending())
	assert.Equal(t, create.Request{Branches: []string{"feat/a"}, From: "main", EnvFrom: project.EnvMain, Remember: []string{"env_from"}}, d.c.req)
	assert.NotEmpty(t, d.c.preview.Fingerprint)
}

func TestAKeptDefaultSaysWhereItCameFrom(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{})})

	d.keys("feat/a", "enter", "enter", "enter")

	assert.Contains(t, d.screen(), "✓ Source branch: main · config\n  ✓ Env strategy: example · config")
}

func TestAnEntryIsCheckedAsItIsAdded(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{Linked: []string{"feat/x"}})})

	d.keys("bad..name", "tab")
	assert.Contains(t, d.screen(), `✗ "bad..name" is not a valid branch name: it cannot hold .. or end with .`)

	d.keys("backspace", "backspace", "backspace", "backspace", "backspace", "backspace", "backspace", "backspace", "backspace", "feat/x", "tab")
	assert.Contains(t, d.screen(), "✗ Branch feat/x is checked out at /elsewhere/feat-x — run `wtm go feat/x` to jump in")

	d.keys("enter")
	assert.Equal(t, int(stepBranches), d.w.Current())
}

func TestNoEntryIsRequired(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{})})

	d.keys("enter")

	assert.Contains(t, d.screen(), "✗ Branches is required")
}

func TestWhatAFlagOrARememberedAnswerSettledIsNotAsked(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{Remembered: map[string]string{project.RememberEnvStrategy: "main"}}), Request: create.Request{Branches: []string{"feat/a"}, From: "main"}})

	assert.Equal(t, int(stepRecap), d.w.Current())
	screen := d.screen()
	assert.Contains(t, screen, "✓ Branches: feat/a · <branches>")
	assert.Contains(t, screen, "✓ Source branch: main · --from")
	assert.Contains(t, screen, "✓ Env strategy: main · remembered")
	assert.Contains(t, screen, "esc cancel")
}

func TestAskAsksARememberedAnswerAgainStartingOnIt(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{Remembered: map[string]string{project.RememberEnvStrategy: "main"}}), Request: create.Request{Branches: []string{"feat/a"}, From: "main", Ask: true}})

	assert.Equal(t, int(stepEnvFrom), d.w.Current())
	assert.Equal(t, "main", d.c.envFrom.list.Value())
	assert.Contains(t, d.screen(), "[x] Always use this answer in this repo")
}

func TestASourceBehindOriginIsOfferedAFastForward(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{Behind: true}), Request: create.Request{Branches: []string{"feat/a"}, From: "main", EnvFrom: project.EnvExample}})

	assert.Equal(t, int(stepFastForward), d.w.Current())
	d.keys("up", "enter")

	assert.Equal(t, new(true), d.c.req.FastForward)
	assert.Contains(t, d.screen(), "Fast-forward  main")
}

func TestTheSourceListPinsTheBaseBranchAndRefreshes(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{Behind: true}), Request: create.Request{Branches: []string{"feat/a"}}})

	assert.Equal(t, int(stepFrom), d.w.Current())
	assert.Contains(t, d.screen(), "▌› main")
	assert.Contains(t, d.screen(), "1 behind")

	d.keys("down", "r")

	assert.Equal(t, "feature", d.c.from.list.Value())
	assert.NotContains(t, d.screen(), "Refreshing")
}

func TestEachWayOutCancels(t *testing.T) {
	settled := create.Request{Branches: []string{"feat/a"}, From: "main", EnvFrom: project.EnvExample}
	cases := map[string]struct {
		Request create.Request
		Keys    []string
	}{
		"esc on the first question": {Keys: []string{"esc"}},
		"no, cancel":                {Request: settled, Keys: []string{"down", "enter"}},
		"ctrl+c":                    {Keys: []string{"ctrl+c"}},
	}
	for name, c := range cases {
		d := open(t, wizardParams{App: newApp(seed{}), Request: c.Request})

		d.keys(c.Keys...)

		assert.Equal(t, form.Cancelled, d.w.Ending(), name)
	}
}

func TestAPlanThatCannotApplySaysWhyAndOffersOnlyToCancel(t *testing.T) {
	refused := domain.Invalid([]domain.FieldError{{Path: "from", Code: "not_found", Template: "{path}: {value} not found", Params: domain.Params{"value": "ghost"}}})
	d := open(t, wizardParams{App: refusing{useCase: newApp(seed{}), err: refused}, Request: create.Request{Branches: []string{"feat/a"}, From: "main", EnvFrom: project.EnvExample}})

	screen := d.screen()
	assert.Contains(t, screen, "Step 7/7 • Cannot apply this plan")
	assert.Contains(t, screen, "✗ the request is invalid\n  ┃      Source branch: ghost not found")
	assert.NotContains(t, screen, "Yes, apply")
}

func TestAStaleRunAsksNothingAgainAndSaysWhy(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{})})
	d.keys("feat/a", "enter", "enter", "enter")
	require.Equal(t, int(stepRecap), d.w.Current())

	again := start(t, d.c, d.w.Again())

	assert.Equal(t, int(stepRecap), again.w.Current())
	screen := again.screen()
	assert.Contains(t, screen, "✓ Source branch: main\n")
	assert.Contains(t, screen, "! the facts changed since the preview")
}

func TestADryRunAsksTheQuestionsAndNoConfirmation(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{}), DryRun: true})

	assert.Contains(t, d.screen(), "Step 1/6 • Branches")
	d.keys("feat/a", "enter", "enter", "enter")

	assert.Equal(t, form.Finished, d.w.Ending())
	assert.Nil(t, d.c.preview)
}

func TestIfNotExistsGivenByItsFlagStandsInTheTrail(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{}), Request: create.Request{Branches: []string{"feat/a"}, From: "main", EnvFrom: project.EnvExample, IfNotExists: true}})

	assert.Equal(t, int(stepRecap), d.w.Current())
	assert.Contains(t, d.screen(), "✓ If not exists: yes · --if-not-exists")
}
