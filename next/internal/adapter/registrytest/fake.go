// Package registrytest is a stateful fake of the registry of the repositories wtm knows, in memory.
package registrytest

import (
	"context"
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/domain"
)

// A Fake stands in for registry.Registry, in memory: the repositories it
// lists, in the order they were added. It keeps its contract (a repository is
// listed once, by its common directory; Prune keeps the order) and records
// every call. A test seeds it with [NewFake], makes a method fail through
// FakeParams.Fail, and reads back what it lists with Repos and Calls. It is
// safe for concurrent use.
type Fake struct {
	mu    sync.Mutex
	repos []domain.Repo
	fail  map[string]error
	calls []string
}

// FakeParams is what a [Fake] starts with.
type FakeParams struct {
	Repos []domain.Repo
	Fail  map[string]error // the error a method returns, by method name
}

// NewFake returns a Fake listing a copy of params.Repos.
func NewFake(params FakeParams) *Fake {
	return &Fake{repos: slices.Clone(params.Repos), fail: params.Fail}
}

// Add lists repo as registry.Registry.Add does, and reports whether it was not listed yet.
func (f *Fake) Add(_ context.Context, repo domain.Repo) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "Add "+repo.CommonDir)
	if err := f.fail["Add"]; err != nil {
		return false, err
	}

	if slices.ContainsFunc(f.repos, func(listed domain.Repo) bool { return listed.CommonDir == repo.CommonDir }) {
		return false, nil
	}
	f.repos = append(f.repos, repo)
	return true, nil
}

// Prune drops the repositories keep refuses as registry.Registry.Prune does, and returns those it keeps and those it dropped.
func (f *Fake) Prune(_ context.Context, keep func(domain.Repo) bool) (kept, dropped []domain.Repo, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, "Prune")
	if err := f.fail["Prune"]; err != nil {
		return nil, nil, err
	}

	for _, repo := range f.repos {
		if keep(repo) {
			kept = append(kept, repo)
			continue
		}
		dropped = append(dropped, repo)
	}
	f.repos = slices.Clone(kept)
	return kept, dropped, nil
}

// Repos returns the repositories the fake lists, in order.
func (f *Fake) Repos() []domain.Repo {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.repos)
}

// Calls returns every call made so far, in order, each as its method name and, for Add, the common directory it was given ("Add /r/.git", "Prune").
func (f *Fake) Calls() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.calls)
}
