package flowui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/tui/components"
)

func loadedSelect(content flow.StepContent) flow.Step {
	return flow.Step{
		Kind: flow.StepSelect, Key: "pr", Label: "Pull request",
		Title: "Pick one", Description: "what the list is for",
		LoadingMessage: "Loading pull requests…",
		Load:           func(flow.Answers) (flow.StepContent, error) { return content, nil },
	}
}

func runFirstLoad(t *testing.T, plan *plan) components.WizardModel {
	t.Helper()
	wizard := components.NewWizardWithParams(components.WizardParams{Steps: plan.steps, Loading: true})
	handle := plan.handler()
	cmd, handled := handle(&wizard, plan.initCmd())
	if !handled {
		t.Fatal("the load request must be handled")
	}
	for _, sub := range cmd().(tea.BatchMsg) {
		if done, ok := sub().(loadDoneMsg); ok {
			handle(&wizard, done)
		}
	}
	return wizard
}

func TestALoadedSelectShowsItsTitleWhileItLoads(t *testing.T) {
	plan, err := build(flow.Session{Steps: []flow.Step{loadedSelect(flow.StepContent{})}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if plan.loadingText != "Loading pull requests…" {
		t.Errorf("loadingText = %q, want the step's message", plan.loadingText)
	}
	wizard := components.NewWizardWithParams(components.WizardParams{Steps: plan.steps})
	view := wizard.View()
	for _, want := range []string{"what the list is for"} {
		if !strings.Contains(view, want) {
			t.Errorf("placeholder is missing %q:\n%s", want, view)
		}
	}
}

func TestALoadedSelectFillsItsOptionsAndBanner(t *testing.T) {
	step := loadedSelect(flow.StepContent{
		Options: []flow.Option{
			{Label: "#42 open", Value: "42"},
			{Label: "#9 fork", Value: "9", Disabled: true, Badges: []flow.Badge{{Text: "fork", Tone: domain.ToneWarning}}},
		},
		Banner: flow.Banner{Title: "GitHub not connected", Lines: []string{"run `gh auth login`"}},
	})
	plan, err := build(flow.Session{Steps: []flow.Step{step}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	wizard := runFirstLoad(t, plan)
	list := wizard.Steps()[0].Model.(components.SelectListModel)
	if !strings.Contains(list.View(), "#42 open") || !strings.Contains(wizard.View(), "what the list is for") {
		t.Errorf("the loaded list must keep the step's description and show the options:\n%s", wizard.View())
	}
	if list.Value() != "42" {
		t.Errorf("cursor = %q, want the first enabled option", list.Value())
	}
	if !strings.Contains(wizard.View(), "GitHub not connected") {
		t.Errorf("the banner must be shown once loaded:\n%s", wizard.View())
	}
}

func TestADisabledOptionIsNotPickable(t *testing.T) {
	list := selectList(flow.StepContent{Options: []flow.Option{
		{Label: "linked", Value: "1", Disabled: true},
		{Label: "free", Value: "2"},
	}})
	if list.Value() != "2" {
		t.Errorf("cursor = %q, want it past the disabled option", list.Value())
	}
}

func TestABranchStepPinsWhatItsContentNames(t *testing.T) {
	step := flow.Step{
		Kind: flow.StepBranchSelect, Key: "parent", Label: "Parent",
		Branches:     []domain.BranchCandidate{{Name: "main"}, {Name: "develop"}},
		Pinned:       "main",
		PinnedSuffix: domain.PinnedSuffixBase,
		Build: func(flow.Answers) (flow.StepContent, error) {
			return flow.StepContent{Pinned: "develop"}, nil
		},
	}
	plan, err := build(flow.Session{Steps: []flow.Step{step}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	list := plan.steps[0].Model.(components.SelectListModel)
	if list.Value() != "develop" {
		t.Errorf("first entry = %q, want the content's pin", list.Value())
	}
	if !strings.Contains(list.View(), "develop"+domain.PinnedSuffixBase) {
		t.Errorf("the pin must carry the step's suffix:\n%s", list.View())
	}
}

func TestPinAbsentOffersABranchNoCandidateCarries(t *testing.T) {
	step := flow.Step{
		Kind: flow.StepBranchSelect, Key: "parent", Label: "Parent",
		Branches:  []domain.BranchCandidate{{Name: "main"}},
		Pinned:    "release",
		PinAbsent: true,
	}
	plan, err := build(flow.Session{Steps: []flow.Step{step}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if got := plan.steps[0].Model.(components.SelectListModel).Value(); got != "release" {
		t.Errorf("first entry = %q, want the absent pin offered", got)
	}
}

// The loaded list is a new model: it used to come in at the default width and
// with no height, so fifty pull requests pushed the wizard off a short terminal.
func TestALoadedSelectKeepsTheTerminalSize(t *testing.T) {
	options := make([]flow.Option, 0, 50)
	for i := range 50 {
		options = append(options, flow.Option{Label: fmt.Sprintf("#%d pull request", i), Value: fmt.Sprint(i)})
	}
	plan, err := build(flow.Session{Steps: []flow.Step{loadedSelect(flow.StepContent{Options: options})}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	wizard := components.NewWizardWithParams(components.WizardParams{Steps: plan.steps, Loading: true})
	sized, _ := wizard.Update(tea.WindowSizeMsg{Width: 100, Height: 20})
	wizard = sized.(components.WizardModel)

	handle := plan.handler()
	cmd, _ := handle(&wizard, plan.initCmd())
	for _, sub := range cmd().(tea.BatchMsg) {
		if done, ok := sub().(loadDoneMsg); ok {
			handle(&wizard, done)
		}
	}

	view := wizard.View()
	if lines := strings.Count(view, "\n") + 1; lines > 20 {
		t.Errorf("view is %d lines, want it within the 20-line terminal:\n%s", lines, view)
	}
	if !strings.Contains(view, "what the list is for") {
		t.Errorf("the description scrolled off:\n%s", view)
	}
}
