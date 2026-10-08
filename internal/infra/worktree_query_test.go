package infra

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func TestFindMainWorktreePath(t *testing.T) {
	dir := gittest.InitRepo(t)

	mainPath, err := FindMainWorktreePath(t.Context(), FindMainWorktreeParams{ProjectDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	resolvedDir, _ := filepath.EvalSymlinks(dir)
	resolvedMain, _ := filepath.EvalSymlinks(mainPath)

	if resolvedMain != resolvedDir {
		t.Errorf("expected main worktree path %s, got %s", resolvedDir, resolvedMain)
	}
}

func TestListWorktrees(t *testing.T) {
	dir := gittest.InitRepo(t)
	wtPath := filepath.Join(t.TempDir(), "wt1")

	_ = CreateWorktree(t.Context(), CreateWorktreeParams{
		ProjectDir: dir,
		Path:       wtPath,
		Branch:     "feature-test",
		FromBranch: "HEAD",
	})

	worktrees, err := ListWorktrees(t.Context(), ListWorktreesParams{ProjectDir: dir})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(worktrees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(worktrees))
	}

	if !worktrees[0].IsMain {
		t.Error("first worktree should be marked as main")
	}
	if worktrees[1].IsMain {
		t.Error("second worktree should not be marked as main")
	}
	if worktrees[1].Branch != "feature-test" {
		t.Errorf("expected branch feature-test, got %s", worktrees[1].Branch)
	}
}

func TestFindWorktreeByBranch(t *testing.T) {
	dir := gittest.InitRepo(t)
	wtPath := filepath.Join(t.TempDir(), "wt-find")

	_ = CreateWorktree(t.Context(), CreateWorktreeParams{
		ProjectDir: dir,
		Path:       wtPath,
		Branch:     "feat-find",
		FromBranch: "HEAD",
	})

	wt, err := FindWorktreeByBranch(t.Context(), FindWorktreeByBranchParams{
		ProjectDir: dir,
		Branch:     "feat-find",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wt.Branch != "feat-find" {
		t.Errorf("expected branch feat-find, got %s", wt.Branch)
	}
}

func TestFindWorktreeByBranchNotFound(t *testing.T) {
	dir := gittest.InitRepo(t)

	_, err := FindWorktreeByBranch(t.Context(), FindWorktreeByBranchParams{
		ProjectDir: dir,
		Branch:     "nonexistent",
	})
	if err == nil {
		t.Fatal("expected error for nonexistent branch")
	}
}
