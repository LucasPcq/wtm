package rules

import "github.com/LucasPcq/wtm/internal/domain"

// AllocateOrdinal returns the smallest number no live worktree holds. Ordinal 0
// belongs to the main checkout, so allocation starts at 1; taken may contain
// duplicates, zeroes and gaps, and a gap is reused rather than grown past —
// a worktree removed frees its number for the next one.
func AllocateOrdinal(taken []int) int {
	used := make(map[int]bool, len(taken))
	for _, n := range taken {
		used[n] = true
	}

	for candidate := domain.MainWorktreeOrdinal + 1; ; candidate++ {
		if !used[candidate] {
			return candidate
		}
	}
}

// OrdinalHolder is one live worktree's claim on a number.
type OrdinalHolder struct {
	Branch  string
	Ordinal int
}

type KeepsOrdinalParams struct {
	Branch  string
	Ordinal int
	// Others are the live worktrees other than this one, with what each holds.
	Others []OrdinalHolder
}

// KeepsOrdinal reports whether a worktree may keep the number recorded for it.
// It loses only to another live worktree holding the same one, and then only if
// that worktree's branch sorts first.
//
// The tie-break is what makes the repair converge. Without it both sides of a
// duplicate re-allocate, compute the same free number and collide again, run
// after run; with it exactly one of them moves, whichever is asked first.
func KeepsOrdinal(params KeepsOrdinalParams) bool {
	if params.Ordinal <= domain.MainWorktreeOrdinal {
		return false
	}
	for _, other := range params.Others {
		if other.Ordinal == params.Ordinal && other.Branch < params.Branch {
			return false
		}
	}
	return true
}

// TakenOrdinals projects holders onto the numbers AllocateOrdinal must avoid.
func TakenOrdinals(holders []OrdinalHolder) []int {
	taken := make([]int, 0, len(holders))
	for _, holder := range holders {
		taken = append(taken, holder.Ordinal)
	}
	return taken
}
