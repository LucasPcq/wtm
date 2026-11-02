package teardown

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Git is the repository as a removal reads and changes it: whether a worktree holds uncommitted changes, what removing worktrees and deleting branches together would lose, the worktrees still listed, and the removal of a worktree and of its branch.
type Git interface {
	Dirty(ctx context.Context, path string) (bool, error)
	Unpushed(ctx context.Context, losing worktree.Losing) (int, error)
	Worktrees(ctx context.Context) ([]worktree.Worktree, error)
	RemoveWorktree(ctx context.Context, path string) error
	DeleteBranch(ctx context.Context, name string) error
}

// Meta deletes a worktree's meta record by its branch.
type Meta interface {
	Delete(branch string) error
}

// EnvFiles reads the lines of a worktree's .env file, none when it does not exist.
type EnvFiles interface {
	Read(path string) ([]dotenv.Line, error)
}

// Daemon is the run daemon as a removal asks it: the jobs it runs (with a *run.Skewed beside them when it is of another build), a shared service started to take back the data a removal owes it, a job stopped, and every job of a worktree stopped, its claims let go of.
type Daemon interface {
	Jobs(ctx context.Context) ([]run.Job, error)
	Start(ctx context.Context, spec run.Spec, out func(run.Line)) (run.Job, error)
	Stop(ctx context.Context, req run.Stop) (run.Outcome, error)
	StopAll(ctx context.Context, req run.StopAll) ([]run.StoppedJob, error)
}

// Owed is the queue of the drops removed worktrees owe the shared services: read, and changed under its lock.
type Owed interface {
	Load() ([]env.OwedDrop, error)
	Update(ctx context.Context, change func([]env.OwedDrop) []env.OwedDrop) error
}
