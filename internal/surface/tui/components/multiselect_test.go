package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/styles"
)

func pressKey(m MultiSelectModel, key string) MultiSelectModel {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return updated
}

func pressSpecial(m MultiSelectModel, t tea.KeyType) MultiSelectModel {
	updated, _ := m.Update(tea.KeyMsg{Type: t})
	return updated
}

func newFilterMultiSelect() MultiSelectModel {
	return NewMultiSelect(NewMultiSelectParams{
		Items: []MultiSelectItem{
			{Label: "feature-a", Value: "fa"},
			{Label: "feature-b", Value: "fb"},
			{Label: "bugfix-c", Value: "bc"},
		},
	})
}

// filterTo enters filter mode, types text, then applies the filter (enter),
// leaving the model in normal mode with the filter still active.
func filterTo(m MultiSelectModel, text string) MultiSelectModel {
	m = pressKey(m, "/")
	for _, r := range text {
		m = pressKey(m, string(r))
	}
	return pressSpecial(m, tea.KeyEnter)
}

func newTestMultiSelect() MultiSelectModel {
	return NewMultiSelect(NewMultiSelectParams{
		Items: []MultiSelectItem{
			{Label: "a", Value: "a"},
			{Label: "b", Value: "b"},
			{Label: "c", Value: "c"},
		},
	})
}

func TestMultiSelectToggleAllSelectsEverything(t *testing.T) {
	m := pressKey(newTestMultiSelect(), "a")

	if got := m.Values(); len(got) != 3 {
		t.Fatalf("expected all 3 selected, got %v", got)
	}
}

func TestMultiSelectToggleAllClearsWhenFull(t *testing.T) {
	m := pressKey(newTestMultiSelect(), "a") // select all
	m = pressKey(m, "a")                     // toggle back to none

	if got := m.Values(); len(got) != 0 {
		t.Fatalf("expected empty selection after second toggle, got %v", got)
	}
}

func TestMultiSelectToggleAllFromPartialSelectsAll(t *testing.T) {
	m := newTestMultiSelect()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace}) // select first item only
	m = pressKey(m, "a")                            // partial → all

	if got := m.Values(); len(got) != 3 {
		t.Fatalf("expected all 3 selected from partial, got %v", got)
	}
}

func TestMultiSelectSelectAllRespectsFilter(t *testing.T) {
	m := newFilterMultiSelect()
	m = filterTo(m, "feature") // narrows to feature-a, feature-b
	m = pressKey(m, "a")       // select all FILTERED only
	m = pressSpecial(m, tea.KeyEsc)

	got := m.Values()
	if len(got) != 2 || got[0] != "fa" || got[1] != "fb" {
		t.Fatalf("expected only filtered items selected, got %v", got)
	}
}

func TestMultiSelectFilterNoMatchSelectsNothing(t *testing.T) {
	m := newFilterMultiSelect()
	m = filterTo(m, "zzz") // matches nothing
	m = pressKey(m, "a")   // select all filtered = none

	if got := m.Values(); len(got) != 0 {
		t.Fatalf("expected no selection on empty filter, got %v", got)
	}
}

func TestMultiSelectToggleUnderFilterPersistsAcrossFilters(t *testing.T) {
	m := newFilterMultiSelect()

	m = filterTo(m, "feature")                      // cursor on feature-a
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace}) // toggle feature-a
	m = pressSpecial(m, tea.KeyEsc)                 // clear filter

	m = filterTo(m, "bugfix")                       // cursor on bugfix-c
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace}) // toggle bugfix-c
	m = pressSpecial(m, tea.KeyEsc)                 // clear filter

	got := m.Values()
	if len(got) != 2 || got[0] != "fa" || got[2-1] != "bc" {
		t.Fatalf("expected feature-a and bugfix-c selected, got %v", got)
	}
}

func TestMultiSelectEscClearsFilterThenAborts(t *testing.T) {
	m := newFilterMultiSelect()
	m = filterTo(m, "feature")

	m = pressSpecial(m, tea.KeyEsc) // first esc clears the active filter
	if m.Aborted() {
		t.Fatal("first esc with active filter should clear, not abort")
	}

	m = pressSpecial(m, tea.KeyEsc) // second esc aborts
	if !m.Aborted() {
		t.Fatal("esc with no filter should abort")
	}
}

func TestMultiSelectFilterStaysVisibleAfterApply(t *testing.T) {
	m := newFilterMultiSelect()
	m = filterTo(m, "feature") // applies, leaves filter input mode

	if !strings.Contains(m.View(), "/ feature") {
		t.Fatalf("expected active filter to stay visible in normal mode, got:\n%s", m.View())
	}
}

func TestMultiSelectReenteringFilterRestoresText(t *testing.T) {
	m := newFilterMultiSelect()
	m = filterTo(m, "feature") // apply
	m = pressKey(m, "/")       // re-enter filter input

	if !strings.Contains(m.View(), "feature") {
		t.Fatalf("expected re-entered filter to restore previous text, got:\n%s", m.View())
	}
}

func TestMultiSelectFilterAcceptsJandK(t *testing.T) {
	m := NewMultiSelect(NewMultiSelectParams{
		Items: []MultiSelectItem{
			{Label: "jira-1", Value: "j1"},
			{Label: "feature", Value: "f"},
			{Label: "kafka", Value: "k"},
		},
	})
	m = pressKey(m, "/")
	m = pressKey(m, "j") // must be typed into the filter, not navigate
	m = pressSpecial(m, tea.KeyEnter)
	m = pressKey(m, "a") // select all filtered (only jira-1 matches "j")
	m = pressSpecial(m, tea.KeyEsc)

	if got := m.Values(); len(got) != 1 || got[0] != "j1" {
		t.Fatalf("expected only jira-1 selected (j typed into filter), got %v", got)
	}
}

func TestMultiSelectShowsSelectedCountWhenFiltered(t *testing.T) {
	m := newFilterMultiSelect()
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace}) // select feature-a (cursor 0)
	m = pressSpecial(m, tea.KeyDown)
	m = pressSpecial(m, tea.KeyDown)                // cursor on bugfix-c
	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace}) // select bugfix-c
	m = filterTo(m, "feature")                      // hides bugfix-c, still selected

	if !strings.Contains(m.View(), "2 selected") {
		t.Fatalf("expected hidden selection surfaced as count, got:\n%s", m.View())
	}
}

func TestMultiSelectBackspaceEmptyReturnsToNormalMode(t *testing.T) {
	m := newFilterMultiSelect()
	m = pressKey(m, "/")
	m = pressKey(m, "f")
	m = pressSpecial(m, tea.KeyBackspace) // filter now empty → exit filter mode

	m, _ = m.Update(tea.KeyMsg{Type: tea.KeySpace}) // space must toggle, not type
	if got := m.Values(); len(got) != 1 {
		t.Fatalf("expected space to toggle in normal mode, got %v", got)
	}
}

// The focused row wears the same accent bar and subtle tint as every other list
// in the product; the heavy ColorSelectedBg highlight this list was left on read
// as a different component.
func TestMultiSelectFocusUsesTheSharedRowTint(t *testing.T) {
	m := NewMultiSelect(NewMultiSelectParams{Items: []MultiSelectItem{
		{Label: "feat", Value: "feat"},
		{Label: "dev-a", Value: "dev-a"},
	}})
	m.SetSize(SetSizeParams{Width: 40, Height: 10})

	view := m.View()

	if !strings.Contains(view, styles.SelectedMarker.Render("▌ ")) {
		t.Error("the focused row must carry the accent bar")
	}
	if heavy := styles.ListItemSelected.Render(""); heavy != "" && strings.Contains(view, heavy) {
		t.Error("the focused row must not use the heavy list highlight")
	}
}

// Both states start their checkbox at the same column: the bar takes exactly the
// indent an unselected row has.
func TestMultiSelectKeepsTheCheckboxesInOneColumn(t *testing.T) {
	m := NewMultiSelect(NewMultiSelectParams{Items: []MultiSelectItem{
		{Label: "feat", Value: "feat"},
		{Label: "dev-a", Value: "dev-a"},
	}})
	m.SetSize(SetSizeParams{Width: 40, Height: 10})

	lines := strings.Split(ansi.Strip(m.View()), "\n")
	var columns []int
	for _, line := range lines {
		// Only the item rows; the filter prompt above them has brackets of its own.
		for _, box := range []string{"[ ]", "[✓]"} {
			if index := strings.Index(line, box); index >= 0 {
				// Columns, not bytes: the accent bar is a three-byte rune one cell wide.
				columns = append(columns, PrintableWidth(line[:index]))
				break
			}
		}
	}
	if len(columns) < 2 {
		t.Fatalf("expected a checkbox per row, got %v in:\n%s", columns, strings.Join(lines, "\n"))
	}
	for _, column := range columns[1:] {
		if column != columns[0] {
			t.Errorf("checkbox columns = %v, want them aligned", columns)
		}
	}
}

func badgedMultiSelect() MultiSelectModel {
	m := NewMultiSelect(NewMultiSelectParams{
		Items: []MultiSelectItem{
			{Label: "main", Value: "/wt/main", Badges: []Badge{{Text: "2 running", Variant: BadgeSuccess}}},
			{Label: "feat/x", Value: "/wt/x", Badges: []Badge{{Text: "current"}}},
		},
		Start: "/wt/x",
	})
	m.SetSize(SetSizeParams{Width: 60, Height: 10})
	return m
}

// The flow declares "N running" and "current" on each worktree; a widget that
// drops them leaves the picker blind to what it is about to start over.
func TestMultiSelectDrawsEachRowsBadges(t *testing.T) {
	view := ansi.Strip(badgedMultiSelect().View())
	for _, want := range []string{"2 running", "current"} {
		if !strings.Contains(view, want) {
			t.Errorf("view = %q, want the badge %q", view, want)
		}
	}
	for _, line := range strings.Split(view, "\n") {
		if w := ansi.StringWidth(line); w > 60 {
			t.Errorf("line %q is %d wide, want the badges inside the row", line, w)
		}
	}
}

func TestMultiSelectOpensOnItsStart(t *testing.T) {
	m := badgedMultiSelect()
	m = pressKey(m, " ")
	if got := strings.Join(m.Values(), ","); got != "/wt/x" {
		t.Errorf("space toggled %q, want the start row", got)
	}
}
