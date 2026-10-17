package form

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
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

	require.True(t, d.w.finished)
	assert.Equal(t, create.Request{Branches: []string{"feat/a"}, From: "main", EnvFrom: project.EnvMain, Remember: []string{"env_from"}}, d.w.req)
	assert.NotEmpty(t, d.w.preview.Fingerprint)
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
	assert.Equal(t, stepBranches, d.w.current)
}

func TestNoEntryIsRequired(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{})})

	d.keys("enter")

	assert.Contains(t, d.screen(), "✗ Branches is required")
}

func TestWhatAFlagOrARememberedAnswerSettledIsNotAsked(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{Remembered: map[string]string{project.RememberEnvStrategy: "main"}}), Request: create.Request{Branches: []string{"feat/a"}, From: "main"}})

	assert.Equal(t, stepRecap, d.w.current)
	screen := d.screen()
	assert.Contains(t, screen, "✓ Branches: feat/a · <branches>")
	assert.Contains(t, screen, "✓ Source branch: main · --from")
	assert.Contains(t, screen, "✓ Env strategy: main · remembered")
	assert.Contains(t, screen, "esc cancel")
}

func TestAskAsksARememberedAnswerAgainStartingOnIt(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{Remembered: map[string]string{project.RememberEnvStrategy: "main"}}), Request: create.Request{Branches: []string{"feat/a"}, From: "main", Ask: true}})

	assert.Equal(t, stepEnvFrom, d.w.current)
	assert.Equal(t, "main", d.w.envFrom.value())
	assert.Contains(t, d.screen(), "[x] Always use this answer in this repo")
}

func TestASourceBehindOriginIsOfferedAFastForward(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{Behind: true}), Request: create.Request{Branches: []string{"feat/a"}, From: "main", EnvFrom: project.EnvExample}})

	assert.Equal(t, stepFastForward, d.w.current)
	d.keys("up", "enter")

	assert.Equal(t, new(true), d.w.req.FastForward)
	assert.Contains(t, d.screen(), "Fast-forward  main")
}

func TestTheSourceListPinsTheBaseBranchAndRefreshes(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{Behind: true}), Request: create.Request{Branches: []string{"feat/a"}}})

	assert.Equal(t, stepFrom, d.w.current)
	assert.Contains(t, d.screen(), "▌› main")
	assert.Contains(t, d.screen(), "1 behind")

	d.keys("down", "r")

	assert.Equal(t, "feature", d.w.from.value())
	assert.Empty(t, d.w.loading)
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

		assert.True(t, d.w.aborted, name)
	}
}

func TestAPlanThatCannotApplySaysWhyAndOffersOnlyToCancel(t *testing.T) {
	refused := domain.Invalid([]domain.FieldError{{Path: "from", Code: "not_found", Template: "{path}: {value} not found", Params: domain.Params{"value": "ghost"}}})
	d := open(t, wizardParams{App: refusing{App: newApp(seed{}), err: refused}, Request: create.Request{Branches: []string{"feat/a"}, From: "main", EnvFrom: project.EnvExample}})

	screen := d.screen()
	assert.Contains(t, screen, "Step 7/7 • Cannot apply this plan")
	assert.Contains(t, screen, "✗ the request is invalid\n  ┃      Source branch: ghost not found")
	assert.NotContains(t, screen, "Yes, apply")
}

func TestAStaleRunAsksNothingAgainAndSaysWhy(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{})})
	d.keys("feat/a", "enter", "enter", "enter")
	require.Equal(t, stepRecap, d.w.current)

	again := start(t, d.w.again())

	assert.Equal(t, stepRecap, again.w.current)
	screen := again.screen()
	assert.Contains(t, screen, "✓ Source branch: main\n")
	assert.Contains(t, screen, "! the facts changed since the preview")
}

func TestADryRunAsksTheQuestionsAndNoConfirmation(t *testing.T) {
	d := open(t, wizardParams{App: newApp(seed{}), DryRun: true})

	assert.Contains(t, d.screen(), "Step 1/6 • Branches")
	d.keys("feat/a", "enter", "enter", "enter")

	assert.True(t, d.w.finished)
	assert.Nil(t, d.w.preview)
}
