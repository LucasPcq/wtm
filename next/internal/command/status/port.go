package status

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as status reads one worktree of it: its uncommitted changes and how much they weigh, its last commits, and how its branch stands against the base branch; and a path resolved to where it really is.
type Git interface {
	Locate(ctx context.Context, path string) (worktree.Location, error)
	Changes(ctx context.Context, dir string) ([]worktree.Change, error)
	Diffstat(ctx context.Context, dir string) (worktree.Diffstat, error)
	Commits(ctx context.Context, dir string, count int) ([]worktree.Commit, error)
	AheadBehindOf(ctx context.Context, span worktree.Span) (worktree.AheadBehind, error)
}

// Meta reads a worktree's meta record by its branch; a missing record is an error wrapping os.ErrNotExist.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
}

// EnvFiles reads a .env file, none when it does not exist.
type EnvFiles interface {
	Read(path string) ([]dotenv.Line, error)
}

// Jobs lists the jobs the run daemon knows, never starting it; it fails with run.ErrUnknown when it cannot tell which are up.
type Jobs func(ctx context.Context) ([]run.Job, error)

// PullRequests lists the open pull requests of the repository, none from a fork; it fails with worktree.ErrPullRequestsUnavailable, wrapped with what to do about it.
type PullRequests func(ctx context.Context) (worktree.PullRequests, error)
