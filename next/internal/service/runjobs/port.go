package runjobs

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Meta reads a worktree's meta record by its branch, failing with an error wrapping fs.ErrNotExist when it has none, and changes it: a start records the namespaces of shared services a worktree made.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
	Update(ctx context.Context, update worktree.MetaUpdate) (domain.Written, error)
}

// EnvFiles reads the lines of a worktree's .env file, none when it does not exist.
type EnvFiles interface {
	Read(path string) ([]dotenv.Line, error)
}

// Daemon is the run daemon as a start asks it: the jobs it runs (with a *run.Skewed beside them when it is of another build), one of this build made sure of, a job started, the jobs of a worktree stopped, and a job stopped before its restart.
type Daemon interface {
	Jobs(ctx context.Context) ([]run.Job, error)
	Ensure(ctx context.Context) error
	Start(ctx context.Context, spec run.Spec, out func(run.Line)) (run.Job, error)
	StopAll(ctx context.Context, req run.StopAll) ([]run.StoppedJob, error)
	Stop(ctx context.Context, req run.Stop) (run.Outcome, error)
}

// Lister is the run daemon as a read asks it: the jobs it runs, with a *run.Skewed beside them when it is of another build, run.ErrUnknown when none answers and its index holds a process.
type Lister interface {
	Jobs(ctx context.Context) ([]run.Job, error)
}

// Stopper is the run daemon as a stop asks it: a job stopped, a daemon woken for it when its index holds the worktree's jobs.
type Stopper interface {
	Stop(ctx context.Context, req run.Stop) (run.Outcome, error)
}

// Listening dials each of ports on the loopback until something answers on every one, within, or ctx ends, and reports which answered.
type Listening func(ctx context.Context, ports []int, within time.Duration) map[int]bool
