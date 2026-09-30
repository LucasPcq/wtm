package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// FormatFastForwardResults counts what moved and names only what did not: a
// branch already up to date is the same non-event repeated, and one line for all
// of them says it once. Raw body: the command's frame owns the outer padding.
func FormatFastForwardResults(w io.Writer, results []domain.FastForwardResult) {
	moved, notable, failed := rules.FastForwardSplit(results)

	tally := Tally(
		TallyPart{Count: len(moved), Label: domain.TallyFastForwarded},
		TallyPart{Count: len(results) - len(moved) - len(notable) - len(failed), Label: domain.TallyUpToDate},
		TallyPart{Count: len(notable), Label: domain.TallySkipped},
		TallyPart{Count: len(failed), Label: domain.TallyFailed},
	)
	// The glyph answers "did this run do what it was asked", so it reads the run
	// and not the count: a tally is one line for several outcomes, and a tick on
	// top of nothing but failures claims a success the exit code denies.
	switch {
	case len(failed) > 0:
		Error(w, tally)
	case len(moved) > 0:
		Success(w, tally)
	default:
		Unchanged(w, tally)
	}
	if len(moved) > 0 {
		Message(w, Indent+strings.Join(rules.FastForwardBranches(moved), ", "))
	}
	// A branch with no upstream, or one that diverged, did not fail: it is a state
	// the reader has to decide about, and calling it a failure says the run broke.
	for _, result := range notable {
		Warning(w, fmt.Sprintf(domain.FastForwardStateFmt, result.Branch, rules.FastForwardStatusLabel(result.Status)))
	}
	for _, result := range failed {
		Error(w, fmt.Sprintf(domain.FastForwardStateFmt, result.Branch, result.Detail))
	}
}

func WriteFastForwardJSON(w io.Writer, results []domain.FastForwardResult) error {
	if results == nil {
		results = []domain.FastForwardResult{}
	}
	return encodeJSON(w, results)
}
