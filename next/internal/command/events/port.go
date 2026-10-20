package events

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Bus is the event bus. Subscribe starts its daemon when none answers, then
// returns the lines published for repos, by their common directory, every
// repository's when repos is empty; the channel is closed when the bus ends the
// subscription or ctx is done.
type Bus interface {
	Subscribe(ctx context.Context, repos []string) (<-chan []byte, error)
}

// Registry is the repositories wtm knows. Register lists an initialized
// repository, announcing it on the bus as repo.added when it was not listed,
// best effort; Prune drops those no longer initialized, announcing each as
// repo.removed, and returns the others.
type Registry interface {
	Register(ctx context.Context, repo domain.Repo)
	Prune(ctx context.Context) ([]domain.Repo, error)
}

// Open opens the repository holding dir for the stream, without registering it. It fails with an error
// wrapping fs.ErrNotExist when dir is not a directory, and with a
// [*domain.Error] of code [domain.CodeNotARepository] when dir is in no
// repository.
type Open func(ctx context.Context, dir string) (Source, error)

// Meta reads a worktree's meta record by its branch.
type Meta interface {
	Load(branch string) (worktree.Meta, error)
}
