package clean

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as clean reads and changes it: where a path given leads and whether two paths are one directory, whether a worktree holds uncommitted changes, what removing worktrees and deleting branches together would lose, the worktrees still listed, and the removal of a worktree and of its branch.
type Git interface {
	Locate(ctx context.Context, path string) (worktree.Location, error)
	Same(ctx context.Context, a, b string) (bool, error)
	Dirty(ctx context.Context, path string) (bool, error)
	Unpushed(ctx context.Context, losing worktree.Losing) (int, error)
	Worktrees(ctx context.Context) ([]worktree.Worktree, error)
	RemoveWorktree(ctx context.Context, path string) error
	DeleteBranch(ctx context.Context, name string) error
}

// Meta reads, rewrites and deletes a worktree's meta record by its branch; Load fails with an error wrapping fs.ErrNotExist when the branch has none.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
	Update(ctx context.Context, update worktree.MetaUpdate) (domain.Written, error)
	Delete(branch string) error
}

// EnvFiles reads the lines of a worktree's .env file, none when it does not exist.
type EnvFiles interface {
	Read(path string) ([]dotenv.Line, error)
}

// Jobs lists the jobs the run daemon runs; it fails with run.ErrUnknown when it cannot tell which are up.
type Jobs func(ctx context.Context) ([]run.Job, error)

// PullRequests returns the pull request of each of branches, its open one else its newest whatever its state; it fails with worktree.ErrPullRequestsUnavailable, wrapped with what to do about it, or the context's error.
type PullRequests func(ctx context.Context, branches []string) ([]worktree.PullRequest, error)

// RunHooks runs a chain of hooks, failing with an *env.HookFailure.
type RunHooks func(ctx context.Context, chain env.HookChain) error

// Shield returns a context the first cancel does not reach, and its release.
type Shield func(ctx context.Context) (context.Context, func())
