package events

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Jobs lists the jobs the run daemon knows, never starting it; it fails with run.ErrUnknown when it cannot tell which are up.
type Jobs func(ctx context.Context) ([]run.Job, error)

// Bus is the event bus. Subscribe starts its daemon when none answers, then
// returns the lines published for repos, by their common directory, every
// repository's when repos is empty; the channel is closed when the bus ends the
// subscription or ctx is done.
type Bus interface {
	Subscribe(ctx context.Context, repos []string) (<-chan []byte, error)
}

// Open opens the repository holding dir for the stream, without registering it. It fails with an error
// wrapping fs.ErrNotExist when dir is not a directory, and with a
// [*domain.Error] of code [domain.CodeNotARepository] when dir is in no
// repository.
type Open func(ctx context.Context, dir string) (Source, error)

// Git is what a poll reads of a repository, lighter than a snapshot: its worktrees and the tips of its branches.
type Git interface {
	Worktrees(ctx context.Context) ([]worktree.Worktree, error)
	Branches(ctx context.Context) (worktree.Branches, error)
}

// Meta reads a worktree's meta record by its branch.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
}
