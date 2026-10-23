// Package registry keeps the list of the repositories wtm knows, for a stream that follows every one of them: a repository joins it once initialized and leaves it once it no longer is, each announced on the event bus.
package registry

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// registerTimeout bounds the registration of a repository, so a command never waits on it.
const registerTimeout = 250 * time.Millisecond

// Service is the registry over its Store, announcing what joins and leaves it with Announce: a repository is listed while its state directory holds a wtm config. The registry is left alone while the state directory is moved ($WTM_STATE_DIR): none joins it, none leaves it.
type Service struct {
	Store       Store
	Announce    Announce
	StateDir    StateDir
	Initialized Initialized
}

// Register lists repo once initialized, announcing it as repo.added when it was not listed yet, then drops the repositories no longer initialized. It never fails nor holds its caller for long: a repository missing is listed again by the next command run in it.
func (s Service) Register(ctx context.Context, repo domain.Repo) {
	dir, moved := s.StateDir(repo.CommonDir)
	if moved || !s.Initialized(dir) {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, registerTimeout)
	defer cancel()
	added, err := s.Store.Add(ctx, repo)
	if err != nil || !added {
		return
	}

	s.Announce(domain.RepoAdded{}, repo)
	_, _ = s.Prune(ctx)
}

// Prune drops the repositories no longer initialized, announcing each as repo.removed, and returns the others.
func (s Service) Prune(ctx context.Context) ([]domain.Repo, error) {
	kept, dropped, err := s.Store.Prune(ctx, func(repo domain.Repo) bool {
		dir, moved := s.StateDir(repo.CommonDir)
		return moved || s.Initialized(dir)
	})
	for _, repo := range dropped {
		s.Announce(domain.RepoRemoved{}, repo)
	}
	return kept, err
}
