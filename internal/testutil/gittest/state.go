package gittest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/infra/git"
)

type State struct {
	Worktrees []git.Worktree
	Branches  git.Branches
}

// StateOf leaves out each Head and Tip: the fake's commits are names, git's hashes.
func StateOf(t testing.TB, reader git.Reader) State {
	t.Helper()
	worktrees, err := reader.Worktrees(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	branches, err := reader.Branches(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	for i := range worktrees {
		worktrees[i].Head = ""
	}
	return State{Worktrees: worktrees, Branches: git.Branches{Local: untipped(branches.Local), Remote: untipped(branches.Remote)}}
}

func untipped(branches []git.Branch) []git.Branch {
	for i := range branches {
		branches[i].Tip = ""
	}
	return branches
}

// TempDir is a temporary directory as git writes it: macOS reaches its own
// through a symlink.
func TempDir(t testing.TB) string {
	t.Helper()
	return Resolved(t, t.TempDir())
}

func Resolved(t testing.TB, path string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	return resolved
}

// Symlinked is a fresh directory reached through a symlink, as git never
// writes it.
func Symlinked(t testing.TB) string {
	t.Helper()
	link := filepath.Join(t.TempDir(), "link")
	if err := os.Symlink(t.TempDir(), link); err != nil {
		t.Fatal(err)
	}
	return link
}

func Names(branches []git.Branch) []string {
	names := make([]string, 0, len(branches))
	for _, branch := range branches {
		names = append(names, branch.Name)
	}
	return names
}
