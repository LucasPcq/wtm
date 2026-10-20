package tree

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as tree reads it, a worktree at a time.
type Git interface {
	Locate(ctx context.Context, path string) (worktree.Location, error)
	Dirty(ctx context.Context, path string) (bool, error)
	Count(ctx context.Context, span worktree.Span) (int, error)
}

// Meta reads a worktree's meta record by its branch.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
}

// Jobs lists the jobs the run daemon runs; it fails with run.ErrUnknown when it cannot tell which are up.
type Jobs func(ctx context.Context) ([]run.Job, error)

// PullRequests returns the newest pull request of each of branches, whatever its state, none from a fork; it fails with worktree.ErrPullRequestsUnavailable, wrapped with what to do about it, keeping what it found before failing.
type PullRequests func(ctx context.Context, branches []string) ([]worktree.PullRequest, error)
