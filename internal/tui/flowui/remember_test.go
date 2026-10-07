package flowui

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

func rememberableStep(memory flow.Memory) flow.Step {
	step := selectStep("env", "example", "main", "parent")
	memory.ID = domain.RememberEnvStrategy
	step.Memory = memory
	return step
}

func TestTheToggleTicksAlwaysUseThisAnswer(t *testing.T) {
	plan, err := build(flow.Session{Steps: []flow.Step{rememberableStep(flow.Memory{}), recapStep("r")}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	wizard := components.NewWizard(plan.steps)
	wizard.Init()
	if view := wizard.View(); !strings.Contains(view, domain.DashboardGlyphCheckOff+" "+domain.RememberToggleLabel) {
		t.Errorf("view does not offer the toggle unticked:\n%s", view)
	}
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyDown})
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyTab})
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter})
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter})

	answers, err := plan.read(wizard)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if answer, _ := answers.Get("env"); answer.Value != "main" || !answer.Remember || !answer.Asked {
		t.Errorf("answer = %+v, want main asked and to be remembered", answer)
	}
}

func TestAStepThatCannotRememberOffersNoToggle(t *testing.T) {
	plan, err := build(flow.Session{Steps: []flow.Step{selectStep("a", "one", "two"), recapStep("r")}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	wizard := components.NewWizard(plan.steps)
	wizard.Init()
	if view := wizard.View(); strings.Contains(view, domain.RememberToggleLabel) {
		t.Errorf("a plain select offers the toggle:\n%s", view)
	}
}

func TestAskOpensTheToggleTickedOnTheRememberedAnswer(t *testing.T) {
	plan, err := build(flow.Session{Steps: []flow.Step{rememberableStep(flow.Memory{Value: "parent", Reask: true}), recapStep("r")}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	wizard := components.NewWizard(plan.steps)
	wizard.Init()
	if view := wizard.View(); !strings.Contains(view, domain.DashboardGlyphCheckOn+" "+domain.RememberToggleLabel) {
		t.Errorf("view does not open the toggle ticked:\n%s", view)
	}
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyTab})
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter})
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter})

	answers, err := plan.read(wizard)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if answer, _ := answers.Get("env"); answer.Value != "parent" || answer.Remember {
		t.Errorf("answer = %+v, want the remembered parent, unticked to be forgotten", answer)
	}
}

func TestARememberedFirstStepIsSettledUpFront(t *testing.T) {
	plan, err := build(flow.Session{Steps: []flow.Step{rememberableStep(flow.Memory{Value: "parent"}), recapStep("r")}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := plan.steps[0].Settled; got != "parent"+domain.RecapRememberedSuffix {
		t.Fatalf("settled = %q, want the remembered parent in place", got)
	}
	wizard := components.NewWizard(plan.steps)
	wizard.Init()
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter})

	answers, err := plan.read(wizard)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if answer, _ := answers.Get("env"); answer.Value != "parent" || !answer.Recalled {
		t.Errorf("answer = %+v, want the remembered parent", answer)
	}
}

// A remembered step behind a condition stays in the session, unseen, so the
// condition is read against the answers given before it.
func TestARememberedConditionalStepIsSettledAgainstEarlierAnswers(t *testing.T) {
	cases := []struct {
		typed string
		want  flow.Answer
	}{
		{typed: "go", want: flow.Answer{Value: "parent", Recalled: true}},
		{typed: "skip", want: flow.Answer{Skipped: true, SkipReason: "nothing to do"}},
	}
	for _, tc := range cases {
		step := rememberableStep(flow.Memory{Value: "parent"})
		step.Skip = func(answers flow.Answers) (bool, string) {
			return answers.Value("name") == "skip", "nothing to do"
		}
		var recapSaw flow.Answer
		recap := recapStep("r")
		build0 := recap.Build
		recap.Build = func(answers flow.Answers) (flow.StepContent, error) {
			recapSaw, _ = answers.Get("env")
			return build0(answers)
		}
		plan, err := build(flow.Session{Steps: []flow.Step{textStep("name"), step, recap}})
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		wizard := components.NewWizard(plan.steps)
		wizard.Init()
		for _, r := range tc.typed {
			wizard = update(wizard, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
		}
		wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter})
		if view := wizard.View(); strings.Contains(view, "example") {
			t.Errorf("%s: the remembered step was shown:\n%s", tc.typed, view)
		}
		wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter})

		answers, err := plan.read(wizard)
		if err != nil {
			t.Fatalf("%s: read: %v", tc.typed, err)
		}
		if answer, _ := answers.Get("env"); !sameAnswer(answer, tc.want) {
			t.Errorf("%s: answer = %+v, want %+v", tc.typed, answer, tc.want)
		}
		if !sameAnswer(recapSaw, tc.want) {
			t.Errorf("%s: the recap saw %+v, want %+v", tc.typed, recapSaw, tc.want)
		}
	}
}

func sameAnswer(a, b flow.Answer) bool {
	return a.Value == b.Value && a.Recalled == b.Recalled && a.Skipped == b.Skipped && a.SkipReason == b.SkipReason
}

func TestATickOnAnAnswerNoMemoryHoldsIsDropped(t *testing.T) {
	step := rememberableStep(flow.Memory{Value: "parent", Reask: true})
	step.Options = append([]flow.Option{{Label: "config default", Value: ""}}, step.Options...)
	plan, err := build(flow.Session{Steps: []flow.Step{step, recapStep("r")}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	wizard := components.NewWizard(plan.steps)
	wizard.Init()
	for range 3 {
		wizard = update(wizard, tea.KeyMsg{Type: tea.KeyUp})
	}
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter})
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter})

	answers, err := plan.read(wizard)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if answer, _ := answers.Get("env"); answer.Value != "" || answer.Remember || !answer.Forget {
		t.Errorf("answer = %+v, want the config default, not to be remembered, the memory to be forgotten", answer)
	}
}

// A question nobody was asked keeps its line in the trail and its place in the
// count: a flag names itself, a remembered answer says so, and the counter
// reads every settled step as done where it stands.
func TestTheTrailReadsBackFlagsAndRememberedAnswers(t *testing.T) {
	name := textStep("name")
	name.Arg = true
	env := rememberableStep(flow.Memory{})
	env.Flag = domain.FlagEnvFrom
	isolation := selectStep("iso", "isolated", "verbatim")
	isolation.Memory = flow.Memory{ID: domain.RememberIsolation, Value: "verbatim"}
	isolation.Skip = func(flow.Answers) (bool, string) { return false, "" }
	session := flow.Session{
		Presets: flow.NewAnswers(map[string]string{"name": "feat/x", "env": "main"}),
		Steps:   []flow.Step{name, env, textStep("asked"), isolation, recapStep("r")},
	}

	plan, err := build(session)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	wizard := sized(components.NewWizard(plan.steps))
	opening := wizard.View()
	for _, want := range []string{"Step 3/5", "✓ Text name: feat/x\n", "✓ Select env: main · --env-from"} {
		if !strings.Contains(opening, want) {
			t.Errorf("opening view should contain %q:\n%s", want, opening)
		}
	}

	for _, r := range "typed" {
		wizard = update(wizard, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter})
	recap := wizard.View()
	for _, want := range []string{"Step 5/5", "✓ Text asked: typed", "✓ Select iso: verbatim" + domain.RecapRememberedSuffix} {
		if !strings.Contains(recap, want) {
			t.Errorf("recap view should contain %q:\n%s", want, recap)
		}
	}
	trail := []string{"✓ Text name", "✓ Select env", "✓ Text asked", "✓ Select iso"}
	for i := 1; i < len(trail); i++ {
		if strings.Index(recap, trail[i-1]) > strings.Index(recap, trail[i]) {
			t.Errorf("trail out of session order: %q before %q:\n%s", trail[i], trail[i-1], recap)
		}
	}

	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter})
	answers, err := plan.read(wizard)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if answers.Value("name") != "feat/x" || answers.Value("env") != "main" || answers.Value("iso") != "verbatim" {
		t.Errorf("answers = %q %q %q, want the presets and the remembered answer", answers.Value("name"), answers.Value("env"), answers.Value("iso"))
	}
}

// Settled at the very front, a remembered step still gets its line, and the
// wizard opens on the first question it asks.
func TestARememberedFirstStepHasItsTrailLine(t *testing.T) {
	plan, err := build(flow.Session{Steps: []flow.Step{rememberableStep(flow.Memory{Value: "parent"}), recapStep("r")}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	wizard := sized(components.NewWizard(plan.steps))
	view := wizard.View()
	for _, want := range []string{"Step 2/2", "✓ Select env: parent" + domain.RecapRememberedSuffix} {
		if !strings.Contains(view, want) {
			t.Errorf("view should contain %q:\n%s", want, view)
		}
	}
	if wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEsc}); !wizard.Aborted() {
		t.Error("esc on the first question asked should back out, not land on the settled step")
	}
}

func TestAnAllSettledSessionOpensNoWizard(t *testing.T) {
	step := rememberableStep(flow.Memory{})
	plan, err := build(flow.Session{Presets: flow.NewAnswers(map[string]string{"env": "main"}), Steps: []flow.Step{step}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if plan.entered != 0 {
		t.Errorf("entered = %d, want nothing to ask", plan.entered)
	}
}

// A remembered step behind a condition that rules it out reads as skipped, not
// as remembered.
func TestARememberedStepRuledOutReadsAsSkipped(t *testing.T) {
	step := rememberableStep(flow.Memory{Value: "parent"})
	step.Skip = func(flow.Answers) (bool, string) { return true, "nothing to do" }
	plan, err := build(flow.Session{Steps: []flow.Step{textStep("name"), step, recapStep("r")}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	wizard := sized(components.NewWizard(plan.steps))
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})
	wizard = update(wizard, tea.KeyMsg{Type: tea.KeyEnter})
	view := wizard.View()
	if !strings.Contains(view, "⊘ Select env — nothing to do") || strings.Contains(view, domain.RecapRememberedSuffix) {
		t.Errorf("view should list the step as skipped, not remembered:\n%s", view)
	}
}

func sized(wizard components.WizardModel) components.WizardModel {
	wizard.Init()
	return update(wizard, tea.WindowSizeMsg{Width: 100, Height: 60})
}
