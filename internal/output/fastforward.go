package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// FormatFastForwardResults counts what moved and names only what did not: a
// branch already up to date is the same non-event repeated, and one line for all
// of them says it once. Raw body: the command's frame owns the outer padding.
func FormatFastForwardResults(w io.Writer, results []domain.FastForwardResult) {
	moved, notable, failed := rules.FastForwardSplit(results)

	Success(w, Tally(
		TallyPart{Count: len(moved), Label: domain.TallyFastForwarded},
		TallyPart{Count: len(results) - len(moved) - len(notable) - len(failed), Label: domain.TallyUpToDate},
		TallyPart{Count: len(notable) + len(failed), Label: domain.TallySkipped},
	))
	if len(moved) > 0 {
		Message(w, styles.Muted.Render(strings.Join(rules.FastForwardBranches(moved), ", ")))
	}
	for _, result := range notable {
		Warning(w, fmt.Sprintf(domain.FastForwardFailedFmt, result.Branch, rules.FastForwardStatusLabel(result.Status)))
	}
	for _, result := range failed {
		Warning(w, fmt.Sprintf(domain.FastForwardFailedFmt, result.Branch, result.Detail))
	}
}

func WriteFastForwardJSON(w io.Writer, results []domain.FastForwardResult) error {
	if results == nil {
		results = []domain.FastForwardResult{}
	}
	return encodeJSON(w, results)
}
