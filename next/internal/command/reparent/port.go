package reparent

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as reparent reads it to find where the caller stands.
type Git interface {
	Locate(ctx context.Context, path string) (worktree.Location, error)
}

// Meta reads and rewrites a worktree's meta record by its branch; Load fails with an error wrapping fs.ErrNotExist when the branch has none.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
	Update(ctx context.Context, update worktree.MetaUpdate) (domain.Written, error)
}
