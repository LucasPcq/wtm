package gitwt_test

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

type implementation struct {
	name string
	port git.ReadWriter
	root string
}

// implementations start alike, seen through a symlink: main checked out,
// feature beside it, main on the remote, a folder taken and a worktree gone.
func implementations(t *testing.T) []implementation {
	t.Helper()
	gitRoot, fakeRoot := gittest.Symlinked(t), gittest.Symlinked(t)
	return []implementation{
		{name: "git", port: gittest.Build(t, seed(gitRoot)), root: gitRoot},
		{name: "fake", port: gittest.NewFake(seed(fakeRoot)), root: fakeRoot},
	}
}

func seed(root string) gittest.FakeParams {
	return gittest.FakeParams{
		Main:      filepath.Join(root, "main"),
		Branch:    "main",
		Local:     []string{"feature"},
		Remote:    []string{"main"},
		Worktrees: []git.Worktree{{Path: filepath.Join(root, "gone"), Prunable: true, State: git.OnBranch{Branch: "gone"}}},
		Folders:   []string{filepath.Join(root, "taken")},
	}
}
