package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestRecapStepAppendsCancelRow(t *testing.T) {
	step := RecapStep(RecapStepParams{
		Name: "Confirm",
		Build: func(prev []Step) RecapContent {
			return RecapContent{
				Description: "recap body",
				Actions:     []SelectItem{{Label: "Yes, do it", Value: "go"}},
			}
		},
	})

	if !step.Recap {
		t.Error("recap step should be marked Recap")
	}
	if step.AutoSkip != nil {
		t.Error("recap step must be unconditional (no AutoSkip)")
	}

	model, ok := step.Build(nil).(SelectListModel)
	if !ok {
		t.Fatalf("expected a SelectListModel, got %T", step.Build(nil))
	}
	if model.desc != "recap body" {
		t.Errorf("recap description = %q, want %q", model.desc, "recap body")
	}
	if model.Value() != "go" {
		t.Errorf("first row value = %q, want the action", model.Value())
	}
	model, _ = model.Update(key(tea.KeyDown)) // hop past the separator to the cancel row
	if model.Value() != domain.WizardCancelValue {
		t.Errorf("last row = %q, want the cancel sentinel", model.Value())
	}
}

func TestRecapStepIsFinalAndReadsNofN(t *testing.T) {
	steps := []Step{
		{Name: "Name", Model: NewTextInput(NewTextInputParams{Title: "Name"})},
		RecapStep(RecapStepParams{
			Name: "Confirm",
			Build: func(prev []Step) RecapContent {
				return RecapContent{Actions: []SelectItem{{Label: "Yes", Value: "y"}}}
			},
		}),
	}
	m := NewWizard(steps)
	m = updateWizard(m, key(tea.KeyEnter)) // submit name → advance into the recap

	if m.current != 1 {
		t.Fatalf("expected to land on the recap (1), got %d", m.current)
	}
	if m.visiblePosition() != 2 || m.visibleCount() != 2 {
		t.Errorf("recap should read 2/2, got %d/%d", m.visiblePosition(), m.visibleCount())
	}
}

func TestRecapStepRendersDistinctHeader(t *testing.T) {
	steps := []Step{
		{Name: "Name", Model: NewTextInput(NewTextInputParams{Title: "Name"})},
		RecapStep(RecapStepParams{
			Name: "Confirm",
			Build: func(prev []Step) RecapContent {
				return RecapContent{
					Description: "Branch: feat",
					Actions:     []SelectItem{{Label: "Yes", Value: "y"}},
				}
			},
		}),
	}
	m := NewWizard(steps)
	m = updateWizard(m, tea.WindowSizeMsg{Width: 80, Height: 24})
	m = updateWizard(m, key(tea.KeyEnter)) // advance into the recap

	view := m.View()
	if !strings.Contains(view, domain.WizardRecapTitle) {
		t.Errorf("recap view should show the distinct %q header, got:\n%s", domain.WizardRecapTitle, view)
	}
}
