package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

func namespaceList() NamespaceListModel {
	return NewNamespaceList(NewNamespaceListParams{
		Title: "t", Description: "d",
		Fields: []domain.NamespaceField{
			{Job: "db-crm", Field: domain.NamespaceFieldName, Value: "app_{worktree}",
				Vars: []string{"$WTM_NAMESPACE", "$WTM_WORKTREE", "$WTM_ORDINAL", "$CRM_DB_PORT"}},
			{Job: "db-crm", Field: domain.NamespaceFieldCreate,
				Vars: []string{"$WTM_NAMESPACE", "$CRM_DB_PORT"}},
			{Job: "db-crm", Field: domain.NamespaceFieldRemove},
		},
	})
}

func nsKey(m NamespaceListModel, k tea.KeyType) NamespaceListModel {
	updated, _ := m.Update(tea.KeyMsg{Type: k})
	return updated
}

func nsType(m NamespaceListModel, text string) NamespaceListModel {
	for _, r := range text {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

// The two commands are the point of the step, and wtm proposes nothing for
// them: an inline command and a script path are both just a /bin/sh line.
func TestNamespaceListAcceptsAnInlineCommandAndAScriptPath(t *testing.T) {
	for _, want := range []string{
		`psql -p $CRM_DB_PORT -c "CREATE DATABASE $WTM_NAMESPACE"`,
		"./scripts/pg-create.sh",
	} {
		m := nsKey(namespaceList(), tea.KeyDown)
		m = nsKey(m, tea.KeyEnter)
		if !m.editing {
			t.Fatal("enter did not open the field")
		}
		m = nsKey(nsType(m, want), tea.KeyEnter)

		if m.editing {
			t.Error("enter did not save")
		}
		if got := m.Fields()[1].Value; got != want {
			t.Errorf("value = %q, want %q", got, want)
		}
	}
}

// A create left empty is an answer: the service is shared outright, data
// included. Only the name is required, because it is what clean names.
func TestNamespaceListRequiresOnlyTheName(t *testing.T) {
	m := nsKey(namespaceList(), tea.KeyDown)
	m = nsKey(nsKey(m, tea.KeyEnter), tea.KeyEnter)
	if m.editing || m.err != "" {
		t.Errorf("an empty create was refused: err = %q", m.err)
	}

	name := nsKey(namespaceList(), tea.KeyEnter)
	for range len("app_{worktree}") {
		name, _ = name.Update(tea.KeyMsg{Type: tea.KeyBackspace})
	}
	name = nsKey(name, tea.KeyEnter)
	if !name.editing || name.err == "" {
		t.Error("an empty name was accepted; it is what clean says it destroys")
	}
}

// Nothing can be guessed about the commands, so the least wtm owes the reader
// is the variables they may actually read — this job's ports, under the names
// it declares them by.
func TestNamespaceListShowsTheAvailableVariablesWhileTyping(t *testing.T) {
	if view := namespaceList().View(); strings.Contains(view, "CRM_DB_PORT") {
		t.Error("the variables are shown before the field is even open")
	}

	m := nsKey(nsKey(namespaceList(), tea.KeyDown), tea.KeyEnter)
	view := m.View()
	for _, want := range []string{"$WTM_NAMESPACE", "$CRM_DB_PORT"} {
		if !strings.Contains(view, want) {
			t.Errorf("view does not offer %s:\n%s", want, view)
		}
	}
}

func TestNamespaceListEnterOnTheLastRowConfirms(t *testing.T) {
	m := namespaceList()
	for range len(m.Fields()) {
		m = nsKey(m, tea.KeyDown)
	}
	if m = nsKey(m, tea.KeyEnter); !m.Done() {
		t.Error("enter on the done row did not confirm")
	}
}
