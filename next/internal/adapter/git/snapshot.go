package git

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// SnapshotSource is what ReadSnapshot reads a repository through.
type SnapshotSource interface {
	Worktrees(ctx context.Context) ([]worktree.Worktree, error)
	Branches(ctx context.Context) (worktree.Branches, error)
	AheadBehind(ctx context.Context, name string) (worktree.AheadBehind, error)
}

// Snapshot returns the repository's worktrees, its local and remote branches,
// and how far ahead and behind its origin each local branch is when their tips
// differ.
func (r Repo) Snapshot(ctx context.Context) (worktree.Snapshot, error) {
	return ReadSnapshot(ctx, r)
}

// ReadSnapshot returns the repository source reads: its worktrees, its local
// and remote branches, and how far ahead and behind its origin each local
// branch is when their tips differ.
func ReadSnapshot(ctx context.Context, source SnapshotSource) (worktree.Snapshot, error) {
	worktrees, err := source.Worktrees(ctx)
	if err != nil {
		return worktree.Snapshot{}, err
	}

	branches, err := source.Branches(ctx)
	if err != nil {
		return worktree.Snapshot{}, err
	}

	divergence := map[string]worktree.AheadBehind{}
	for _, name := range branches.Differing() {
		divergence[name], err = source.AheadBehind(ctx, name)
		if err != nil {
			return worktree.Snapshot{}, err
		}
	}
	return worktree.Snapshot{Worktrees: worktrees, Branches: branches, Divergence: divergence}, nil
}
