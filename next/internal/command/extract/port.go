package extract

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as extract reads and changes it: its snapshot, what lies at a path, a worktree's uncommitted changes, the conflicts they would meet in a target, and the move itself.
type Git interface {
	Locate(ctx context.Context, path string) (worktree.Location, error)
	Changes(ctx context.Context, dir string) ([]worktree.Change, error)
	Conflicts(ctx context.Context, check worktree.ConflictCheck) ([]worktree.Conflict, error)
	Move(ctx context.Context, move worktree.Move) (worktree.Moved, error)
}

// Meta reads a worktree's meta record by its branch; Load fails with an error wrapping fs.ErrNotExist when the branch has none.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
}

// A Shield returns a context that ignores the cancellation of ctx, for a section that must run to its end, and the release that ends the section.
type Shield func(ctx context.Context) (context.Context, func())
