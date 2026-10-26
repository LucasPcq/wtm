package repository

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"

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

// Locator resolves a path to where it really is, and tells whether something is there.
type Locator interface {
	Locate(ctx context.Context, path string) (worktree.Location, error)
}

// Prober tells whether a worktree's directory is there and whether it holds uncommitted changes.
type Prober interface {
	Locator
	Dirty(ctx context.Context, path string) (bool, error)
}

// Meta reads a worktree's meta record by its branch; a missing record is an error wrapping fs.ErrNotExist.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
}

// Ancestors tells whether a commit is another or one of its ancestors.
type Ancestors interface {
	IsAncestor(ctx context.Context, ancestry worktree.Ancestry) (bool, error)
}

// MetaWriter rewrites a worktree's meta record by its branch.
type MetaWriter interface {
	Update(ctx context.Context, update worktree.MetaUpdate) (domain.Written, error)
}
