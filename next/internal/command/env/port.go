package env

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as env reads it: where a path given leads.
type Git interface {
	Locate(ctx context.Context, path string) (worktree.Location, error)
}

// Meta reads a worktree's meta record by its branch, a missing record an error wrapping fs.ErrNotExist, and rewrites it.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
	Update(ctx context.Context, update worktree.MetaUpdate) (domain.Written, error)
}

// Files reads and edits a worktree's .env files: Read gives none for a file that does not exist.
type Files interface {
	Read(path string) ([]dotenv.Line, error)
	Edit(ctx context.Context, edit dotenv.Edit) (domain.Written, error)
}

// Jobs lists the jobs wtm's daemon knows, never starting it; it fails with run.ErrUnknown when it cannot tell which are up.
type Jobs func(ctx context.Context) ([]run.Job, error)
