package configedit

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git lists the branches the base branch can be, so a base branch that is none of them is said.
type Git interface {
	Branches(ctx context.Context) (worktree.Branches, error)
}

// Editor opens the file at path in the user's editor, attached to the terminal, and returns once it is closed.
type Editor func(ctx context.Context, path string) error
