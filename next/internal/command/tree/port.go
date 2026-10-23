package tree

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// Git is the repository as tree reads it, a worktree at a time, and whether a commit is behind another.
type Git interface {
	repository.Prober
	AheadBehindOf(ctx context.Context, span worktree.Span) (worktree.AheadBehind, error)
	IsAncestor(ctx context.Context, ancestry worktree.Ancestry) (bool, error)
}

// Meta reads a worktree's meta record by its branch; a missing record is an error wrapping fs.ErrNotExist.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
}

// Jobs lists the jobs the run daemon runs; it fails with run.ErrUnknown when it cannot tell which are up.
type Jobs func(ctx context.Context) ([]run.Job, error)

// PullRequests returns the pull request of each of branches, its open one else its newest whatever its state, none from a fork, an empty branch (a worktree on none) having none; it fails with worktree.ErrPullRequestsUnavailable, wrapped with what to do about it, or the context's error, keeping what it found before failing.
type PullRequests func(ctx context.Context, branches []string) ([]worktree.PullRequest, error)
