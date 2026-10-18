package list

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as list reads it, a worktree at a time, and refreshes it from origin.
type Git interface {
	Dirty(ctx context.Context, path string) (bool, error)
	Count(ctx context.Context, span worktree.Span) (int, error)
	FetchRef(ctx context.Context, name string) error
}

// Meta reads a worktree's meta record by its branch.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
}

// Jobs lists the jobs the run daemon runs; it fails with run.ErrUnknown when it cannot tell which are up.
type Jobs func(ctx context.Context) ([]run.Job, error)

// PullRequests lists the open pull requests of the repository; it fails with worktree.ErrGitHubMissing or worktree.ErrGitHubSignedOut when GitHub cannot be asked.
type PullRequests func(ctx context.Context) ([]worktree.PullRequest, error)

// Commands spells the command line that carries out action on the worktree of detail.
type Commands func(action Action, detail Detail) []string
