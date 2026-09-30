package envwizard

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestRestoreRecapLinesPreviewWhatVerbatimPutsBack(t *testing.T) {
	entries := []domain.EnvRestoredEntry{
		{File: ".env", Key: "WEB_PORT", From: "3010", To: "3000"},
		{File: ".env", Key: domain.EnvComposeProjectName, From: "repo-feat", Removed: true},
	}

	switched := strings.Join(restoreRecapLines(restoreRecapParams{Entries: entries, Switch: true}), "\n")
	for _, want := range []string{domain.EnvRestoreRecapTitle, "WEB_PORT", `"3000"`, domain.EnvComposeProjectName} {
		if !strings.Contains(switched, want) {
			t.Errorf("switch recap = %q, want %q in it", switched, want)
		}
	}

	if offered := strings.Join(restoreRecapLines(restoreRecapParams{Entries: entries, Offered: true}), "\n"); !strings.Contains(offered, domain.EnvRestoreRecapIfKeptTitle) {
		t.Errorf("offered recap = %q, want the conditional title", offered)
	}
	if got := restoreRecapLines(restoreRecapParams{Entries: entries}); got != nil {
		t.Errorf("no switch and no verbatim action: got %q, want nothing", got)
	}
}
