package git

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Snapshot returns the repository's worktrees, its local and remote branches,
// and how far ahead and behind its origin each local branch is when their tips
// differ.
func (r Repo) Snapshot(ctx context.Context) (worktree.Snapshot, error) {
	return worktree.ReadSnapshot(ctx, r)
}
