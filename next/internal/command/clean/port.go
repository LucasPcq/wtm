package clean

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as clean reads it to name a worktree: where a path given leads and whether two paths are one directory. The removal's own reads and changes are teardown's.
type Git interface {
	Locate(ctx context.Context, path string) (worktree.Location, error)
	Same(ctx context.Context, a, b string) (bool, error)
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
