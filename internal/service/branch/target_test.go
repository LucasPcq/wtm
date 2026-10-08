package branch

import (
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestTargetNewBranch(t *testing.T) {
	dir := gittest.InitRepo(t)

	got := Target(t.Context(), BranchParams{ProjectDir: dir, Branch: "feat/absent"})
	if got.State != domain.BranchTargetNew {
		t.Errorf("State = %v, want BranchTargetNew", got.State)
	}
	if got.Branch != "feat/absent" {
		t.Errorf("Branch = %q, want %q", got.Branch, "feat/absent")
	}
	if got.WorktreePath != "" {
		t.Errorf("WorktreePath = %q, want empty", got.WorktreePath)
	}
}

func TestTargetExistingBranchWithoutWorktree(t *testing.T) {
	dir := gittest.InitRepo(t)
	gittest.CreateBranch(t, dir, "feat/existing")

	got := Target(t.Context(), BranchParams{ProjectDir: dir, Branch: "feat/existing"})
	if got.State != domain.BranchTargetExisting {
		t.Errorf("State = %v, want BranchTargetExisting", got.State)
	}
	if got.WorktreePath != "" {
		t.Errorf("WorktreePath = %q, want empty for a free branch", got.WorktreePath)
	}
}

func TestTargetBranchCheckedOutInAnotherWorktree(t *testing.T) {
	dir := gittest.InitRepo(t)
	gittest.CreateBranch(t, dir, "feat/taken")
	wtPath := filepath.Join(t.TempDir(), "taken")
	addWorktree(t, dir, wtPath, "feat/taken")

	got := Target(t.Context(), BranchParams{ProjectDir: dir, Branch: "feat/taken"})
	if got.State != domain.BranchTargetCheckedOut {
		t.Fatalf("State = %v, want BranchTargetCheckedOut", got.State)
	}
	if got.WorktreePath == "" {
		t.Error("WorktreePath should name the worktree holding the branch")
	}
}

// The main worktree is listed by `git worktree list` too, so the branch checked
// out in the repo itself must classify as taken rather than free.
func TestTargetBranchCheckedOutInMainWorktree(t *testing.T) {
	dir := gittest.InitRepo(t)

	got := Target(t.Context(), BranchParams{ProjectDir: dir, Branch: "main"})
	if got.State != domain.BranchTargetCheckedOut {
		t.Errorf("State = %v, want BranchTargetCheckedOut for the checked-out main branch", got.State)
	}
}

func addWorktree(t *testing.T, dir, path, branch string) {
	t.Helper()
	cmd := exec.Command("git", "worktree", "add", path, branch)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add: %s: %v", out, err)
	}
}
