package configset

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git lists the branches the base branch can be.
type Git interface {
	Branches(ctx context.Context) (worktree.Branches, error)
}

// Detect is what the repository's files suggest of its settings: its .env files, its hooks and the folders a hook can run in.
type Detect func() project.Detected
