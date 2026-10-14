package git_test

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

type port interface {
	git.Reader
	git.Writer
}

type implementation struct {
	name string
	port port
	root string
	at   func(name string) string
}

func implementations(t *testing.T) []implementation {
	t.Helper()
	// Each root is a symlink git resolves: at names a path as git writes it.
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
		Worktrees: []git.Worktree{
			{Path: filepath.Join(root, "linked"), State: git.OnBranch{Branch: "linked"}},
			{Path: filepath.Join(root, "dirty"), State: git.OnBranch{Branch: "dirty"}},
			{Path: filepath.Join(root, "locked"), Locked: true, State: git.OnBranch{Branch: "locked"}},
			{Path: filepath.Join(root, "gone"), Prunable: true, State: git.OnBranch{Branch: "gone"}},
			{Path: filepath.Join(root, "detached"), State: git.Detached{}},
			{Path: filepath.Join(root, "rebasing"), State: git.Rebasing{Branch: "rebasing"}},
		},
		Folders: []string{filepath.Join(root, "taken")},
		Dirty:   []string{filepath.Join(root, "dirty")},
		Commits: map[git.Ref][]string{
			git.LocalBranch{Name: "unmerged"}: {"unmerged work"},
			git.LocalBranch{Name: "linked"}:   {"linked work"},
			git.LocalBranch{Name: "diverged"}: {"diverged here"},
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
