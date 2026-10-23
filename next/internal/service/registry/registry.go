// Package registry keeps the list of the repositories wtm knows, for a stream that follows every one of them: a repository joins it once initialized and leaves it once it no longer is, each announced on the event bus.
package registry

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// registerTimeout bounds the registration of a repository, so a command never waits on it.
const registerTimeout = 250 * time.Millisecond

// Service is the registry over its Store, announcing what joins and leaves it with Announce: a repository is listed while it is Initialized. The registry is left alone while the state directory is moved (Moved): none joins it, none leaves it, and it lists none, since a repository read with a moved state would be read with another's config and records.
type Service struct {
	Store       Store
	Announce    Announce
	Moved       Moved
	Initialized Initialized
}

// Inert reports whether the registry is left alone, the state directory moved: no repository it lists or another process announces may be read with that state.
func (s Service) Inert() bool {
	return s.Moved()
}

// Register lists repo once initialized, announcing it as repo.added when it was not listed yet, then, time left, drops the repositories no longer initialized. It never fails nor holds its caller for long: a repository missing is listed again by the next command run in it.
func (s Service) Register(ctx context.Context, repo domain.Repo) {
	if s.Inert() || !s.Initialized(repo) {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, registerTimeout)
	defer cancel()
	added, err := s.Store.Add(ctx, repo)
	if err != nil || !added {
		return
	}

	// what the store wrote is announced whatever the time left: the publisher bounds its own wait
	s.Announce(domain.RepoAdded{}, repo)
	if ctx.Err() == nil {
		_, _ = s.Prune(ctx)
	}
}

// Prune drops the repositories no longer initialized, announcing each as repo.removed, and returns the others; none while the registry is inert.
func (s Service) Prune(ctx context.Context) ([]domain.Repo, error) {
	if s.Inert() {
		return nil, nil
	}

	kept, dropped, err := s.Store.Prune(ctx, s.Initialized)
	for _, repo := range dropped {
		s.Announce(domain.RepoRemoved{}, repo)
	}
	return kept, err
}
