package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// newChoiceStepWizard builds a three-step wizard: a text input, a ChoiceStep whose
// relevance derives from a flag, then a trailing step so a skip has somewhere to land.
func newChoiceStepWizard(apply *bool) WizardModel {
	steps := []Step{
		{Name: "Name", Model: NewTextInput(NewTextInputParams{Title: "Name"}), Summary: TextSummary},
		ChoiceStep(ChoiceStepParams{
			Name: "Choice",
			Decide: func(prev []Step) (bool, string, NewSelectListParams) {
				if !*apply {
					return false, "not applicable", NewSelectListParams{}
				}
				return true, "", NewSelectListParams{
					Items: []SelectItem{
						{Label: "Option A", Value: "a"},
						{Label: "Option B", Value: "b"},
					},
				}
			},
		}),
		{Name: "Last", Model: NewTextInput(NewTextInputParams{Title: "Last"})},
	}
	return NewWizard(steps)
}

func TestChoiceStepShownWhenApplies(t *testing.T) {
	apply := true
	m := newChoiceStepWizard(&apply)
	m = typeText(m, "x")
	m = updateWizard(m, key(tea.KeyEnter)) // submit name → enter choice step

	if m.current != 1 {
		t.Fatalf("expected to land on the choice step (1), got %d", m.current)
	}
	if m.Skipped(1) {
		t.Error("choice step should not be skipped when it applies")
	}
	if _, ok := m.steps[1].Model.(SelectListModel); !ok {
		t.Fatalf("expected a SelectListModel, got %T", m.steps[1].Model)
	}
}

func TestChoiceStepAutoSkipsWithReason(t *testing.T) {
	apply := false
	m := newChoiceStepWizard(&apply)
	m = typeText(m, "x")
	m = updateWizard(m, key(tea.KeyEnter)) // submit name → choice auto-skips → land on Last

	if !m.Skipped(1) {
		t.Error("choice step should be auto-skipped when it does not apply")
	}
	if m.current != 2 {
		t.Fatalf("expected to land on the last step (2), got %d", m.current)
	}
	// The skip reason is surfaced in the summaries as "⊘ <Name> — <reason>".
	summary := m.renderSummaries(0)
	if !strings.Contains(summary, "⊘") || !strings.Contains(summary, "not applicable") {
		t.Errorf("expected a skipped-step summary carrying the reason, got:\n%s", summary)
	}
}

func TestChoiceStepEscGoesBackNotAbort(t *testing.T) {
	apply := true
	m := newChoiceStepWizard(&apply)
	m = typeText(m, "x")
	m = updateWizard(m, key(tea.KeyEnter)) // enter choice step
	m = updateWizard(m, key(tea.KeyEsc))   // Esc on the choice → back to the name step

	if m.current != 0 {
		t.Errorf("Esc on the choice step should go back to step 0, got %d", m.current)
	}
	if m.Aborted() {
		t.Error("Esc on a non-first step must not abort the wizard")
	}
}
