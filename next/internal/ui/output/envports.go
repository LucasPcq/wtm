package output

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain/env"
)

// SettlementNote returns what plan writes into a worktree's .env files in one line, how many ports it shifts and how many owned values it writes, or "" when it writes neither.
func SettlementNote(plan env.Redacted) string {
	var parts []string
	if shifted := len(env.Plan(plan).Rewrites()); shifted > 0 {
		parts = append(parts, fmt.Sprintf("%d %s settled (offset +%d)", shifted, noun(shifted, "port"), plan.Offset))
	}
	if written := changedOwned(plan.Owned); written > 0 {
		parts = append(parts, fmt.Sprintf("%d owned %s written", written, noun(written, "value")))
	}

	return strings.Join(parts, Dot)
}

func changedOwned(owned []env.OwnedEntry) int {
	count := 0
	for _, entry := range owned {
		if entry.Changed {
			count++
		}
	}
	return count
}

func noun(count int, one string) string {
	if count == 1 {
		return one
	}
	return one + "s"
}
