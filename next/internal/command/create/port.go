package create

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as create reads it: its snapshot, and what lies at a path a new worktree would take.
type Git interface {
	Locate(ctx context.Context, path string) (worktree.Location, error)
}
