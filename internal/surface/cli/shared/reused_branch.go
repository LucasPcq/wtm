package shared

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// ReusedBranchNoteParams holds the divergence-from-origin facts needed to word
// the reused-branch note.
type ReusedBranchNoteParams struct {
	Branch string
	Ahead  int
	Behind int
}

// ReusedBranchNoteResult is the reuse note text plus whether it should render as
// a warning: only a diverged branch means the worktree is missing commits that
// are on origin, which a plain informational line would understate.
type ReusedBranchNoteResult struct {
	Text    string
	Warning bool
}

// ReusedBranchNote states that a worktree checked out an existing local branch
// rather than origin's, and how the two relate — the one thing a user reading
// "Checked out PR #42" or "Created worktree x on existing branch" would otherwise
// get wrong. Shared by create and checkout so the two commands say it identically.
func ReusedBranchNote(p ReusedBranchNoteParams) ReusedBranchNoteResult {
	switch rules.ClassifyDivergence(p.Ahead, p.Behind) {
	case domain.DivergenceBehind:
		return ReusedBranchNoteResult{Text: fmt.Sprintf(domain.BranchReusedBehindWarning, p.Branch, p.Behind)}
	case domain.DivergenceDiverged:
		return ReusedBranchNoteResult{
			Text:    fmt.Sprintf(domain.BranchReusedDivergedWarning, p.Branch, p.Ahead, p.Behind),
			Warning: true,
		}
	default:
		return ReusedBranchNoteResult{Text: fmt.Sprintf(domain.BranchReusedNote, p.Branch)}
	}
}
