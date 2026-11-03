package runlogs

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// Git is the repository as run logs reads it: where a path leads.
type Git interface {
	repository.Locator
}

// Daemon is the run daemon as run logs asks it, never starting it: the jobs it knows (with a *run.Skewed beside them when it is of another build), and a job's last lines, then, when the tail follows, each new one until it ends.
type Daemon interface {
	Jobs(ctx context.Context) ([]run.Job, error)
	Lines(ctx context.Context, req run.Tail, out func(run.Line)) (run.Job, error)
}

// ReadLog returns the last n lines of the log file names, oldest first, none when it does not exist.
type ReadLog func(file run.LogFile, n int) []run.Line
