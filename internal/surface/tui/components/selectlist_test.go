package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func slPressRunes(m SelectListModel, s string) SelectListModel {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)})
	return updated
}

func slPressType(m SelectListModel, t tea.KeyType) SelectListModel {
	updated, _ := m.Update(tea.KeyMsg{Type: t})
	return updated
}

func newTestSelectList() SelectListModel {
	return NewSelectList(NewSelectListParams{
		Items: []SelectItem{
			{Label: "feature-a", Value: "fa"},
			{Label: "feature-b", Value: "fb"},
			{Label: "bugfix-c", Value: "bc"},
		},
	})
}

// slFilterTo enters filter mode, types text, then exits the input with esc,
// which keeps the filter applied (unified semantics).
func slFilterTo(m SelectListModel, text string) SelectListModel {
	m = slPressRunes(m, "/")
	for _, r := range text {
		m = slPressRunes(m, string(r))
	}
	return slPressType(m, tea.KeyEsc)
}

func TestSelectListEscInFilterKeepsFilter(t *testing.T) {
	m := slFilterTo(newTestSelectList(), "feature")

	if m.Aborted() {
		t.Fatal("esc in filter input should not abort")
	}
	if !strings.Contains(m.View(), "/ feature") {
		t.Fatalf("expected filter to stay applied after esc, got:\n%s", m.View())
	}
}

func TestSelectListReenteringFilterRestoresText(t *testing.T) {
	m := slFilterTo(newTestSelectList(), "feature")
	m = slPressRunes(m, "/") // re-enter input mode

	if !strings.Contains(m.View(), "feature") {
		t.Fatalf("expected re-entered filter to restore text, got:\n%s", m.View())
	}
}

func TestSelectListEscClearsFilterThenAborts(t *testing.T) {
	m := slFilterTo(newTestSelectList(), "feature")

	m = slPressType(m, tea.KeyEsc) // first esc: clear active filter
	if m.Aborted() {
		t.Fatal("first esc with active filter should clear, not abort")
	}
	if strings.Contains(m.View(), "/ feature") {
		t.Fatalf("filter should be cleared, got:\n%s", m.View())
	}

	m = slPressType(m, tea.KeyEsc) // second esc: abort
	if !m.Aborted() {
		t.Fatal("esc with no active filter should abort")
	}
}

func TestSelectListFilterNarrowsAndSelects(t *testing.T) {
	m := newTestSelectList()
	m = slPressRunes(m, "/")
	for _, r := range "bug" {
		m = slPressRunes(m, string(r))
	}
	m = slPressType(m, tea.KeyEnter) // confirm highlighted filtered item

	if !m.Chosen() {
		t.Fatal("expected selection confirmed from filter mode")
	}
	if m.Value() != "bc" {
		t.Fatalf("expected bugfix-c selected, got %q", m.Value())
	}
}

func TestSelectListStartsOnTheNamedValue(t *testing.T) {
	m := NewSelectList(NewSelectListParams{
		Title: "t",
		Items: []SelectItem{{Label: "api", Value: "api"}, {Label: "all", Value: "all"}},
		Start: "all",
	})

	if got := m.Value(); got != "all" {
		t.Errorf("the cursor starts on %q, want the named value", got)
	}
}

func TestSelectListIgnoresAnUnknownStart(t *testing.T) {
	m := NewSelectList(NewSelectListParams{
		Title: "t",
		Items: []SelectItem{{Label: "api", Value: "api"}, {Label: "all", Value: "all"}},
		Start: "ghost",
	})

	if got := m.Value(); got != "api" {
		t.Errorf("value = %q, want the first item when the start is unknown", got)
	}
}

// "No matches" answers a filter. A list still waiting for its items has no
// filter to answer, and saying it matched nothing reads as a failed run.
func TestSelectListRendersNothingWhileItHasNoItem(t *testing.T) {
	m := NewSelectList(NewSelectListParams{Title: "Review & confirm"})

	if got := m.View(); got != "" {
		t.Errorf("view = %q, want an empty body until the items arrive", got)
	}
}

func TestSelectListStillReportsAFilterThatMatchesNothing(t *testing.T) {
	m := slFilterTo(newTestSelectList(), "zzz")

	if !strings.Contains(m.View(), "No matches") {
		t.Errorf("a filter matching nothing must say so:\n%s", m.View())
	}
}
