package repository

import (
	"context"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// readers bounds the worktrees read at once, each costing a few git commands.
const readers = 8

// A Presence is where a worktree stands on disk: whether its directory is gone, and whether it holds uncommitted changes.
type Presence struct {
	Missing bool
	Dirty   bool
}

// Present reads the presence of the worktree w through git: missing when git calls it prunable or its directory is gone, else whether it is dirty. It fails with what kept git from telling.
func Present(ctx context.Context, git Prober, w worktree.Worktree) (Presence, error) {
	// git never lists a locked worktree prunable, even with its directory gone: the disk is asked.
	if w.Prunable {
		return Presence{Missing: true}, nil
	}
	location, err := git.Locate(ctx, w.Path)
	if err != nil {
		return Presence{}, err
	}
	if !location.Exists {
		return Presence{Missing: true}, nil
	}

	dirty, err := git.Dirty(ctx, w.Path)
	return Presence{Dirty: dirty}, err
}

// Each returns what read reads of each of worktrees, in their order, a few read at once.
func Each[S any](worktrees []worktree.Worktree, read func(worktree.Worktree) S) []S {
	states := make([]S, len(worktrees))
	slots := make(chan struct{}, readers)
	var wg sync.WaitGroup
	for i, w := range worktrees {
		slots <- struct{}{}
		wg.Go(func() {
			defer func() { <-slots }()
			states[i] = read(w)
		})
	}
	wg.Wait()

	return states
}
