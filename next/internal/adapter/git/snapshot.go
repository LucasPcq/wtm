package git

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Snapshot returns the repository's worktrees, its local and remote branches,
// and how far ahead and behind its origin each local branch is when their tips
// differ.
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
