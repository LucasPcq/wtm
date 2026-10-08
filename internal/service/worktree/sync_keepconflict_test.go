package worktree

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// rebaseInProgress reports whether the worktree at dir has a rebase stopped
// mid-way (unresolved conflict left in place).
func rebaseInProgress(t *testing.T, dir string) bool {
	t.Helper()
	return strings.Contains(strings.ToLower(git(t, dir, "status")), "rebase")
}

// conflictingSiblings sets up a repo where feat edits conflict.txt, then main
// moves the same file, so rebasing feat onto main conflicts. Returns the feat
// worktree path.
func conflictingSiblings(t *testing.T, dir, stateDir, trees string) string {
	t.Helper()
	featPath := filepath.Join(trees, "feat")

	commitFile(t, dir, "conflict.txt", "base")
	git(t, dir, "worktree", "add", "-b", "feat", featPath, "main")
	commitFile(t, featPath, "conflict.txt", "feat change")
	commitFile(t, dir, "conflict.txt", "main change")
	writeMeta(t, stateDir, "feat", "main")
	return featPath
}

// TestSyncKeepConflictLeavesRebaseInProgress verifies --keep-conflict leaves the
// conflicting rebase in the worktree and records the unmerged files.
func TestSyncKeepConflictLeavesRebaseInProgress(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	featPath := conflictingSiblings(t, dir, stateDir, t.TempDir())

	result, err := Sync(t.Context(), SyncParams{ProjectDir: dir, StateDir: stateDir, BaseBranch: "main", KeepConflict: true})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	step := result.Steps[0]
	if step.Status != domain.SyncStatusConflict {
		t.Fatalf("status = %q, want conflict", step.Status)
	}
	if !step.KeptInProgress {
		t.Fatal("KeptInProgress = false, want true")
	}
	if len(step.ConflictFiles) != 1 || step.ConflictFiles[0] != "conflict.txt" {
		t.Fatalf("ConflictFiles = %v, want [conflict.txt]", step.ConflictFiles)
	}
	if !rebaseInProgress(t, featPath) {
		t.Fatal("expected the rebase to be left in progress in the worktree")
	}
}

// TestSyncDefaultConflictAbortsButReportsFiles verifies the default mode still
// aborts (clean tree) yet now surfaces the conflicting files, captured before the
// abort.
func TestSyncDefaultConflictAbortsButReportsFiles(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	featPath := conflictingSiblings(t, dir, stateDir, t.TempDir())

	result, err := Sync(t.Context(), SyncParams{ProjectDir: dir, StateDir: stateDir, BaseBranch: "main"})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	step := result.Steps[0]
	if step.Status != domain.SyncStatusConflict {
		t.Fatalf("status = %q, want conflict", step.Status)
	}
	if step.KeptInProgress {
		t.Fatal("KeptInProgress = true, want false (default aborts)")
	}
	if len(step.ConflictFiles) != 1 || step.ConflictFiles[0] != "conflict.txt" {
		t.Fatalf("ConflictFiles = %v, want [conflict.txt]", step.ConflictFiles)
	}
	if rebaseInProgress(t, featPath) {
		t.Fatal("expected the rebase to be aborted (clean working tree)")
	}
}

// TestSyncSecondRunReportsRebaseInProgress guards the regression where a worktree
// left mid-rebase by --keep-conflict (detached HEAD) was read as an empty-branch
// node and misreported as a parent-chain cycle. The second run must succeed and
// report the paused rebase distinctly.
func TestSyncSecondRunReportsRebaseInProgress(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	featPath := conflictingSiblings(t, dir, stateDir, t.TempDir())

	if _, err := Sync(t.Context(), SyncParams{ProjectDir: dir, StateDir: stateDir, BaseBranch: "main", KeepConflict: true}); err != nil {
		t.Fatalf("first Sync: %v", err)
	}
	if !rebaseInProgress(t, featPath) {
		t.Fatal("expected the first run to leave a rebase in progress")
	}

	result, err := Sync(t.Context(), SyncParams{ProjectDir: dir, StateDir: stateDir, BaseBranch: "main"})
	if err != nil {
		t.Fatalf("second Sync errored (cycle-detection regression): %v", err)
	}
	if got := result.Steps[0].Status; got != domain.SyncStatusRebaseInProgress {
		t.Fatalf("status = %q, want rebase_in_progress", got)
	}
}

// TestSyncKeepConflictContinuesIndependentBranches verifies that with
// --keep-conflict a conflicting branch is left in progress while an independent
// sibling still syncs.
func TestSyncKeepConflictContinuesIndependentBranches(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	trees := t.TempDir()

	commitFile(t, dir, "conflict.txt", "base")

	// Both siblings branch from main BEFORE it moves. feat (created first →
	// evaluated first) edits conflict.txt; sib edits a different file.
	featPath := filepath.Join(trees, "feat")
	git(t, dir, "worktree", "add", "-b", "feat", featPath, "main")
	commitFile(t, featPath, "conflict.txt", "feat change")
	writeMeta(t, stateDir, "feat", "main")

	sibPath := filepath.Join(trees, "sib")
	git(t, dir, "worktree", "add", "-b", "sib", sibPath, "main")
	commitFile(t, sibPath, "sib.txt", "sib work")
	writeMeta(t, stateDir, "sib", "main")

	// main moves on conflict.txt → feat conflicts, sib rebases cleanly.
	commitFile(t, dir, "conflict.txt", "main change")

	result, err := Sync(t.Context(), SyncParams{ProjectDir: dir, StateDir: stateDir, BaseBranch: "main", KeepConflict: true})
	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}

	byBranch := make(map[string]domain.SyncStepResult, len(result.Steps))
	for _, s := range result.Steps {
		byBranch[s.Branch] = s
	}

	if got := byBranch["feat"].Status; got != domain.SyncStatusConflict {
		t.Fatalf("feat status = %q, want conflict", got)
	}
	if got := byBranch["sib"].Status; got != domain.SyncStatusSynced {
		t.Fatalf("sib status = %q, want synced (independent branch should keep syncing)", got)
	}
	if !rebaseInProgress(t, featPath) {
		t.Fatal("expected feat's rebase to be left in progress")
	}
}
