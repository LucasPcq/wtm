package repository

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Config loads the project's config.toml with the global settings, and its run.toml; Load fails with project.ErrConfigNotFound when the repository has no wtm config.
type Config interface {
	Load() (project.Config, project.Global, error)
	LoadRun() (project.Run, error)
}

// Snapshotter reads the repository's snapshot.
type Snapshotter interface {
	Snapshot(ctx context.Context) (worktree.Snapshot, error)
}
