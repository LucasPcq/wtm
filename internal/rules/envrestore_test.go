package rules

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// LUC-274: a row elided from the left showed only the list's last origin, the
// same on both sides — the port that moved, at the front, was cut away.
func TestEnvRestoredRowsShowWhatMovedInAList(t *testing.T) {
	rows := EnvRestoredRows([]domain.EnvRestoredEntry{{
		File: ".env", Key: "ORIGINS",
		From: "http://localhost:3040,http://a:" + fakeSecret + "@localhost:3001",
		To:   "http://localhost:3000,http://a:" + fakeSecret + "@localhost:3001",
	}}, ".env")

	if len(rows) != 1 {
		t.Fatalf("rows = %q, want one", rows)
	}
	if !strings.Contains(rows[0], "localhost:3000") || !strings.Contains(rows[0], "localhost:3040") {
		t.Errorf("row = %q, want both ports of the move", rows[0])
	}
	if strings.Contains(rows[0], fakeSecret) {
		t.Errorf("row = %q, want the password masked", rows[0])
	}
}
