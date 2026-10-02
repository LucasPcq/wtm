// Package gittest provides helpers for creating real git repositories in tests.
package gittest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// InitRepo creates a temporary git repository with an initial empty commit.
// The repository path is returned; cleanup is handled by t.TempDir().
func InitRepo(t testing.TB) string {
	t.Helper()
	dir := t.TempDir()

	commands := [][]string{
		{"git", "init", "-b", "main"},
		{"git", "config", "user.email", "test@test.com"},
		{"git", "config", "user.name", "Test"},
		{"git", "commit", "--allow-empty", "-m", "initial"},
	}

	for _, args := range commands {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s failed: %s: %v", strings.Join(args, " "), out, err)
		}
	}

	return dir
}

func CreateBranch(t testing.TB, dir, name string) {
	t.Helper()
	Git(t, dir, "branch", name)
}

// Git runs a git command in dir, failing the test on error.
func Git(t testing.TB, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %s: %v", strings.Join(args, " "), out, err)
	}
}

// AddOrigin creates a bare repository outside dir, registers it as origin and
// pushes the current branch with an upstream. It is what makes upstream state
// (ahead/behind, and the "[gone]" prune looks for) reproducible in a test.
func AddOrigin(t testing.TB, dir string) string {
	t.Helper()
	remote := t.TempDir()

	cmd := exec.Command("git", "init", "--bare", "-b", "main")
	cmd.Dir = remote
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %s: %v", out, err)
	}

	Git(t, dir, "remote", "add", "origin", remote)
	Git(t, dir, "push", "-u", "origin", "main")
	return remote
}

// PushBranch publishes a branch and records it as its upstream, from whichever
// worktree it is checked out in.
func PushBranch(t testing.TB, dir, name string) {
	t.Helper()
	Git(t, dir, "push", "-u", "origin", name)
}

// DeleteBranchInRemote removes a branch inside the bare repository itself, not
// through a delete-push: a push would also drop the local remote-tracking ref,
// which is exactly the state a `git fetch --prune` is supposed to discover.
func DeleteBranchInRemote(t testing.TB, remote, name string) {
	t.Helper()
	Git(t, remote, "branch", "-D", name)
}

// JamWorktree breaks the worktree's link back to its repository, which git
// refuses to remove even under a double --force, leaving it registered. A lock
// no longer does: wtm lifts it under --force.
func JamWorktree(t testing.TB, path string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(path, ".git"), []byte("gitdir: /nonexistent\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}
