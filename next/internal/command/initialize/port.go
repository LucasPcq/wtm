package initialize

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git reads the repository's worktrees and branches, and the branch origin names its default.
type Git interface {
	Snapshot(ctx context.Context) (worktree.Snapshot, error)
	OriginHead(ctx context.Context) (string, error)
}

// Detect reads what the repository's files suggest of its settings.
type Detect func() project.Detected

// RunConfig reads the repository's run.toml.
type RunConfig interface {
	LoadRun() (project.Run, error)
}

// Register lists the repository among those wtm knows, once its config is written; it never fails.
type Register func(ctx context.Context)
