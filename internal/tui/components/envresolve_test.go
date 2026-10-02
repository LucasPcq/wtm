package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

func conflictModel(t *testing.T) EnvResolveModel {
	t.Helper()
	files := []domain.EnvFileResult{{
		Target:   ".env",
		Source:   "main",
		Strategy: domain.EnvStrategyMain,
		Diff: domain.EnvDiff{Mode: domain.EnvModeRefresh, Entries: []domain.EnvKeyDiff{
			{Key: "DB_HOST", Status: domain.EnvKeyConflict, CurrentValue: "cur", ResolvedValue: "res", Source: "main"},
		}},
	}}
	return NewEnvResolve(NewEnvResolveParams{Files: files})
}

func sendEnv(m EnvResolveModel, k tea.KeyMsg) EnvResolveModel {
	out, _ := m.Update(k)
	return out
}

// TestEnvResolveEditPersistsWhenNotCycled: an edit that is left in place is applied.
func TestEnvResolveEditPersistsWhenNotCycled(t *testing.T) {
	m := conflictModel(t)
	m = sendEnv(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	// clear the prefilled "res" then type "mine"
	for range "res" {
		m = sendEnv(m, tea.KeyMsg{Type: tea.KeyBackspace})
	}
	for _, r := range "mine" {
		m = sendEnv(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	m = sendEnv(m, tea.KeyMsg{Type: tea.KeyEnter})

	d := m.Decisions()[0]
	if d.FilledValues["DB_HOST"] != "mine" {
		t.Fatalf("FilledValues = %v, want DB_HOST=mine", d.FilledValues)
	}
}

// TestEnvResolveEditPrefillsSelectedAction: opening the editor prefills the value of
// the currently selected action — "keep" → current value, "use source" → source
// value (regression: it used to always prefill the source value).
func TestEnvResolveEditPrefillsSelectedAction(t *testing.T) {
	// Default selection is "keep" → editing should start from the current value.
	m := conflictModel(t)
	m = sendEnv(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if got := m.input.Value(); got != "cur" {
		t.Fatalf("edit from keep prefilled %q, want current value %q", got, "cur")
	}
	m = sendEnv(m, tea.KeyMsg{Type: tea.KeyEsc}) // cancel edit

	// Cycle to "use source" → editing should start from the resolved value.
	m = sendEnv(m, tea.KeyMsg{Type: tea.KeyRight})
	m = sendEnv(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	if got := m.input.Value(); got != "res" {
		t.Fatalf("edit from use-source prefilled %q, want source value %q", got, "res")
	}
}

// TestEnvResolveRecapShowsOrphanValue: the recap always shows the value, including
// an orphan whether kept or pruned.
func TestEnvResolveRecapShowsOrphanValue(t *testing.T) {
	files := []domain.EnvFileResult{{
		Target:   ".env",
		Source:   "main",
		Strategy: domain.EnvStrategyMain,
		Diff: domain.EnvDiff{Mode: domain.EnvModeRefresh, Entries: []domain.EnvKeyDiff{
			{Key: "OLD_KEY", Status: domain.EnvKeyOrphan, CurrentValue: "stale"},
		}},
	}}
	m := NewEnvResolve(NewEnvResolveParams{Files: files})
	recap := func(m EnvResolveModel) string {
		return strings.Join(rules.EnvResolveRecapLines(rules.EnvResolveRecapParams{Files: files, Decisions: m.Decisions()}), "\n")
	}

	joined := recap(m)
	if !strings.Contains(joined, `OLD_KEY  keep "stale"`) {
		t.Fatalf("recap should show the orphan value on keep, got:\n%s", joined)
	}

	m = sendEnv(m, tea.KeyMsg{Type: tea.KeyRight}) // keep -> prune
	joined = recap(m)
	if !strings.Contains(joined, `OLD_KEY  prune "stale"`) {
		t.Fatalf("recap should show the orphan value on prune, got:\n%s", joined)
	}
}

// TestEnvResolveCycleDiscardsEdit: cycling after an edit drops the stored edit so
// the display and the decision stay consistent (regression for the stale-edit bug).
func TestEnvResolveCycleDiscardsEdit(t *testing.T) {
	m := conflictModel(t)
	m = sendEnv(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'e'}})
	m = sendEnv(m, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'Z'}})
	m = sendEnv(m, tea.KeyMsg{Type: tea.KeyEnter})
	m = sendEnv(m, tea.KeyMsg{Type: tea.KeyRight})
	m = sendEnv(m, tea.KeyMsg{Type: tea.KeyRight})

	row := m.rows[m.cursor]
	if row.useEdit || row.edited != "" {
		t.Fatalf("edit not discarded: useEdit=%v edited=%q", row.useEdit, row.edited)
	}
	d := m.Decisions()[0]
	if len(d.FilledValues) != 0 {
		t.Fatalf("FilledValues should be empty after cycling away from edit, got %v", d.FilledValues)
	}
	if d.Decisions["DB_HOST"] != domain.EnvDecisionKeep {
		t.Fatalf("decision = %v, want keep", d.Decisions)
	}
}

func TestEnvResolveOpensOnTheFlagDefaults(t *testing.T) {
	files := []domain.EnvFileResult{{Target: ".env", Diff: domain.EnvDiff{Entries: []domain.EnvKeyDiff{
		{Key: "DB_HOST", Status: domain.EnvKeyConflict, CurrentValue: "cur", ResolvedValue: "res", Source: "main"},
		{Key: "OLD", Status: domain.EnvKeyOrphan, CurrentValue: "1"},
	}}}}
	m := NewEnvResolve(NewEnvResolveParams{Files: files, Defaults: domain.EnvResolveDefaults{Overwrite: true, Prune: true}})

	d := m.Decisions()[0]
	if d.Decisions["DB_HOST"] != domain.EnvDecisionOverwrite || len(d.PruneKeys) != 1 {
		t.Errorf("decisions = %+v, want the conflict overwritten and the orphan pruned", d)
	}
}
