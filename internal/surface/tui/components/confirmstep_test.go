package components

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// newConfirmStepWizard builds a two-step wizard: a text input followed by a
// ConfirmStep whose relevance and wording derive from the input's value.
func newConfirmStepWizard() WizardModel {
	steps := []Step{
		{Name: "Name", Model: NewTextInput(NewTextInputParams{Title: "Name"}), Summary: TextSummary},
		ConfirmStep(ConfirmStepParams{
			Name: "Confirm",
			Decide: func(prev []Step) (bool, string, NewConfirmParams) {
				name := TextSummary(prev[0].Model)
				if name == "" {
					return false, "no name entered", NewConfirmParams{}
				}
				return true, "", NewConfirmParams{Title: "Delete " + name + "?", DefaultYes: true}
			},
		}),
	}
	return NewWizard(steps)
}

func typeText(m WizardModel, s string) WizardModel {
	for _, r := range s {
		m = updateWizard(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func TestConfirmStepShownWhenDecideApplies(t *testing.T) {
	m := newConfirmStepWizard()
	m = typeText(m, "feature")
	m = updateWizard(m, key(tea.KeyEnter)) // submit name → enter confirm step

	if m.current != 1 {
		t.Fatalf("expected to land on the confirm step (1), got %d", m.current)
	}
	if m.Skipped(1) {
		t.Error("confirm step should not be skipped when Decide applies")
	}
	confirm, ok := m.steps[1].Model.(ConfirmModel)
	if !ok {
		t.Fatalf("expected step 1 to be a ConfirmModel, got %T", m.steps[1].Model)
	}
	if confirm.title != "Delete feature?" {
		t.Errorf("confirm title = %q, want derived from prior step", confirm.title)
	}
}

func TestConfirmStepAutoSkippedWhenDecideDeclines(t *testing.T) {
	m := newConfirmStepWizard()
	// Leave the name empty → Decide returns apply=false.
	m = updateWizard(m, key(tea.KeyEnter)) // submit empty name

	if !m.Skipped(1) {
		t.Error("confirm step should be auto-skipped when Decide declines")
	}
	if !m.Done() {
		t.Error("wizard should complete once the only remaining step is skipped")
	}
}

func TestConfirmStepConfirmedIsReadable(t *testing.T) {
	m := newConfirmStepWizard()
	m = typeText(m, "feature")
	m = updateWizard(m, key(tea.KeyEnter)) // enter confirm step (defaults to Yes)
	m = updateWizard(m, key(tea.KeyEnter)) // accept

	confirm, ok := m.steps[1].Model.(ConfirmModel)
	if !ok {
		t.Fatalf("expected ConfirmModel, got %T", m.steps[1].Model)
	}
	if !confirm.Confirmed() {
		t.Error("expected the confirmation to be recorded as confirmed")
	}
}

func TestConfirmStepEscGoesBackNotAbort(t *testing.T) {
	m := newConfirmStepWizard()
	m = typeText(m, "feature")
	m = updateWizard(m, key(tea.KeyEnter)) // enter confirm step
	if m.current != 1 {
		t.Fatalf("precondition: expected confirm step, got %d", m.current)
	}

	m = updateWizard(m, key(tea.KeyEsc)) // Esc on the confirm → back to the name step

	if m.current != 0 {
		t.Errorf("Esc on the confirm step should go back to step 0, got %d", m.current)
	}
	if m.Aborted() {
		t.Error("Esc on a non-first step must not abort the wizard")
	}
}
