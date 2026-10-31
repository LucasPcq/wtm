package sync

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as sync reads and rebases it: where a path given leads, origin's branches fetched, a worktree's uncommitted changes, a branch against its parent, the commits of origin it never held and where it forked from its parent, then the rebase, the push, and the move of a branch no worktree holds.
type Git interface {
	Locate(ctx context.Context, path string) (worktree.Location, error)
	Fetch(ctx context.Context) error
	Changes(ctx context.Context, dir string) ([]worktree.Change, error)
	AheadBehindOf(ctx context.Context, span worktree.Span) (worktree.AheadBehind, error)
	NeverHeld(ctx context.Context, name string) (int, error)
	ForkPoint(ctx context.Context, span worktree.Span) (worktree.Fork, error)
	Rebase(ctx context.Context, rebase worktree.Rebase) (worktree.Rebased, error)
	Push(ctx context.Context, push worktree.Push) error
	IsAncestor(ctx context.Context, ancestry worktree.Ancestry) (bool, error)
	UpdateBranch(ctx context.Context, update worktree.BranchUpdate) error
}

// Meta reads a worktree's meta record by its branch, which names its parent; a missing record is an error wrapping fs.ErrNotExist.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
}

// Shield returns a context the first cancel does not reach, and its release.
type Shield func(ctx context.Context) (context.Context, func())
