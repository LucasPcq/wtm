package worktree

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// An interrupt during a rebase aborts it — the branch is where it was, no
// rebase is left in progress — and the cascade below is reported as never
// reached, not as failed.
func TestAnInterruptedSyncAbortsTheRebaseAndNamesWhatItCutShort(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	trees := t.TempDir()
	featPath := filepath.Join(trees, "feat")
	dev1Path := filepath.Join(trees, "dev1")
	git(t, dir, "worktree", "add", "-b", "feat", featPath, "main")
	commitFile(t, featPath, "feat.txt", "feat work")
	git(t, dir, "worktree", "add", "-b", "dev1", dev1Path, "feat")
	commitFile(t, dev1Path, "dev1.txt", "dev1 work")
	commitFile(t, dir, "main.txt", "main moved")
	writeMeta(t, stateDir, "feat", "main")
	writeMeta(t, stateDir, "dev1", "feat")
	featTip := git(t, dir, "rev-parse", "feat")
	hook := filepath.Join(dir, ".git", "hooks", "post-checkout")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\nsleep 5\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(500*time.Millisecond, cancel)

	result, err := Sync(ctx, SyncParams{ProjectDir: dir, StateDir: stateDir, BaseBranch: "main"})

	if err != nil {
		t.Fatalf("Sync error: %v", err)
	}
	if len(result.Steps) != 2 || result.Steps[0].Status != domain.SyncStatusCancelled || result.Steps[1].Status != domain.SyncStatusCancelled {
		t.Fatalf("steps = %+v, want both cancelled", result.Steps)
	}
	if got := git(t, dir, "rev-parse", "feat"); got != featTip {
		t.Errorf("feat moved to %s, want it left at %s", got, featTip)
	}
	if out := git(t, featPath, "status"); filepath.Base(git(t, featPath, "rev-parse", "--abbrev-ref", "HEAD")) != "feat" {
		t.Errorf("feat's worktree is left mid-rebase:\n%s", out)
	}
}
