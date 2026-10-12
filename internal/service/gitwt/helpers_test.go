package gitwt_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/git"
)

// The few helpers the tests share. Everything a test file holds is a test.

// tempDir is where a worktree goes: macOS reaches its temp dir through a
// symlink that git resolves.
func tempDir(t *testing.T) string {
	t.Helper()
	return resolved(t, t.TempDir())
}

func resolved(t *testing.T, path string) string {
	t.Helper()
	real, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)
	return real
}

type state struct {
	worktrees []git.Worktree
	branches  git.Branches
}

// snapshot leaves out each Head: the fake has no commits.
func snapshot(t *testing.T, port port) state {
	t.Helper()
	worktrees, err := port.Worktrees(t.Context())
	require.NoError(t, err)
	branches, err := port.Branches(t.Context())
	require.NoError(t, err)
	for i := range worktrees {
		worktrees[i].Head = ""
	}
	return state{worktrees: worktrees, branches: branches}
}

// untracked leaves a file the steps after an add would: a copied .env.
func untracked(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		return
	}
	require.NoError(t, os.WriteFile(filepath.Join(path, ".env"), []byte("PORT=3000\n"), 0o644))
}
