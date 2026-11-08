package daemonrestart

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// Shield returns a context the first cancel does not reach, and its release: the second signal waits for the release.
type Shield func(ctx context.Context) (context.Context, func())

// Exists reports whether the directory at path is there: a job whose worktree was removed does not come back.
type Exists func(path string) bool

// Daemon is wtm's daemon as a restart reaches it: how it stands, never starting it; the records of its index, which say how each job up was started; its shutdown, which returns once it is gone; a daemon of this build made sure of; and a job started from its spec.
type Daemon interface {
	Status(ctx context.Context) (run.Daemon, error)
	Records() []run.Record
	Shutdown(ctx context.Context) error
	Ensure(ctx context.Context) error
	Start(ctx context.Context, spec run.Spec, out func(run.Line)) (run.Job, error)
}
