package rundown

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as run down reads it: where a path leads, and whether two lead to the same file.
type Git interface {
	Locate(ctx context.Context, path string) (worktree.Location, error)
	Same(ctx context.Context, a, b string) (bool, error)
}

// Daemon is the run daemon as run down asks it: the jobs it runs (with a *run.Skewed beside them when it is of another build, run.ErrUnknown when none answers and its index holds a process), one of this build made sure of, and a job stopped, a daemon woken for it when its index holds the worktree's jobs.
type Daemon interface {
	Jobs(ctx context.Context) ([]run.Job, error)
	Ensure(ctx context.Context) error
	Stop(ctx context.Context, req run.Stop) (run.Outcome, error)
}
