package extract

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as extract reads and changes it: its snapshot, what lies at a path, a worktree's uncommitted changes, the conflicts they would meet in a target, and the move itself.
type Git interface {
	Snapshot(ctx context.Context) (worktree.Snapshot, error)
	Locate(ctx context.Context, path string) (worktree.Location, error)
	Changes(ctx context.Context, dir string) ([]worktree.Change, error)
	Conflicts(ctx context.Context, check worktree.ConflictCheck) ([]worktree.Conflict, error)
	Move(ctx context.Context, move worktree.Move) (worktree.Moved, error)
}

// Config loads the project's config.toml with the global settings, and its run.toml; Load fails with project.ErrConfigNotFound when the repository has no wtm config.
type Config interface {
	Load() (project.Config, project.Global, error)
	LoadRun() (project.Run, error)
}

// Answers keeps the answers a user asked to remember, keyed by their id in config.toml.
type Answers interface {
	Remember(ctx context.Context, answers map[string]string) (domain.Written, error)
}

// Meta reads a worktree's meta record by its branch; Load fails with an error wrapping fs.ErrNotExist when the branch has none.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
}

// A Shield returns a context that ignores the cancellation of ctx, for a section that must run to its end, and the release that ends the section.
type Shield func(ctx context.Context) (context.Context, func())
