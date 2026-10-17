package flowui

import (
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/surface/tui/branchrefresh"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
)

func candidates(names ...string) []domain.BranchCandidate {
	found := make([]domain.BranchCandidate, 0, len(names))
	for _, name := range names {
		found = append(found, domain.BranchCandidate{Name: name})
	}
	return found
}

func refreshableBranchStep(key string, initial []domain.BranchCandidate) flow.Step {
	return flow.Step{
		Kind:     flow.StepBranchSelect,
		Key:      key,
		Label:    "Source branch",
		Title:    "Source branch",
		Branches: initial,
		Refresh:  func() []domain.BranchCandidate { return initial },
	}
}

func startWizard(t *testing.T, steps ...flow.Step) components.WizardModel {
	t.Helper()
	plan, err := build(flow.Session{Steps: steps})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	wizard := components.NewWizardWithParams(components.WizardParams{
		Steps:   plan.steps,
		OnMsg:   plan.handler(t.Context()),
		Loading: true,
	})
	return update(wizard, tea.WindowSizeMsg{Width: 100, Height: 40})
}

func typeText(m components.WizardModel, text string) components.WizardModel {
	return update(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)})
}

func key(m components.WizardModel, keyType tea.KeyType) components.WizardModel {
	return update(m, tea.KeyMsg{Type: keyType})
}

func TestAFetchEndingOnTheNamesStepKeepsWhatWasTyped(t *testing.T) {
	names := flow.Step{
		Kind:  flow.StepTextList,
		Key:   "branches",
		Label: "Branches",
		Build: func(flow.Answers) (flow.StepContent, error) { return flow.StepContent{}, nil },
	}
	wizard := startWizard(t, names, refreshableBranchStep("source", candidates("main")))

	wizard = typeText(wizard, "a")
	wizard = key(wizard, tea.KeyTab)
	wizard = typeText(wizard, "b")
	wizard = update(wizard, branchrefresh.RefreshedMsg{Candidates: candidates("main", "feat/x")})

	if wizard.Loading() {
		t.Error("the spinner must stop once the fetch is back")
	}
	list, ok := wizard.CurrentStepModel().(components.TextListModel)
	if !ok {
		t.Fatalf("current model = %T, want the names step still on screen", wizard.CurrentStepModel())
	}
	if got := strings.Join(list.Values(), ","); got != "a" {
		t.Errorf("entries = %q, want the one added before the fetch ended", got)
	}

	wizard = key(wizard, tea.KeyEnter)
	picker, ok := wizard.CurrentStepModel().(components.SelectListModel)
	if !ok {
		t.Fatalf("current model = %T, want enter to reach the branch step", wizard.CurrentStepModel())
	}
	if got := strings.Join(wizard.Steps()[0].Model.(components.TextListModel).Values(), ","); got != "a,b" {
		t.Errorf("answered names = %q, want both, the typed one included", got)
	}
	if !strings.Contains(picker.View(), "feat/x") {
		t.Errorf("the branch step must read the fetched candidates on entry:\n%s", picker.View())
	}
}

func TestAFetchEndingOnThePickerKeepsTheHighlightedBranch(t *testing.T) {
	wizard := startWizard(t, refreshableBranchStep("source", candidates("main", "feat/x", "feat/y")), recapStep("recap"))

	wizard = key(wizard, tea.KeyDown)
	wizard = key(wizard, tea.KeyDown)
	wizard = update(wizard, branchrefresh.RefreshedMsg{Candidates: candidates("main", "feat/new", "feat/x", "feat/y")})

	picker, ok := wizard.CurrentStepModel().(components.SelectListModel)
	if !ok {
		t.Fatalf("current model = %T, want the picker", wizard.CurrentStepModel())
	}
	if got := picker.Value(); got != "feat/y" {
		t.Errorf("cursor = %q, want the branch highlighted before the fetch ended", got)
	}
	if !strings.Contains(picker.View(), "feat/new") {
		t.Errorf("the picker must show the fetched candidates:\n%s", picker.View())
	}
}

func TestAFetchEndingWhileFilteringLeavesTheFilterAlone(t *testing.T) {
	wizard := startWizard(t, refreshableBranchStep("source", candidates("main", "feat/x")), recapStep("recap"))

	wizard = typeText(wizard, "/")
	wizard = typeText(wizard, "x")
	wizard = update(wizard, branchrefresh.RefreshedMsg{Candidates: candidates("main", "feat/x", "feat/xx")})

	picker, ok := wizard.CurrentStepModel().(components.SelectListModel)
	if !ok {
		t.Fatalf("current model = %T, want the picker", wizard.CurrentStepModel())
	}
	if !picker.Filtering() || picker.Value() != "feat/x" {
		t.Errorf("filtering = %v, cursor = %q, want the filter and its match untouched", picker.Filtering(), picker.Value())
	}
	if wizard.Loading() {
		t.Error("the spinner must stop once the fetch is back")
	}
	if slices.Contains(strings.Fields(picker.View()), "feat/xx") {
		t.Errorf("a list being filtered is not rebuilt under the reader:\n%s", picker.View())
	}
}
