package prune

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as prune reads it to find what is done with: where the caller stands, whether a branch tip is behind its pull request's head, which branches track a branch origin deleted, and origin's branches fetched again. The removal's own reads and changes are teardown's.
type Git interface {
	Locate(ctx context.Context, path string) (worktree.Location, error)
	IsAncestor(ctx context.Context, ancestry worktree.Ancestry) (bool, error)
	Gone(ctx context.Context) (map[string]bool, error)
	RefreshOrigin(ctx context.Context, branches []string) error
}

// Meta reads and rewrites a worktree's meta record by its branch; Load fails with an error wrapping fs.ErrNotExist when the branch has none.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
	Update(ctx context.Context, update worktree.MetaUpdate) (domain.Written, error)
}

// Jobs lists the jobs the run daemon runs; it fails with run.ErrUnknown when it cannot tell which are up.
type Jobs func(ctx context.Context) ([]run.Job, error)

// PullRequests returns the pull request of each of branches, its open one else its newest whatever its state; it fails with worktree.ErrPullRequestsUnavailable, wrapped with what to do about it, or the context's error.
type PullRequests func(ctx context.Context, branches []string) ([]worktree.PullRequest, error)
