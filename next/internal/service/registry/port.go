package registry

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// Store is the list of the repositories wtm knows, keyed by their common directory. Add lists a repository and reports whether it was not listed yet; Prune drops those keep refuses and returns those it keeps and those it dropped.
type Store interface {
	Add(ctx context.Context, repo domain.Repo) (bool, error)
	Prune(ctx context.Context, keep func(domain.Repo) bool) (kept, dropped []domain.Repo, err error)
}

// Announce publishes event about repo on the event bus, best effort.
type Announce func(event domain.Event, repo domain.Repo)

// Moved reports whether wtm's state was moved out of the repositories ($WTM_STATE_DIR): where any repository keeps its own is then unknown.
type Moved func() bool

// Initialized reports whether repo's state directory holds a wtm config.
type Initialized func(repo domain.Repo) bool
