package git

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Snapshot reads the worktrees and the branches, then counts ahead and behind each local branch whose origin tip differs.
func (r Repo) Snapshot(ctx context.Context) (worktree.Snapshot, error) {
	worktrees, err := r.Worktrees(ctx)
	if err != nil {
		return worktree.Snapshot{}, err
	}

	branches, err := r.Branches(ctx)
	if err != nil {
		return worktree.Snapshot{}, err
	}

	divergence := map[string]worktree.AheadBehind{}
	for _, name := range branches.Differing() {
		divergence[name], err = r.AheadBehind(ctx, name)
		if err != nil {
			return worktree.Snapshot{}, err
		}
	}
	return worktree.Snapshot{Worktrees: worktrees, Branches: branches, Divergence: divergence}, nil
}
