package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

func testVarGroups() []domain.NamespaceVarGroup {
	return []domain.NamespaceVarGroup{
		{Label: "worktree", Vars: []string{"$WTM_NAMESPACE", "$WTM_WORKTREE", "$WTM_ORDINAL"}},
		{Label: "ports", Vars: []string{"$CRM_DB_PORT"}},
	}
}

func namespaceList() NamespaceListModel {
	return NewNamespaceList(NewNamespaceListParams{
		Title: "t", Description: "d",
		Fields: []domain.NamespaceField{
			{Job: "db-crm", Field: domain.NamespaceFieldName, Value: "app_{worktree}", Vars: testVarGroups()},
			{Job: "db-crm", Field: domain.NamespaceFieldCreate, Vars: testVarGroups()},
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
// it declares them by. Grouped and labelled: one run-on line stopped being
// readable as soon as a job declared more than one port.
func TestNamespaceListShowsTheAvailableVariablesWhileTyping(t *testing.T) {
	if view := namespaceList().View(); strings.Contains(view, "CRM_DB_PORT") {
		t.Error("the variables are shown before the field is even open")
	}

	m := nsKey(nsKey(namespaceList(), tea.KeyDown), tea.KeyEnter)
	view := m.View()
	for _, want := range []string{"available", "worktree", "$WTM_NAMESPACE", "ports", "$CRM_DB_PORT"} {
		if !strings.Contains(view, want) {
			t.Errorf("view does not offer %q:\n%s", want, view)
		}
	}

	// Each group on its own line, so the two halves stay apart.
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "$WTM_NAMESPACE") && strings.Contains(line, "$CRM_DB_PORT") {
			t.Errorf("the worktree variables and the ports share a line:\n%s", line)
		}
	}
}

// A long group wraps under its own first variable rather than running off the
// pane or restating its label.
func TestNamespaceListWrapsALongGroupUnderItself(t *testing.T) {
	ports := []string{"$A_PORT", "$B_PORT", "$C_PORT", "$D_PORT", "$E_PORT", "$F_PORT", "$G_PORT", "$H_PORT"}
	m := NewNamespaceList(NewNamespaceListParams{Fields: []domain.NamespaceField{{
		Job: "db", Field: domain.NamespaceFieldCreate,
		Vars: []domain.NamespaceVarGroup{{Label: "ports", Vars: ports}},
	}}})
	m.SetSize(SetSizeParams{Width: 60, Height: 20})
	m = nsKey(m, tea.KeyEnter)

	view := m.View()
	wrapped := 0
	for _, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "_PORT") {
			wrapped++
			if len(line) > 60 {
				t.Errorf("line runs past the pane (%d):\n%s", len(line), line)
			}
		}
	}
	if wrapped < 2 {
		t.Errorf("eight variables did not wrap:\n%s", view)
	}
	if strings.Count(view, "ports") != 1 {
		t.Errorf("the label was repeated on the wrap:\n%s", view)
	}
}

// Three lines are one question in three parts: the service is named once.
func TestNamespaceListNamesEachServiceOnce(t *testing.T) {
	m := NewNamespaceList(NewNamespaceListParams{Fields: []domain.NamespaceField{
		{Job: "db-crm", Field: domain.NamespaceFieldName},
		{Job: "db-crm", Field: domain.NamespaceFieldCreate},
		{Job: "db-crm", Field: domain.NamespaceFieldRemove},
		{Job: "keycloak", Field: domain.NamespaceFieldName},
	}})
	view := m.View()
	if got := strings.Count(view, "db-crm"); got != 1 {
		t.Errorf("db-crm named %d times, want once:\n%s", got, view)
	}
	if got := strings.Count(view, "keycloak"); got != 1 {
		t.Errorf("keycloak named %d times, want once:\n%s", got, view)
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

// The callout re-wraps a line that is too long and loses the hanging indent
// with it, which turns the block back into the run-on prose it replaced. The
// bound is the house convention, held by the steps that came before.
func TestNamespaceStepDescriptionLinesStayWithinTheCallout(t *testing.T) {
	const maxWidth = 72

	for _, line := range strings.Split(domain.NamespaceStepDesc, "\n") {
		if got := len([]rune(line)); got > maxWidth {
			t.Errorf("line is %d wide, past %d — it will wrap and break its indent:\n%s", got, maxWidth, line)
		}
	}
}

// Each of the three fields is explained, and each says what leaving it empty
// means — the escape hatches are the part a reader cannot guess.
func TestNamespaceStepDescriptionCoversTheThreeFields(t *testing.T) {
	for _, want := range []string{
		"name", "create", "remove",
		"$WTM_NAMESPACE",
		"the path to a script",
		"share the service outright",
		"stopping is not destroying",
	} {
		if !strings.Contains(domain.NamespaceStepDesc, want) {
			t.Errorf("the description never mentions %q", want)
		}
	}
}
