package runps

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// Jobs lists the jobs wtm's daemon knows on the machine, never starting it: with a *run.Skewed beside them when it is of another build; it fails with run.ErrUnknown when it cannot tell which are up.
type Jobs func(ctx context.Context) ([]run.Job, error)

// Open reads the repository holding dir: its config, run.toml and worktrees, the meta record of each, and where wtm keeps its state; it refuses a dir in no repository.
type Open func(ctx context.Context, dir string) (Project, error)

// ReadLog returns the last n lines of the log file names, oldest first, none when it does not exist.
type ReadLog func(file run.LogFile, n int) ([]run.Line, error)
