package runlist

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// Git is the repository as run list reads it: where a path leads.
type Git interface {
	repository.Locator
}

// Meta reads a worktree's meta record by its branch, as repository.Records reads it: its ordinal sets its ports.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
}

// Jobs lists the jobs wtm's daemon knows, never starting it; it fails with run.ErrUnknown when it cannot tell which are up.
type Jobs func(ctx context.Context) ([]run.Job, error)

// ReadLog returns the last n lines of the log file names, oldest first, none when it does not exist.
type ReadLog func(file run.LogFile, n int) ([]run.Line, error)
