package runlogs

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// Git is the repository as run logs reads it: where a path leads.
type Git interface {
	repository.Locator
}

// Daemon is wtm's daemon as run logs asks it, never starting it: the jobs it knows (with a *run.Skewed beside them when it is of another build), and a job's last lines, then, when the tail follows, each new one until it ends.
type Daemon interface {
	Jobs(ctx context.Context) ([]run.Job, error)
	Lines(ctx context.Context, req run.Tail, out func(run.Line)) (run.Job, error)
}

// ReadLog returns the last n lines of the log file names, oldest first, none when it does not exist; it fails when a file of the log exists and cannot be read, with the lines read before.
type ReadLog func(file run.LogFile, n int) ([]run.Line, error)

// ScanLog hands visit every line of the log file names, oldest first, its backups before it, until visit returns false; it fails when a file that exists cannot be read.
type ScanLog func(file run.LogFile, visit func(run.Line) bool) error

// Meta reads a worktree's meta record by its branch, failing with an error wrapping fs.ErrNotExist when it has none: its ordinal and isolation name its compose project.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
}

// EnvFiles reads the lines of a .env file, none when it does not exist: one may declare the compose project.
type EnvFiles interface {
	Read(path string) ([]dotenv.Line, error)
}
