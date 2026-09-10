package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

func envValueList() EnvValueListModel {
	return NewEnvValueList(NewEnvValueListParams{
		Title: "t", Description: "d",
		Fields: []domain.EnvValueField{
			{Job: "keycloak", File: "apps/web/.env", Key: "KEYCLOAK_URL", Current: "http://localhost:8080"},
			{Job: "keycloak", File: "apps/web/.env", Key: "KEYCLOAK_REALM", Current: "myapp", Value: "{namespace}", Linked: true, Vars: testVarGroups()},
		},
	})
}

func evKey(m EnvValueListModel, k tea.KeyType) EnvValueListModel {
	updated, _ := m.Update(tea.KeyMsg{Type: k})
	return updated
}

func evSpace(m EnvValueListModel) EnvValueListModel {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{' '}})
	return updated
}

func evType(m EnvValueListModel, text string) EnvValueListModel {
	for _, r := range text {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

// Space is the whole gesture the step exists for: it is what says "this key
// names my slice".
func TestEnvValueListTogglesAKeyWithSpace(t *testing.T) {
	m := evSpace(envValueList())

	if !m.Fields()[0].Linked {
		t.Error("KEYCLOAK_URL: not linked after space, want it linked")
	}
	if m = evSpace(m); m.Fields()[0].Linked {
		t.Error("KEYCLOAK_URL: still linked after a second space, want it unlinked")
	}
}

// An unlinked row shows what the key holds today — that is the only thing that
// lets a reader tell one opaque key from another — and a linked one shows what
// it becomes.
func TestEnvValueListShowsTheCurrentValueOfAnUnlinkedKey(t *testing.T) {
	view := envValueList().View()

	if !strings.Contains(view, "http://localhost:8080") {
		t.Errorf("view = %q, want the unlinked key's current value", view)
	}
	if !strings.Contains(view, "{namespace}") {
		t.Errorf("view = %q, want the linked key's template", view)
	}
	if !strings.Contains(view, "myapp") {
		t.Errorf("view = %q, want the linked key's current value beside its template", view)
	}
}

// Editing a template is asking for it to be written, so it links the row rather
// than leaving an edit nobody applies.
func TestEnvValueListLinksARowWhenItsTemplateIsEdited(t *testing.T) {
	m := evKey(envValueList(), tea.KeyEnter)

	if !m.Fields()[0].Linked {
		t.Error("KEYCLOAK_URL: not linked after an edit, want editing to link it")
	}
}

// A linked key whose template is empty would have wtm own the line and write
// nothing into it. Unlinking is how a key is left alone.
func TestEnvValueListRefusesAnEmptyTemplate(t *testing.T) {
	m := evKey(envValueList(), tea.KeyDown)
	m = evKey(m, tea.KeyEnter)
	for range len("{namespace}") {
		m = evKey(m, tea.KeyBackspace)
	}
	m = evKey(m, tea.KeyEnter)

	// The banner capitalizes what it is given, so the assertion is on the part
	// that stays put.
	if !strings.Contains(m.View(), "needs a template") {
		t.Errorf("view = %q, want the empty template refused", m.View())
	}
	if m.Fields()[1].Value != "{namespace}" {
		t.Errorf("value = %q, want the template left as it was", m.Fields()[1].Value)
	}
}

// The composite case the vocabulary exists for: an issuer built from where the
// service answers and which slice this worktree holds.
func TestEnvValueListKeepsAnEditedTemplate(t *testing.T) {
	m := evKey(envValueList(), tea.KeyDown)
	m = evKey(m, tea.KeyEnter)
	for range len("{namespace}") {
		m = evKey(m, tea.KeyBackspace)
	}
	m = evType(m, "{origin}/realms/{namespace}")
	m = evKey(m, tea.KeyEnter)

	if got := m.Fields()[1].Value; got != "{origin}/realms/{namespace}" {
		t.Errorf("value = %q, want the edited template", got)
	}
}

// The vocabulary is shown while typing, where it is needed.
func TestEnvValueListShowsTheVocabularyWhileEditing(t *testing.T) {
	m := evKey(envValueList(), tea.KeyDown)
	m = evKey(m, tea.KeyEnter)

	if !strings.Contains(m.View(), domain.NamespaceVarsHeading) {
		t.Errorf("view = %q, want the vocabulary while editing", m.View())
	}
}
