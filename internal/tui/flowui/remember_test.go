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
	if got := names(plan.steps); len(got) != 1 || got[0] != "Recap" {
		t.Fatalf("steps = %v, want the recap alone", got)
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
