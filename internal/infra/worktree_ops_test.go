package infra

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestCreateWorktreeNewBranch(t *testing.T) {
	dir := gittest.InitRepo(t)
	wtPath := filepath.Join(t.TempDir(), "my-worktree")

	err := CreateWorktree(CreateWorktreeParams{
		ProjectDir: dir,
		Path:       wtPath,
		Branch:     "feat-new",
		FromBranch: "HEAD",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}
}

func TestCreateWorktreeExistingBranch(t *testing.T) {
	dir := gittest.InitRepo(t)
	gittest.CreateBranch(t, dir, "existing-branch")
	wtPath := filepath.Join(t.TempDir(), "existing-wt")

	err := CreateWorktree(CreateWorktreeParams{
		ProjectDir:  dir,
		Path:        wtPath,
		Branch:      "existing-branch",
		ReuseBranch: true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(wtPath); os.IsNotExist(err) {
		t.Error("worktree directory was not created")
	}
}

func TestRemoveWorktree(t *testing.T) {
	dir := gittest.InitRepo(t)
	wtPath := filepath.Join(t.TempDir(), "wt-remove")

	_ = CreateWorktree(CreateWorktreeParams{
		ProjectDir: dir,
		Path:       wtPath,
		Branch:     "feat-remove",
		FromBranch: "HEAD",
	})

	err := RemoveWorktree(RemoveWorktreeParams{
		ProjectDir: dir,
		Path:       wtPath,
		Force:      false,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, statErr := os.Stat(wtPath); !os.IsNotExist(statErr) {
		t.Error("worktree directory should be removed")
	}
}

// upstreamOf returns the configured upstream of a local branch, or "" when it
// has none.
func upstreamOf(t *testing.T, dir, branch string) string {
	t.Helper()
	cmd := exec.Command("git", "for-each-ref", "--format=%(upstream:short)", "refs/heads/"+branch)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git for-each-ref %s: %v", branch, err)
	}
	return strings.TrimSpace(string(out))
}

// A child created from a parent that only exists on origin must not inherit
// origin/<parent> as its upstream: `git push` would then land on the parent's
// remote branch.
func TestCreateWorktreeFromRemoteBranchDoesNotTrackParent(t *testing.T) {
	dir := gittest.InitRepo(t)
	gittest.AddOrigin(t, dir)
	gittest.CreateBranch(t, dir, "parent")
	gittest.PushBranch(t, dir, "parent")
	gittest.Git(t, dir, "checkout", "main")
	gittest.Git(t, dir, "branch", "-D", "parent")
	// Pin git's default so the assertion does not depend on the ambient config.
	gittest.Git(t, dir, "config", "branch.autoSetupMerge", "true")

	err := CreateWorktree(CreateWorktreeParams{
		ProjectDir: dir,
		Path:       filepath.Join(t.TempDir(), "child"),
		Branch:     "child",
		FromBranch: "origin/parent",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if upstream := upstreamOf(t, dir, "child"); upstream != "" {
		t.Errorf("child branch must have no upstream, got %q", upstream)
	}
}
