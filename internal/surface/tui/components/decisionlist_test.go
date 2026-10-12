package components

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func decisionList(validate func(map[string]string) error) DecisionListModel {
	return NewDecisionList(NewDecisionListParams{
		Rows: []DecisionRow{
			{Key: ".env", Options: []string{"copy", "link", "skip"}, Choice: "link"},
			{Key: ".env.local", Options: []string{"copy", "link", "skip"}, Choice: "gone"},
		},
		Validate: validate,
	})
}

func updateDecision(m DecisionListModel, k tea.KeyMsg) DecisionListModel {
	updated, _ := m.Update(k)
	return updated
}

func TestADecisionNoOptionHoldsStartsOnTheFirst(t *testing.T) {
	values := decisionList(nil).Values()

	if values[".env"] != "link" || values[".env.local"] != "copy" {
		t.Errorf("each key keeps a choice it may take, got %v", values)
	}
}

func TestADecisionCyclesBothWaysAroundItsOptions(t *testing.T) {
	m := updateDecision(decisionList(nil), tea.KeyMsg{Type: tea.KeyRight})
	if got := m.Values()[".env"]; got != "skip" {
		t.Fatalf("right moves to the next option, got %q", got)
	}
	m = updateDecision(m, tea.KeyMsg{Type: tea.KeyRight})
	if got := m.Values()[".env"]; got != "copy" {
		t.Fatalf("right past the last wraps to the first, got %q", got)
	}
	m = updateDecision(updateDecision(m, tea.KeyMsg{Type: tea.KeyDown}), tea.KeyMsg{Type: tea.KeyLeft})
	if got := m.Values()[".env.local"]; got != "skip" {
		t.Errorf("left on the second row wraps it to its last option, got %q", got)
	}
}

func TestADecisionListRefusedByItsCheckStaysOpenWithWhy(t *testing.T) {
	m := updateDecision(decisionList(func(map[string]string) error { return errors.New("pick another") }), tea.KeyMsg{Type: tea.KeyEnter})

	if m.Done() {
		t.Fatal("a refused answer does not end the step")
	}
	if !strings.Contains(m.View(), "Pick another") {
		t.Errorf("the refusal shows under the list:\n%s", m.View())
	}
}

func TestADecisionListEndsOnEnterAndBacksOutOnEsc(t *testing.T) {
	if !updateDecision(decisionList(nil), tea.KeyMsg{Type: tea.KeyEnter}).Done() {
		t.Error("enter answers the step")
	}
	if !updateDecision(decisionList(nil), tea.KeyMsg{Type: tea.KeyEsc}).Aborted() {
		t.Error("esc backs out")
	}
}
