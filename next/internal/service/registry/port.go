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

// StateDir returns where the repository of commonDir keeps wtm's state, and whether it was moved out of that repository ($WTM_STATE_DIR), which leaves the registry unable to tell where any repository keeps its own.
type StateDir func(commonDir string) (dir string, moved bool)

// Initialized reports whether the state directory dir holds a wtm config.
type Initialized func(dir string) bool
