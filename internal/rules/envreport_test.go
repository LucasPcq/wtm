package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func fileWith(entries ...domain.EnvKeyDiff) domain.EnvFileResult {
	return domain.EnvFileResult{
		Target:  ".env",
		Applied: true,
		Diff:    domain.EnvDiff{Mode: domain.EnvModeRefresh, Entries: entries},
	}
}

// The detail column has to start at the same offset whatever a row's status is.
// A block whose columns wander reads as noise beside the port table it sits next
// to — which is the whole reason the rows are built here rather than printed one
// by one with whatever glyph each printer happens to carry.
func TestEnvKeyRowsAlignTheDetailColumn(t *testing.T) {
	rows := EnvKeyRows(EnvKeyRowsParams{File: fileWith(
		domain.EnvKeyDiff{Key: "JWT_SECRET", Status: domain.EnvKeyResolved, ResolvedValue: "x", Source: domain.EnvSourceMain},
		domain.EnvKeyDiff{Key: "A", Status: domain.EnvKeyMissing, Placeholder: "p"},
		domain.EnvKeyDiff{Key: "OLD", Status: domain.EnvKeyOrphan, CurrentValue: "z"},
	), Check: true})

	if len(rows) != 3 {
		t.Fatalf("EnvKeyRows() returned %d rows, want 3", len(rows))
	}

	want := detailStart(rows[0].Text)
	for _, r := range rows[1:] {
		if got := detailStart(r.Text); got != want {
			t.Errorf("detail of %q starts at %d, want %d", r.Text, got, want)
		}
	}
}

// detailStart is the offset of the first character after the padded key column.
func detailStart(text string) int {
	key := strings.Fields(text)[0]
	rest := text[len(key):]
	return len(key) + len(rest) - len(strings.TrimLeft(rest, " "))
}

func TestEnvKeyRowsOrderAndStatuses(t *testing.T) {
	rows := EnvKeyRows(EnvKeyRowsParams{File: fileWith(
		domain.EnvKeyDiff{Key: "ORPHAN", Status: domain.EnvKeyOrphan, CurrentValue: "z"},
		domain.EnvKeyDiff{Key: "CONFLICT", Status: domain.EnvKeyConflict, CurrentValue: "a", ResolvedValue: "b", Source: domain.EnvSourceMain},
		domain.EnvKeyDiff{Key: "ADDED", Status: domain.EnvKeyResolved, ResolvedValue: "x", Source: domain.EnvSourceMain},
		domain.EnvKeyDiff{Key: "MISSING", Status: domain.EnvKeyMissing, Placeholder: "p"},
	), Check: true})

	want := []domain.EnvKeyStatus{domain.EnvKeyResolved, domain.EnvKeyConflict, domain.EnvKeyMissing, domain.EnvKeyOrphan}
	for i, status := range want {
		if rows[i].Status != status {
			t.Errorf("row %d is %q, want %q — added, contested, unanswered, left over", i, rows[i].Status, status)
		}
	}
}

// An apply counts what it did and names only what it left for the reader; a
// check lists every key it would touch, since that list is what it was asked for.
func TestEnvKeyRowsOfAnApplyNameOnlyWhatIsLeft(t *testing.T) {
	file := fileWith(
		domain.EnvKeyDiff{Key: "ADDED", Status: domain.EnvKeyResolved, ResolvedValue: "x", Source: domain.EnvSourceMain, Action: domain.EnvActionAdded},
		domain.EnvKeyDiff{Key: "OVER", Status: domain.EnvKeyConflict, CurrentValue: "a", ResolvedValue: "b", Source: domain.EnvSourceMain, Action: domain.EnvActionOverwritten},
		domain.EnvKeyDiff{Key: "KEPT", Status: domain.EnvKeyConflict, CurrentValue: "a", ResolvedValue: "b", Source: domain.EnvSourceMain, Action: domain.EnvActionKept},
		domain.EnvKeyDiff{Key: "GONE", Status: domain.EnvKeyOrphan, CurrentValue: "z", Action: domain.EnvActionPruned},
		domain.EnvKeyDiff{Key: "MISSING", Status: domain.EnvKeyMissing, Placeholder: "p"},
	)

	rows := EnvKeyRows(EnvKeyRowsParams{File: file})
	if len(rows) != 2 || !strings.HasPrefix(rows[0].Text, "KEPT") || !strings.Contains(rows[0].Text, "conflict kept") || !strings.HasPrefix(rows[1].Text, "MISSING") {
		t.Errorf("rows = %+v, want the kept conflict and the unanswered key alone", rows)
	}
	if got := EnvFileTally(file); got != "1 added · 1 overwritten · 1 pruned" {
		t.Errorf("tally = %q", got)
	}

	check := EnvKeyRows(EnvKeyRowsParams{File: fileWith(file.Diff.Entries[0]), Check: true})
	if len(check) != 1 || !strings.HasSuffix(check[0].Text, "would be added from main") {
		t.Errorf("check rows = %+v, want the addition listed", check)
	}
}

func TestEnvQuoteNamesTheEmptyValue(t *testing.T) {
	if got := EnvQuote(""); got != "(empty)" {
		t.Errorf("EnvQuote(\"\") = %q, want a visible marker", got)
	}
}

// A key row states what a key is, never that the run succeeded. The tick is
// reserved for the file verdict and the closing line, so a pending addition can
// no longer be mistaken for one already written.
func TestEnvKeyRowsCarryNoOutcome(t *testing.T) {
	rows := EnvKeyRows(EnvKeyRowsParams{File: fileWith(
		domain.EnvKeyDiff{Key: "ADDED", Status: domain.EnvKeyResolved, ResolvedValue: "x", Source: domain.EnvSourceMain},
		domain.EnvKeyDiff{Key: "MISSING", Status: domain.EnvKeyMissing, Placeholder: "p"},
		domain.EnvKeyDiff{Key: "ORPHAN", Status: domain.EnvKeyOrphan, CurrentValue: "z"},
	), Check: true})

	for _, r := range rows {
		if strings.Contains(r.Text, domain.EnvKeyGlyphAdd) && r.Status != domain.EnvKeyResolved {
			t.Errorf("row %q carries a glyph the surface should own", r.Text)
		}
		if strings.Contains(r.Text, "✓") {
			t.Errorf("row %q claims an outcome", r.Text)
		}
	}
}

func TestEnvReportFields(t *testing.T) {
	got := EnvReportFields(domain.EnvSyncResult{Branch: "feat/x", Mode: domain.EnvModeRefresh})
	if len(got) != 2 || got[0].Value != "feat/x" || got[1].Value != "refresh" {
		t.Fatalf("EnvReportFields() = %+v, want the worktree and the mode", got)
	}

	check := EnvReportFields(domain.EnvSyncResult{Branch: "feat/x", Mode: domain.EnvModeAdd, Check: true})
	if !strings.Contains(check[1].Value, "read-only") {
		t.Errorf("mode = %q, want a read-only check to say so", check[1].Value)
	}

	// A result with no branch (nothing was selected) shows the mode alone rather
	// than an empty row.
	if got := EnvReportFields(domain.EnvSyncResult{Mode: domain.EnvModeAdd}); len(got) != 1 {
		t.Errorf("EnvReportFields() = %+v, want the mode alone", got)
	}
}

func TestEnvFileVerdictTellsASettledPassInThePast(t *testing.T) {
	cases := []struct {
		params   EnvFileVerdictParams
		expected string
	}{
		{EnvFileVerdictParams{}, domain.EnvFileInSyncMessage},
		{EnvFileVerdictParams{PortsMove: true, Check: true}, domain.EnvFileKeysInSyncMessage},
		{EnvFileVerdictParams{PortsMove: true}, domain.EnvFileValuesSettledMessage},
	}
	for _, tc := range cases {
		if got := EnvFileVerdict(tc.params); got != tc.expected {
			t.Errorf("EnvFileVerdict(%+v) = %q, want %q", tc.params, got, tc.expected)
		}
	}
}
