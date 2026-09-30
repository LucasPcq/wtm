package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func scopeList() ScopeListModel {
	return NewScopeList(NewScopeListParams{
		Title: "t", Description: "d",
		Entries: []rules.ServiceScopeChoice{
			{File: "c.yml", Service: "api", Image: "", Fixed: true, Reason: domain.ScopeReasonBuild},
			{File: "c.yml", Service: "db", Image: "postgres:16"},
			{File: "c.yml", Service: "cache", Image: "redis:7"},
		},
	})
}

func pressScope(m ScopeListModel, key string) ScopeListModel {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return updated
}

// The cursor never lands on a row whose answer is not the reader's to give.
func TestScopeListStartsPastAFixedRow(t *testing.T) {
	if got := scopeList().cursor; got != 1 {
		t.Errorf("cursor = %d, want 1: row 0 is fixed", got)
	}
}

func TestScopeListWalksPastFixedRows(t *testing.T) {
	m := pressScope(scopeList(), "k")
	if m.cursor != 1 {
		t.Errorf("cursor = %d, want it to stay rather than land on the fixed row", m.cursor)
	}
}

func TestScopeListTogglesTheSelectedService(t *testing.T) {
	m := pressScope(scopeList(), " ")
	if m.Entries()[1].Scope != domain.JobScopeShared {
		t.Errorf("scope = %q, want shared", m.Entries()[1].Scope)
	}
	if m.Entries()[0].Scope == domain.JobScopeShared {
		t.Error("a fixed row was toggled")
	}

	back := pressScope(m, " ")
	if back.Entries()[1].Scope != domain.JobScopePerWorktree {
		t.Errorf("scope = %q, want it toggled back", back.Entries()[1].Scope)
	}
}

// A fixed row shows its reason where the others show their answer, so the list
// stays complete without pretending there is a choice.
func TestScopeListShowsAFixedRowsReason(t *testing.T) {
	view := scopeList().View()
	if !strings.Contains(view, domain.ScopeReasonBuild) {
		t.Errorf("view does not carry the reason:\n%s", view)
	}
	if !strings.Contains(view, "postgres:16") {
		t.Errorf("view does not name the image:\n%s", view)
	}
}

func TestScopeListEnterConfirmsAndEscAborts(t *testing.T) {
	done, _ := scopeList().Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !done.Done() {
		t.Error("enter did not confirm")
	}
	aborted, _ := scopeList().Update(tea.KeyMsg{Type: tea.KeyEsc})
	if !aborted.Aborted() {
		t.Error("esc did not abort")
	}
}
