package worktree

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// An interrupt while git adds the worktree lets the addition finish, metadata
// included: half a worktree is one no later command can read or remove cleanly.
func TestAnInterruptDuringTheAddLeavesAWholeWorktree(t *testing.T) {
	dir := gittest.InitRepo(t)
	hook := filepath.Join(dir, ".git", "hooks", "post-checkout")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nsleep 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(dir, ".git", "wtm")
	config := domain.Config{}
	config.Project.Worktrees.BasePath = filepath.Join(t.TempDir(), "trees")
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(300*time.Millisecond, cancel)

	result, err := Create(ctx, domain.CreateParams{
		ProjectDir: dir, StateDir: stateDir, Branch: "feat/x", FromBranch: "main",
		SourceBranch: "main", Config: config, SkipHooks: true,
	})

	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !StillTracked(t.Context(), FindByBranchParams{ProjectDir: dir, Branch: "feat/x"}) {
		t.Error("git does not know the worktree")
	}
	if _, statErr := os.Stat(filepath.Join(rules.WorktreeMetaDir(stateDir, "feat/x"), domain.MetaFileName)); statErr != nil {
		t.Errorf("metadata: %v", statErr)
	}
	if _, statErr := os.Stat(filepath.Join(result.Path, ".git")); statErr != nil {
		t.Errorf("checkout: %v", statErr)
	}
}

func TestAnInterruptBeforeTheAddCreatesNothing(t *testing.T) {
	dir := gittest.InitRepo(t)
	config := domain.Config{}
	config.Project.Worktrees.BasePath = filepath.Join(t.TempDir(), "trees")
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	_, err := Create(ctx, domain.CreateParams{
		ProjectDir: dir, StateDir: filepath.Join(dir, ".git", "wtm"), Branch: "feat/x", FromBranch: "main",
		SourceBranch: "main", Config: config, SkipHooks: true,
	})

	if err == nil || rules.ExitCode(err) != domain.ExitCodeCancelled {
		t.Fatalf("err = %v, want the cancellation", err)
	}
	if StillTracked(t.Context(), FindByBranchParams{ProjectDir: dir, Branch: "feat/x"}) {
		t.Error("a worktree was added after the interrupt")
	}
}
