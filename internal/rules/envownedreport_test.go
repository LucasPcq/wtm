package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// ownedOnlyResult is a run whose only work is a key wtm writes in full — the
// shape of a worktree whose [[env]] link moves its DATABASE_URL onto its own
// slice, with no port to shift and no key missing.
func ownedOnlyResult(applied bool) domain.EnvSyncResult {
	return domain.EnvSyncResult{
		Files: []domain.EnvFileResult{{Target: "apps/crm/api/.env"}},
		Ports: domain.EnvPortPlan{
			Applied: applied,
			Owned: []domain.EnvOwnedEntry{{
				File: "apps/crm/api/.env", Key: "DATABASE_URL",
				Value: "postgresql://app@localhost:5432/app_feat-a", Changed: true,
			}},
		},
	}
}

// The rule beside this one already says it: "saying no changes written there is
// not a wording problem, it is a false report". A key wtm owns is written the
// same way a port is.
func TestEnvOutcomeSummaryCountsAKeyWtmWritesInFull(t *testing.T) {
	summary := EnvOutcomeSummary(ownedOnlyResult(true))

	if summary.Text == domain.EnvNothingWrittenMessage {
		t.Fatal("summary says nothing was written, but DATABASE_URL was rewritten")
	}
	if summary.Verdict != domain.EnvVerdictDone {
		t.Errorf("verdict = %v, want it read as an accomplishment", summary.Verdict)
	}
	if !strings.Contains(summary.Text, "1") {
		t.Errorf("summary = %q, want it to count the one value settled", summary.Text)
	}
}

// A run with nothing to do still says so.
func TestEnvOutcomeSummarySaysNothingWhenTheOwnedKeyIsAlreadyRight(t *testing.T) {
	result := ownedOnlyResult(true)
	result.Ports.Owned[0].Changed = false

	if got := EnvOutcomeSummary(result); got.Text != domain.EnvNothingWrittenMessage {
		t.Errorf("summary = %q, want no changes written", got.Text)
	}
}

// `wtm env --check` answering "clean" about a worktree whose DATABASE_URL still
// names another worktree's slice is the same false report, in the mode whose
// whole job is to catch it.
func TestEnvHasDriftSeesAPendingOwnedWrite(t *testing.T) {
	if !EnvHasDrift(ownedOnlyResult(false)) {
		t.Error("no drift reported, want the pending owned write seen")
	}
}

func TestEnvHasNoDriftWhenTheOwnedKeyIsSettled(t *testing.T) {
	result := ownedOnlyResult(false)
	result.Ports.Owned[0].Changed = false

	if EnvHasDrift(result) {
		t.Error("drift reported, want none")
	}
}

// LUC-274: a check that found a hand-edited [[env]] key said only "keys in
// sync — its linked values would move", naming nothing.
func TestEnvOwnedRowsNameTheKeyACheckWouldRewrite(t *testing.T) {
	plan := ownedOnlyResult(false).Ports

	rows := EnvOwnedRows(EnvOwnedRowsParams{Plan: plan, File: "apps/crm/api/.env", Check: true})

	if len(rows) != 1 || !strings.HasPrefix(rows[0], "DATABASE_URL ") || !strings.Contains(rows[0], `"postgresql://app@localhost:5432/app_feat-a"`) {
		t.Errorf("rows = %q, want the key named with the value wtm would write", rows)
	}
	if rows := EnvOwnedRows(EnvOwnedRowsParams{Plan: plan, File: ".env", Check: true}); len(rows) != 0 {
		t.Errorf("rows of another file = %q, want none", rows)
	}
	if rows := EnvOwnedRows(EnvOwnedRowsParams{Plan: plan, File: "apps/crm/api/.env"}); len(rows) != 0 {
		t.Errorf("rows of an apply = %q, want none: its summary counts the write", rows)
	}
}
