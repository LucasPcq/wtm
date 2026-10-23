package registry_test

import (
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

var (
	main  = domain.Repo{Root: "/code/app", CommonDir: "/code/app/.git"}
	other = domain.Repo{Root: "/code/other", CommonDir: "/code/other/.git"}
)

// An announced is one announcement: its event and the repository it names.
type announced struct {
	Event domain.Event
	Repo  domain.Repo
}

// A bus records each announcement.
type bus struct {
	mu   sync.Mutex
	seen []announced
}

func (b *bus) announcements() []announced {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.seen)
}

func (b *bus) announce(event domain.Event, repo domain.Repo) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.seen = append(b.seen, announced{Event: event, Repo: repo})
}

// A disk says which repositories hold a wtm config, by their common directory, and whether the state was moved out of the repositories.
type disk struct {
	configs map[string]bool
	moved   bool
}

func (d disk) isMoved() bool { return d.moved }

func (d disk) initialized(repo domain.Repo) bool { return d.configs[repo.CommonDir] }
