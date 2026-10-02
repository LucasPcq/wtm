package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestTallyDropsZeroCounts(t *testing.T) {
	got := Tally(
		domain.TallyPart{Count: 3, Label: domain.TallyApplied},
		domain.TallyPart{Count: 0, Label: domain.TallySkipped},
		domain.TallyPart{Count: 1, Label: domain.TallyBlocked},
	)
	if got != "3 applied · 1 blocked" {
		t.Errorf("Tally = %q, want %q", got, "3 applied · 1 blocked")
	}
	if empty := Tally(domain.TallyPart{Count: 0, Label: domain.TallyApplied}); empty != "" {
		t.Errorf("a run that did nothing tallied %q, want nothing", empty)
	}
}

// Every hint in the CLI is one arrow and one bold command, so a reader learns
// once where to look for what to do next.
