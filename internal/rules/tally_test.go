package rules

import (
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// A conclusion counts what happened, never what did not: a zero count would put
// "0 blocked" on every clean run and teach the reader to skip the line.
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
