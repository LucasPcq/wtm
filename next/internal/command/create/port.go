package create

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as create reads it: its snapshot, and what lies at a path a new worktree would take.
type Git interface {
	Snapshot(ctx context.Context) (worktree.Snapshot, error)
	Locate(ctx context.Context, path string) (worktree.Location, error)
}

// Config loads the project's config.toml with the global settings, and its run.toml; Load fails with project.ErrConfigNotFound when the repository has no wtm config.
type Config interface {
	Load() (project.Config, project.Global, error)
	LoadRun() (project.Run, error)
}
