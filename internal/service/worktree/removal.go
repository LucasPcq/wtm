package worktree

import (
	"context"
	"errors"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
)

// StillTracked says whether git still lists the branch's worktree after a
// removal failed. `git worktree remove` drops its own entry even when it could
// not delete every file, so a failure is either nothing done or a removal done
// badly, and the two call for opposite answers.
func StillTracked(ctx context.Context, params FindByBranchParams) bool {
	_, err := FindByBranch(ctx, params)
	return !errors.Is(err, domain.ErrWorktreeNotFound)
}

// FinishRemoval completes a removal git carried out while leaving files behind:
// the branch and wtm's state go the way a clean removal takes them, so the
// worktree is not half-gone — unknown to git, yet with a branch and a namespace
// nothing will ever reclaim.
func FinishRemoval(ctx context.Context, params domain.CleanParams) error {
	if err := infra.DeleteLocalBranch(ctx, infra.DeleteLocalBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
		Force:      params.Force,
	}); err != nil {
		return fmt.Errorf("delete branch: %w", err)
	}
	purgeState(WorktreeRef{ProjectDir: params.ProjectDir, StateDir: params.StateDir, Branch: params.Branch})
	return nil
}
