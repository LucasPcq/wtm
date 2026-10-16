package git_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

const remote = "origin"

type port interface {
	Worktrees(ctx context.Context) ([]worktree.Worktree, error)
	MainWorktree(ctx context.Context) (worktree.Worktree, error)
	GitCommonDir(ctx context.Context) (string, error)
	Locate(ctx context.Context, path string) (worktree.Location, error)
	Branches(ctx context.Context) (branch.Branches, error)
	AheadBehind(ctx context.Context, name string) (branch.AheadBehind, error)
	Dirty(ctx context.Context, path string) (bool, error)
	AddWorktree(ctx context.Context, addition worktree.Addition) error
	RemoveWorktree(ctx context.Context, path string) error
	DeleteBranch(ctx context.Context, name string) error
	FetchRef(ctx context.Context, name string) error
	FastForwardRef(ctx context.Context, name string) error
	MergeFFOnly(ctx context.Context, merge worktree.Merge) error
}

type implementation struct {
	name string
	port port
	root string
	at   func(name string) string
}

func implementations(t *testing.T) []implementation {
	t.Helper()
	gitRoot, fakeRoot := gittest.Symlinked(t), gittest.Symlinked(t)
	return []implementation{
		{name: "git", port: gittest.Build(t, seed(gitRoot)), root: gitRoot, at: under(t, gitRoot)},
		{name: "fake", port: gittest.NewFake(seed(fakeRoot)), root: fakeRoot, at: under(t, fakeRoot)},
	}
}

func seed(root string) gittest.FakeParams {
	return gittest.FakeParams{
		Main:   filepath.Join(root, "main"),
		Branch: "main",
		Local:  []string{"feature", "unmerged", "diverged"},
		Remote: []string{"main", "feature", "remote-only", "diverged"},
		Worktrees: []worktree.Worktree{
			{Path: filepath.Join(root, "linked"), State: worktree.OnBranch{Branch: "linked"}},
			{Path: filepath.Join(root, "dirty"), State: worktree.OnBranch{Branch: "dirty"}},
			{Path: filepath.Join(root, "gone"), State: worktree.OnBranch{Branch: "gone"}},
			{Path: filepath.Join(root, "detached"), State: worktree.Detached{}},
			{Path: filepath.Join(root, "rebasing"), State: worktree.Rebasing{Branch: "rebasing"}},
		},
		Gone:    []string{filepath.Join(root, "gone")},
		Folders: []string{filepath.Join(root, "taken")},
		Empty:   []string{filepath.Join(root, "empty")},
		Dirty:   []string{filepath.Join(root, "dirty")},
		Commits: map[branch.Ref][]string{
			branch.LocalBranch{Name: "unmerged"}: {"unmerged work"},
			branch.LocalBranch{Name: "linked"}:   {"linked work"},
			branch.LocalBranch{Name: "diverged"}: {"diverged here"},
		},
		Origin: map[string][]string{
			"main":     {"upstream main"},
			"feature":  {"upstream feature"},
			"diverged": {"diverged there"},
			"pushed":   {"pushed work"},
		},
	}
}

func under(t *testing.T, root string) func(name string) string {
	resolved := gittest.Resolved(t, root)
	return func(name string) string { return filepath.Join(resolved, name) }
}
