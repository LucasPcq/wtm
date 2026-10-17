package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
)

// The worktree a prune stopped on is counted with the rest: a tally that left
// it out would add up to fewer worktrees than the run was given.
func TestPruneTallyCountsTheWorktreeItStoppedOn(t *testing.T) {
	var buf bytes.Buffer
	FormatPruneResult(&buf, domain.PruneResult{
		Pruned:  []domain.PruneCandidate{{Branch: "feat/a"}},
		Skipped: []domain.PruneSkip{{Branch: "feat/c", Reason: domain.PruneSkipInterrupted}},
		Failed:  &domain.PruneFailure{Branch: "feat/b", Error: "cancelled: on_clean: hook stopped"},
	})

	if !strings.Contains(buf.String(), "1 pruned · 1 skipped · 1 failed") {
		t.Errorf("tally must count the stopped worktree:\n%s", buf.String())
	}
}

func TestPruneThatRemovedNothingStillCountsWhatItDidNot(t *testing.T) {
	var buf bytes.Buffer
	FormatPruneResult(&buf, domain.PruneResult{
		Skipped: []domain.PruneSkip{{Branch: "feat/c", Reason: domain.PruneSkipInterrupted}},
		Failed:  &domain.PruneFailure{Branch: "feat/b", Error: "cancelled"},
	})

	if !strings.Contains(buf.String(), domain.GlyphUnchanged+" 1 skipped · 1 failed") {
		t.Errorf("want the unchanged tally:\n%s", buf.String())
	}
}
