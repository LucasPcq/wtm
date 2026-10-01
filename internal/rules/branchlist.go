package rules

import (
	"fmt"
	"slices"

	"github.com/LucasPcq/wtm/internal/domain"
)

type BranchEntryParams struct {
	Entry   string
	Entries []string
	// DerivedNames is whether two branches reducing to one name clash: only
	// when run.toml declares jobs, as for a live worktree.
	DerivedNames bool
}

func BranchEntryProblem(params BranchEntryParams) error {
	if slices.Contains(params.Entries, params.Entry) {
		return fmt.Errorf(domain.CreateBranchListedTwiceFmt, params.Entry)
	}
	if !params.DerivedNames {
		return nil
	}
	clash, found := WorktreeNameClash(WorktreeNameClashParams{Branch: params.Entry, Live: params.Entries})
	if !found {
		return nil
	}
	return fmt.Errorf("%w: "+domain.WorktreeNameClashFmt, domain.ErrWorktreeNameTaken, params.Entry, clash.Branch, clash.Name)
}
