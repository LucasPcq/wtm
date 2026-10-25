package git_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/gittest"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// port is what both git.Repo and gittest.Fake implement, so one test runs
// against each.
type port interface {
	Worktrees(ctx context.Context) ([]worktree.Worktree, error)
	MainWorktree(ctx context.Context) (worktree.Worktree, error)
	CommonDir(ctx context.Context) (string, error)
	Locate(ctx context.Context, path string) (worktree.Location, error)
	Branches(ctx context.Context) (worktree.Branches, error)
	CommitDates(ctx context.Context) (map[string]time.Time, error)
	AheadBehind(ctx context.Context, name string) (worktree.AheadBehind, error)
	AheadBehindOf(ctx context.Context, span worktree.Span) (worktree.AheadBehind, error)
	IsAncestor(ctx context.Context, ancestry worktree.Ancestry) (bool, error)
	Commits(ctx context.Context, dir string, count int) ([]worktree.Commit, error)
	Dirty(ctx context.Context, path string) (bool, error)
	AddWorktree(ctx context.Context, addition worktree.Addition) error
	RemoveWorktree(ctx context.Context, path string) error
	DeleteBranch(ctx context.Context, name string) error
	Unpushed(ctx context.Context, name string, going []string) (int, error)
	Fetch(ctx context.Context) error
	FetchRef(ctx context.Context, name string) error
	FastForwardRef(ctx context.Context, name string) error
	MergeFFOnly(ctx context.Context, merge worktree.Merge) error
	Snapshot(ctx context.Context) (worktree.Snapshot, error)
}

// implementation is one port under test: its root directory and at, which
// returns the resolved path of a name under it.
type implementation struct {
	name string
	port port
	root string
	at   func(name string) string
}

// implementations returns the real repository and the fake, both seeded alike.
func implementations(t *testing.T) []implementation {
	t.Helper()
	gitRoot, fakeRoot := gittest.Symlinked(t), gittest.Symlinked(t)
	return []implementation{
		{name: "git", port: gittest.Build(t, seed(gitRoot)), root: gitRoot, at: under(t, gitRoot)},
		{name: "fake", port: gittest.NewFake(seed(fakeRoot)), root: fakeRoot, at: under(t, fakeRoot)},
	}
}

// seed is a repository under root with a worktree in every state git can
// list, local branches ahead, behind and diverged from origin, and paths taken
// or free for a new worktree.
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
			{Path: filepath.Join(root, "locked"), State: worktree.OnBranch{Branch: "locked"}, Locked: true},
			{Path: filepath.Join(root, "locked-gone"), State: worktree.OnBranch{Branch: "locked-gone"}, Locked: true},
		},
		Gone:    []string{filepath.Join(root, "gone"), filepath.Join(root, "locked-gone")},
		Folders: []string{filepath.Join(root, "taken")},
		Empty:   []string{filepath.Join(root, "empty")},
		Dirty:   []string{filepath.Join(root, "dirty")},
		Commits: map[worktree.Ref][]string{
			worktree.LocalBranch{Name: "unmerged"}: {"unmerged work"},
			worktree.LocalBranch{Name: "linked"}:   {"linked work"},
			worktree.LocalBranch{Name: "diverged"}: {"diverged here"},
		},
		Origin: map[string][]string{
			"main":     {"upstream main"},
			"feature":  {"upstream feature"},
			"diverged": {"diverged there"},
			"pushed":   {"pushed work"},
		},
	}
}

// under returns the function that joins a name to root with its symlinks
// resolved.
func under(t *testing.T, root string) func(name string) string {
	resolved := gittest.Resolved(t, root)
	return func(name string) string { return filepath.Join(resolved, name) }
}
