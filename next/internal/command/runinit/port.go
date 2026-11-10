package runinit

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// RunConfig reads run.toml as written, with its digest, and writes it whole, refusing one changed since (project.ErrRunChanged).
type RunConfig interface {
	SavedRun() (project.SavedRun, error)
	SaveRun(ctx context.Context, rewrite project.RunRewrite) (domain.Written, error)
}

// Settings reads config.toml as written, with its digest, and writes it whole, refusing one changed since (project.ErrConfigChanged).
type Settings interface {
	Saved() (project.Saved, error)
	Save(ctx context.Context, rewrite project.Rewrite) (domain.Written, error)
}

// Compose reads the repository's docker compose file, with its digest, and rewrites values of it in place, refusing one changed since (project.ErrComposeChanged).
type Compose interface {
	Compose() (project.Compose, error)
	SaveCompose(ctx context.Context, rewrite project.ComposeRewrite) (domain.Written, error)
}

// Detector reads what the repository's files say: its package manifests and their scripts, and its .env files.
type Detector interface {
	Workspace() project.Workspace
	Detect() project.Detected
}

// EnvFiles reads the lines of an .env file, by its path.
type EnvFiles interface {
	Read(path string) ([]dotenv.Line, error)
}

// Meta reads a worktree's meta record by its branch: run init previews a worktree at the ordinal it records.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
}
