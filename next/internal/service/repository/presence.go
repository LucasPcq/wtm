package repository

import (
	"context"
	"path/filepath"
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/domain"
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
	gone, err := Gone(ctx, git, w)
	if err != nil || gone {
		return Presence{Missing: gone}, err
	}

	dirty, err := git.Dirty(ctx, w.Path)
	return Presence{Dirty: dirty}, err
}

// Gone reports whether the directory of the worktree w is gone: git calls it prunable, or nothing is there. It fails with what kept git from telling.
func Gone(ctx context.Context, git Locator, w worktree.Worktree) (bool, error) {
	// git never lists a locked worktree prunable, even with its directory gone: the disk is asked.
	if w.Prunable {
		return true, nil
	}
	location, err := git.Locate(ctx, w.Path)
	if err != nil {
		return false, err
	}

	return !location.Exists, nil
}

// GoneIn is Gone through git, for a service handed it as a function.
func GoneIn(git Locator) func(ctx context.Context, w worktree.Worktree) (bool, error) {
	return func(ctx context.Context, w worktree.Worktree) (bool, error) { return Gone(ctx, git, w) }
}

// Located is the path a worktree's name stands for, read from where caller stands and resolved through its symbolic links by git; "" when the name is empty or nothing exists there, the name then being a branch or a part of one (worktree.Named).
func Located(ctx context.Context, git Locator, name string, caller domain.Caller) (string, error) {
	location, err := Resolved(ctx, git, name, caller)
	if err != nil || !location.Exists {
		return "", err
	}
	return location.Path, nil
}

// Resolved is where the path name leads from where caller stands, resolved through its symbolic links by git, whether or not something is there (a worktree removed whose directory is gone); none for an empty name.
func Resolved(ctx context.Context, git Locator, name string, caller domain.Caller) (worktree.Location, error) {
	if name == "" {
		return worktree.Location{}, nil
	}

	path := name
	if !filepath.IsAbs(path) && caller.Cwd != "" {
		path = filepath.Join(caller.Cwd, path)
	}
	return git.Locate(ctx, path)
}

// Root is path as git spells the root of the worktree among worktrees it is, when it is one under another spelling (a disk that ignores case: git keeps a path as it was given), else path itself; only a path that matches no root as written is asked of the disk.
func Root(ctx context.Context, git Matcher, worktrees []worktree.Worktree, path string) (string, error) {
	if slices.ContainsFunc(worktrees, func(w worktree.Worktree) bool { return w.Path == path }) {
		return path, nil
	}
	for _, w := range worktrees {
		if w.Path == "" || w.Prunable {
			continue
		}
		same, err := git.Same(ctx, path, w.Path)
		if err != nil {
			return "", err
		}
		if same {
			return w.Path, nil
		}
	}

	return path, nil
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
