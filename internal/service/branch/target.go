package branch

import (
	"context"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

// Target inspects the branch a worktree is about to be created on: whether a
// local branch of that name already exists, whether another worktree (including
// the main one) already has it checked out, and how it diverges from origin.
// Callers use it to decide whether the source branch is a real start-point or
// only the recorded sync parent, and to refuse a branch git cannot check out
// twice. Best effort: an unreadable repo classifies as BranchTargetNew, leaving
// git itself to report the failure.
func Target(ctx context.Context, params BranchParams) domain.BranchTarget {
	if !infra.LocalBranchExists(ctx, infra.LocalBranchExistsParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	}) {
		return domain.BranchTarget{State: domain.BranchTargetNew, Branch: params.Branch}
	}

	state, ab := Divergence(ctx, params)

	wt, err := infra.FindWorktreeByBranch(ctx, infra.FindWorktreeByBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	})
	if err != nil {
		return domain.BranchTarget{
			State:       domain.BranchTargetExisting,
			Branch:      params.Branch,
			Origin:      state,
			AheadBehind: ab,
		}
	}

	return domain.BranchTarget{
		State:        domain.BranchTargetCheckedOut,
		Branch:       params.Branch,
		WorktreePath: wt.Path,
		Origin:       state,
		AheadBehind:  ab,
	}
}
