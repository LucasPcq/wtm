package create

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as create reads it: what lies at a path a new worktree would take.
type Git interface {
	Locate(ctx context.Context, path string) (worktree.Location, error)
}

// Meta reads a worktree's meta record by its branch.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
}
