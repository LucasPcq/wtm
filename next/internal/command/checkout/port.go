package checkout

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as checkout reads it: origin's copy of the pull request's branch, fetched; the branches and worktrees once it is; and what lies at the path a new worktree would take.
type Git interface {
	FetchRef(ctx context.Context, name string) error
	Snapshot(ctx context.Context) (worktree.Snapshot, error)
	Locate(ctx context.Context, path string) (worktree.Location, error)
}

// PullRequest returns the pull request number, whatever its state, with the branch it would merge and the branch it would merge into; it fails with worktree.ErrPullRequestNotFound when the repository has none of that number, else with worktree.ErrPullRequestsUnavailable wrapped with what to do about it, or the context's error.
type PullRequest func(ctx context.Context, number int) (worktree.PullRequest, error)

// PullRequests returns the open pull requests, narrowed by filter, a fork's marked; it fails with worktree.ErrPullRequestsUnavailable wrapped with what to do about it, or the context's error.
type PullRequests func(ctx context.Context, filter worktree.PullRequestFilter) ([]worktree.PullRequest, error)
