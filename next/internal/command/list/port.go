package list

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// Git is the repository as list reads it, a worktree at a time, and refreshes it from origin.
type Git interface {
	repository.Prober
	AheadBehindOf(ctx context.Context, span worktree.Span) (worktree.AheadBehind, error)
	Fetch(ctx context.Context) error
}

// Meta reads a worktree's meta record by its branch, as repository.Records reads it.
type Meta interface {
	repository.Meta
}

// Jobs lists the jobs the run daemon runs; it fails with run.ErrUnknown when it cannot tell which are up.
type Jobs func(ctx context.Context) ([]run.Job, error)

// PullRequests returns the pull request of each of branches, its open one else its newest whatever its state, none from a fork, an empty branch (a worktree on none) having none; it fails with worktree.ErrPullRequestsUnavailable, wrapped with what to do about it, or the context's error, keeping what it found before failing.
type PullRequests func(ctx context.Context, branches []string) ([]worktree.PullRequest, error)
