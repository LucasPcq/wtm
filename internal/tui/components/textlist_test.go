package components

import (
	"errors"
	"slices"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func newTestTextList(entries ...string) TextListModel {
	return NewTextList(NewTextListParams{
		Title:    "Branches",
		Entries:  entries,
		Required: "at least one",
		Check: func(e TextListEntry) (string, error) {
			name := strings.TrimSpace(e.Entry)
			if name == "" {
				return "", errors.New("blank")
			}
			if slices.Contains(e.Entries, name) {
				return "", errors.New("twice")
			}
			return name, nil
		},
	})
}

func typeInList(m TextListModel, text string) TextListModel {
	for _, r := range text {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func pressList(m TextListModel, keys ...tea.KeyType) TextListModel {
	for _, k := range keys {
		m, _ = m.Update(tea.KeyMsg{Type: k})
	}
	return m
}

func TestTextListTabAddsAndClears(t *testing.T) {
	m := pressList(typeInList(newTestTextList(), "feat/a"), tea.KeyTab)
	if got := strings.Join(m.Values(), ","); got != "feat/a" {
		t.Fatalf("values = %q", got)
	}
	if m.Done() {
		t.Error("tab must not end the step")
	}
	m = pressList(typeInList(m, "feat/b"), tea.KeyTab)
	if got := strings.Join(m.Values(), ","); got != "feat/a,feat/b" {
		t.Errorf("values = %q", got)
	}
}

func TestTextListEnterAddsThenContinues(t *testing.T) {
	m := pressList(typeInList(newTestTextList(), "feat/a"), tea.KeyEnter)
	if !m.Done() || strings.Join(m.Values(), ",") != "feat/a" {
		t.Fatalf("done=%v values=%v, want feat/a added and the step done", m.Done(), m.Values())
	}
}

func TestTextListEnterOnEmptyFieldContinues(t *testing.T) {
	m := pressList(newTestTextList("feat/a"), tea.KeyEnter)
	if !m.Done() {
		t.Error("enter on an empty field with entries should continue")
	}
}

func TestTextListEnterWithNoEntryStays(t *testing.T) {
	m := pressList(newTestTextList(), tea.KeyEnter)
	if m.Done() {
		t.Error("enter with an empty list must not continue")
	}
	if !strings.Contains(strings.ToLower(m.View()), "at least one") {
		t.Errorf("view %q should say an entry is required", m.View())
	}
}

func TestTextListEnterWithAnInvalidEntryStays(t *testing.T) {
	m := pressList(typeInList(newTestTextList("feat/a"), "feat/a"), tea.KeyEnter)
	if m.Done() {
		t.Fatal("an invalid entry must not be dropped silently on enter")
	}
	if !strings.Contains(strings.ToLower(m.View()), "twice") {
		t.Errorf("view %q should show the refusal", m.View())
	}
	if len(m.Values()) != 1 {
		t.Errorf("values = %v, the refused entry must not be added", m.Values())
	}
}

func TestTextListBackspaceOnEmptyFieldRemovesLast(t *testing.T) {
	m := pressList(newTestTextList("feat/a", "feat/b"), tea.KeyBackspace)
	if got := strings.Join(m.Values(), ","); got != "feat/a" {
		t.Errorf("values = %q, want the last entry removed", got)
	}
}

func TestTextListBackspaceWhileTypingEditsTheField(t *testing.T) {
	m := pressList(typeInList(newTestTextList("feat/a"), "fe"), tea.KeyBackspace)
	if len(m.Values()) != 1 {
		t.Errorf("values = %v, backspace in a non-empty field must not remove an entry", m.Values())
	}
}

func TestTextListKeepsEntriesAcrossReset(t *testing.T) {
	w := NewWizard([]Step{{Name: "Branches", Model: newTestTextList("feat/a")}})
	w.resetStep(0)
	list, ok := w.steps[0].Model.(TextListModel)
	if !ok || strings.Join(list.Values(), ",") != "feat/a" {
		t.Errorf("model %T after reset, going back must keep the entries", w.steps[0].Model)
	}
}

func TestTextListAdvancesTheWizard(t *testing.T) {
	w := NewWizard([]Step{{Name: "Branches", Model: newTestTextList("feat/a")}})
	updated, _ := w.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if wm, ok := updated.(WizardModel); !ok || !wm.Done() {
		t.Error("enter on the list step should complete the wizard")
	}
}

func TestTextListShowsBadges(t *testing.T) {
	m := NewTextList(NewTextListParams{
		Entries: []string{"feat/a"},
		Badge:   func(string) Badge { return Badge{Text: "existing"} },
	})
	if !strings.Contains(m.View(), "existing") {
		t.Errorf("view %q should carry the entry's badge", m.View())
	}
}
