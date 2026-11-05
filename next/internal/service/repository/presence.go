package repository

import (
	"context"
	"path/filepath"
	"slices"
	"strings"
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

// LocateParams are the worktrees of the repository, the names asked, and where the caller stands.
type LocateParams struct {
	Worktrees []worktree.Worktree
	Names     []string
	Caller    domain.Caller
}

// Locate is where each of params' names that is no worktree's name leads, as the path it stands for from where the caller stands (Resolved), whether or not something is there (a worktree removed), spelled as git spells the root of the worktree it is (Root).
func Locate(ctx context.Context, git Resolver, params LocateParams) (map[string]string, error) {
	located := map[string]string{}
	for _, name := range params.Names {
		if strings.TrimSpace(name) == "" || slices.ContainsFunc(params.Worktrees, func(w worktree.Worktree) bool { return w.Name() == name }) {
			continue
		}

		location, err := Resolved(ctx, git, name, params.Caller)
		if err != nil {
			return nil, err
		}
		if located[name], err = Root(ctx, git, params.Worktrees, location.Path); err != nil {
			return nil, err
		}
	}

	return located, nil
}

// LocateIn is Locate through git, for a service handed it as a function of the worktrees, the names asked and where the caller stands.
func LocateIn(git Resolver) func(ctx context.Context, worktrees []worktree.Worktree, names []string, caller domain.Caller) (map[string]string, error) {
	return func(ctx context.Context, worktrees []worktree.Worktree, names []string, caller domain.Caller) (map[string]string, error) {
		return Locate(ctx, git, LocateParams{Worktrees: worktrees, Names: names, Caller: caller})
	}
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

// LocatedIn is Located through git, for a service handed it as a function.
func LocatedIn(git Locator) func(ctx context.Context, name string, caller domain.Caller) (string, error) {
	return func(ctx context.Context, name string, caller domain.Caller) (string, error) {
		return Located(ctx, git, name, caller)
	}
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
