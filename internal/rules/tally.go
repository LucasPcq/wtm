package rules

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// Tally renders the counted half of a multi-item conclusion — "3 applied ·
// 1 skipped". It is what replaces one line per success: the reader checks the
// count, and only the exceptions are worth a line of their own.
func Tally(parts ...domain.TallyPart) string {
	kept := make([]string, 0, len(parts))
	for _, part := range parts {
		if part.Count == 0 {
			continue
		}
		kept = append(kept, fmt.Sprintf(domain.TallyPartFmt, part.Count, part.Label))
	}
	return strings.Join(kept, domain.TallySeparator)
}
