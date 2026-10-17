package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

func cmdList() CmdListModel {
	return NewCmdList(NewCmdListParams{
		Title: "t",
		Fixes: []domain.JobCmdFix{
			{Job: "web-dev", Cmd: "vite", Vars: []string{"PORT"}},
			{Job: "api-dev", Cmd: "node server.js", Vars: []string{"API_PORT"}},
		},
	})
}

func typeInto(m CmdListModel, s string) CmdListModel {
	for _, r := range s {
		m, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	return m
}

func TestCmdListShowsTheCommandAndTheVariableItIgnores(t *testing.T) {
	view := cmdList().View()

	for _, want := range []string{"web-dev", "vite", "PORT"} {
		if !strings.Contains(view, want) {
			t.Errorf("view is missing %q:\n%s", want, view)
		}
	}
}

func TestCmdListEditsACommandInPlace(t *testing.T) {
	m, _ := cmdList().Update(key(tea.KeyEnter))
	m = typeInto(m, " --port ${PORT}")
	m, _ = m.Update(key(tea.KeyEnter))

	if got := m.Fixes()[0].Cmd; got != "vite --port ${PORT}" {
		t.Errorf("cmd = %q, want the edited command", got)
	}
}

func TestCmdListStartsFromTheCurrentCommand(t *testing.T) {
	// The command is amended, not retyped: an empty field would mean deleting it.
	m, _ := cmdList().Update(key(tea.KeyEnter))

	if got := m.input.Value(); got != "vite" {
		t.Errorf("the field opened on %q, want the current command", got)
	}
}

func TestCmdListRefusesAnEmptyCommand(t *testing.T) {
	m, _ := cmdList().Update(key(tea.KeyEnter))
	for range "vite" {
		m, _ = m.Update(key(tea.KeyBackspace))
	}
	m, _ = m.Update(key(tea.KeyEnter))

	if m.Fixes()[0].Cmd != "vite" {
		t.Errorf("an empty command must be refused, got %q", m.Fixes()[0].Cmd)
	}
	if m.err == "" {
		t.Error("the refusal must say why")
	}
}

func TestCmdListDoneRowConfirms(t *testing.T) {
	m := cmdList()
	m, _ = m.Update(key(tea.KeyDown))
	m, _ = m.Update(key(tea.KeyDown))
	m, _ = m.Update(key(tea.KeyEnter))

	if !m.Done() {
		t.Error("enter on the Done row must confirm the step")
	}
}

func TestCmdListEscapeAborts(t *testing.T) {
	m, _ := cmdList().Update(key(tea.KeyEsc))

	if !m.Aborted() {
		t.Error("escape must go back")
	}
}

func TestCmdListRowStopsFlaggingAFixedCommand(t *testing.T) {
	m, _ := cmdList().Update(key(tea.KeyEnter))
	m = typeInto(m, " --port ${PORT}")
	m, _ = m.Update(key(tea.KeyEnter))

	first := strings.SplitN(m.View(), "\n", 2)[0]
	if strings.Contains(first, "PORT ") {
		t.Errorf("the row still names a variable the command now references:\n%s", first)
	}
	if !strings.Contains(first, domain.CmdListReferenced) {
		t.Errorf("a fixed row must read as done:\n%s", first)
	}
}

func TestCmdListKeepsTheVariableVisibleWhileEditing(t *testing.T) {
	// The variable to type is the one thing the row exists for: hiding it behind
	// the input leaves nothing to go on.
	m, _ := cmdList().Update(key(tea.KeyEnter))

	first := strings.SplitN(m.View(), "\n", 2)[0]
	if !strings.Contains(first, "PORT") {
		t.Errorf("the edited row no longer names the variable:\n%s", first)
	}
}
