package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/service/provision"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheFormDrawsEachStepAsTheEngineDid(t *testing.T) {
	d := open(t, sessionParams{App: newApp(seed{})})
	d.looksLike("1_branches")

	d.Keys("feat/a", "tab", "feature", "tab")
	d.looksLike("2_branches_entries")

	d.Keys("enter")
	d.looksLike("3_source")

	d.Keys("enter")
	d.looksLike("4_env")

	d.Keys("down", "down", "tab", "enter")
	d.looksLike("5_recap")

	d.Keys("esc")
	d.looksLike("6_back_to_env")
}

func TestTheRequestIsWhatWasAnswered(t *testing.T) {
	d := open(t, sessionParams{App: newApp(seed{})})

	d.Keys("feat/a", "enter", "enter", "down", "down", "tab", "enter", "enter")

	require.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Equal(t, create.Request{Branches: []string{"feat/a"}, From: "main", Given: provision.Given{EnvFrom: project.EnvMain, Remember: []string{"env_from"}}}, d.s.Request())
	assert.NotEmpty(t, d.s.Preview().Fingerprint)
}

func TestAKeptDefaultIsItsValueAlone(t *testing.T) {
	d := open(t, sessionParams{App: newApp(seed{})})

	d.Keys("feat/a", "enter", "enter", "enter")

	assert.Contains(t, d.Screen(), "✓ Source branch: main\n  ✓ Env strategy: config default\n")
}

func TestAnEntryIsCheckedAsItIsAdded(t *testing.T) {
	d := open(t, sessionParams{App: newApp(seed{Linked: []string{"feat/x"}})})

	d.Keys("bad..name", "tab")
	assert.Contains(t, d.Screen(), `✗ "bad..name" is not a valid branch name: it cannot hold .. or end with .`)

	d.Keys("backspace", "backspace", "backspace", "backspace", "backspace", "backspace", "backspace", "backspace", "backspace", "feat/x", "tab")
	assert.Contains(t, d.Screen(), "✗ Branch feat/x is checked out at /elsewhere/feat-x\n  → wtm go feat/x   jump in\n")

	d.Keys("enter")
	assert.Equal(t, stepBranches, d.Wizard.Current())
}

func TestNoEntryIsRequired(t *testing.T) {
	d := open(t, sessionParams{App: newApp(seed{})})

	d.Keys("enter")

	assert.Contains(t, d.Screen(), "✗ Branches is required")
}

func TestWhatAFlagOrARememberedAnswerSettledIsNotAsked(t *testing.T) {
	d := open(t, sessionParams{App: newApp(seed{Remembered: map[string]string{project.RememberEnvStrategy: "main"}}), Request: create.Request{Branches: []string{"feat/a"}, From: "main"}})

	assert.Equal(t, stepRecap, d.Wizard.Current())
	screen := d.Screen()
	assert.Contains(t, screen, "✓ Branches: feat/a\n")
	assert.Contains(t, screen, "✓ Source branch: main · --from")
	assert.Contains(t, screen, "✓ Env strategy: main · remembered")
	assert.Contains(t, screen, "esc cancel")
}

func TestTheRecapMarksARecalledAnswerAndSaysHowToBeAskedIt(t *testing.T) {
	d := open(t, sessionParams{App: newApp(seed{Remembered: map[string]string{project.RememberEnvStrategy: "main"}}), Request: create.Request{Branches: []string{"feat/a"}, From: "main"}})

	screen := d.Screen()
	assert.Contains(t, screen, "┃  Env       main · remembered\n")
	assert.Contains(t, screen, "┃  Remembered answers are not asked: --ask asks them again.")
}

func TestAskAsksARememberedAnswerAgainStartingOnIt(t *testing.T) {
	d := open(t, sessionParams{App: newApp(seed{Remembered: map[string]string{project.RememberEnvStrategy: "main"}}), Request: create.Request{Branches: []string{"feat/a"}, From: "main", Given: provision.Given{Ask: true}}})

	assert.Equal(t, stepEnvFrom, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "▌› main")
	assert.Contains(t, d.Screen(), "[x] Always use this answer in this repo")
}

func TestASourceBehindOriginIsOfferedAFastForward(t *testing.T) {
	d := open(t, sessionParams{App: newApp(seed{Behind: true}), Request: create.Request{Branches: []string{"feat/a"}, From: "main", Given: provision.Given{EnvFrom: project.EnvExample}}})

	assert.Equal(t, stepFastForward, d.Wizard.Current())
	d.Keys("up", "enter")

	assert.Equal(t, new(true), d.s.Request().FastForward)
	assert.Contains(t, d.Screen(), "Fast-forward  main")
}

func TestTheSourceListPinsTheBaseBranchAndRefreshes(t *testing.T) {
	d := open(t, sessionParams{App: newApp(seed{Behind: true}), Request: create.Request{Branches: []string{"feat/a"}}})

	assert.Equal(t, stepFrom, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "▌› main")
	assert.Contains(t, d.Screen(), "1 behind")

	d.Keys("down", "r")

	assert.Contains(t, d.Screen(), "▌› feature")
	assert.NotContains(t, d.Screen(), "Refreshing")
}

func TestEachWayOutCancels(t *testing.T) {
	settled := create.Request{Branches: []string{"feat/a"}, From: "main", Given: provision.Given{EnvFrom: project.EnvExample}}
	cases := map[string]struct {
		Request create.Request
		Keys    []string
	}{
		"esc on the first question": {Keys: []string{"esc"}},
		"no, cancel":                {Request: settled, Keys: []string{"down", "enter"}},
		"ctrl+c":                    {Keys: []string{"ctrl+c"}},
	}
	for name, c := range cases {
		d := open(t, sessionParams{App: newApp(seed{}), Request: c.Request})

		d.Keys(c.Keys...)

		assert.Equal(t, form.Cancelled, d.Wizard.Ending(), name)
	}
}

func TestAPlanThatCannotApplySaysWhyAndOffersOnlyToCancel(t *testing.T) {
	refused := domain.Invalid([]domain.FieldError{{Path: "from", Code: "not_found", Template: "{path}: {value} not found", Params: domain.Params{"value": "ghost"}}})
	d := open(t, sessionParams{App: refusing{useCase: newApp(seed{}), err: refused}, Request: create.Request{Branches: []string{"feat/a"}, From: "main", Given: provision.Given{EnvFrom: project.EnvExample}}})

	screen := d.Screen()
	assert.Contains(t, screen, "Step 7/7 • Cannot apply this plan")
	assert.Contains(t, screen, "✗ the request is invalid\n  ┃      Source branch: ghost not found")
	assert.NotContains(t, screen, "Yes, ")
}

func TestAStaleRunAsksNothingAgainAndSaysWhy(t *testing.T) {
	d := open(t, sessionParams{App: newApp(seed{})})
	d.Keys("feat/a", "enter", "enter", "enter")
	require.Equal(t, stepRecap, d.Wizard.Current())

	again := start(t, d.s, d.Wizard.Again())

	assert.Equal(t, stepRecap, again.Wizard.Current())
	screen := again.Screen()
	assert.Contains(t, screen, "✓ Source branch: main\n")
	assert.Contains(t, screen, "! the facts changed since the preview")
}

func TestADryRunAsksTheQuestionsAndNoConfirmation(t *testing.T) {
	d := open(t, sessionParams{App: newApp(seed{}), Options: domain.RunOptions{DryRun: true}})

	assert.Contains(t, d.Screen(), "Step 1/6 • Branches")
	d.Keys("feat/a", "enter", "enter", "enter")

	assert.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Nil(t, d.s.Preview())
}

func TestIfNotExistsGivenByItsFlagStandsInTheTrail(t *testing.T) {
	d := open(t, sessionParams{App: newApp(seed{}), Request: create.Request{Branches: []string{"feat/a"}, From: "main", Given: provision.Given{EnvFrom: project.EnvExample}, IfNotExists: true}})

	assert.Equal(t, stepRecap, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "✓ If not exists: yes · --if-not-exists")
}

func TestAskAsksARememberedSourceUpdateAgainStartingOnIt(t *testing.T) {
	remembered := map[string]string{project.RememberSourceUpdate: project.SourceUpdateFastForward}
	d := open(t, sessionParams{App: newApp(seed{Behind: true, Remembered: remembered}), Request: create.Request{Branches: []string{"feat/a"}, From: "main", Given: provision.Given{EnvFrom: project.EnvExample, Ask: true}}})

	require.Equal(t, stepFastForward, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "▌› Fast-forward main to origin")
	assert.Contains(t, d.Screen(), "[x] Always use this answer in this repo")
}

func TestARememberedConfigDefaultStartsOnItsRowTicked(t *testing.T) {
	d := open(t, sessionParams{App: newApp(seed{Remembered: map[string]string{project.RememberEnvStrategy: project.RememberEnvConfig}}), Request: create.Request{Branches: []string{"feat/a"}, From: "main", Given: provision.Given{Ask: true}}})

	assert.Equal(t, stepEnvFrom, d.Wizard.Current())
	assert.Contains(t, d.Screen(), "▌› config default (example)")
	assert.Contains(t, d.Screen(), "[x] Always use this answer in this repo")
}

func TestPickingTheConfigDefaultAnswersConfigOverARememberedStrategy(t *testing.T) {
	d := open(t, sessionParams{App: newApp(seed{Remembered: map[string]string{project.RememberEnvStrategy: "main"}}), Request: create.Request{Branches: []string{"feat/a"}, From: "main", Given: provision.Given{Ask: true}}})

	d.Keys("up", "up", "enter")

	assert.Equal(t, provision.EnvFromConfig, d.s.Request().EnvFrom)
	assert.Equal(t, []string{"env_from"}, d.s.Request().Remember)
	assert.Contains(t, d.Screen(), "✓ Env strategy: config default")
}
