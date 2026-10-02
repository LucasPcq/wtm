package worktree

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/ghtest"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// cleanConfig builds a Config carrying the given on_clean hooks.
func cleanConfig(onClean []domain.HookCommand) domain.Config {
	return domain.Config{
		Project: domain.ProjectConfig{
			Hooks: domain.HooksConfig{OnClean: onClean},
		},
	}
}

func TestCleanRunsOnCleanHooksBeforeRemoval(t *testing.T) {
	source := gittest.InitRepo(t)
	stateDir := t.TempDir()

	featPath := filepath.Join(t.TempDir(), "feat")
	gitRun(t, source, "worktree", "add", "-q", "-b", "feat", featPath, "HEAD")

	marker := filepath.Join(stateDir, "CLEANED")
	params := domain.CleanParams{
		ProjectDir: source,
		StateDir:   stateDir,
		Branch:     "feat",
		Config:     cleanConfig([]domain.HookCommand{{Cmd: "touch " + marker}}),
	}

	if err := Clean(params); err != nil {
		t.Fatalf("Clean: %v", err)
	}

	if _, err := os.Stat(marker); err != nil {
		t.Errorf("on_clean hook did not run — marker missing: %v", err)
	}
	if _, err := os.Stat(featPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("worktree still present after Clean: %v", err)
	}
	if branchExists(t, source, "feat") {
		t.Error("branch feat still exists after Clean")
	}
}

func TestCleanOnCleanHookFailureAbortsRemoval(t *testing.T) {
	source := gittest.InitRepo(t)
	stateDir := t.TempDir()

	featPath := filepath.Join(t.TempDir(), "feat")
	gitRun(t, source, "worktree", "add", "-q", "-b", "feat", featPath, "HEAD")

	params := domain.CleanParams{
		ProjectDir: source,
		StateDir:   stateDir,
		Branch:     "feat",
		Config:     cleanConfig([]domain.HookCommand{{Cmd: "false"}}),
	}

	err := Clean(params)
	if err == nil {
		t.Fatal("expected Clean to fail when an on_clean hook fails")
	}
	if _, statErr := os.Stat(featPath); statErr != nil {
		t.Errorf("worktree should survive a failed on_clean hook: %v", statErr)
	}
	if !branchExists(t, source, "feat") {
		t.Error("branch should survive a failed on_clean hook")
	}
}

func TestCleanOnCleanHookContinueOnError(t *testing.T) {
	source := gittest.InitRepo(t)
	stateDir := t.TempDir()

	featPath := filepath.Join(t.TempDir(), "feat")
	gitRun(t, source, "worktree", "add", "-q", "-b", "feat", featPath, "HEAD")

	params := domain.CleanParams{
		ProjectDir: source,
		StateDir:   stateDir,
		Branch:     "feat",
		Config: cleanConfig([]domain.HookCommand{
			{Cmd: "false", ContinueOnError: true},
		}),
	}

	if err := Clean(params); err != nil {
		t.Fatalf("Clean should proceed past a continue_on_error hook: %v", err)
	}
	if _, err := os.Stat(featPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("worktree should be removed despite the tolerated hook failure: %v", err)
	}
}

// TestPruneWorktreesClearsStaleMetadata exercises the recovery path used by
// ForceClean after a manual directory deletion (the sudo rm step itself needs a
// password and is not unit-tested): once the directory is gone, `git worktree
// prune` must drop the stale administrative entry so git no longer lists it.
func TestPruneWorktreesClearsStaleMetadata(t *testing.T) {
	source := gittest.InitRepo(t)

	featPath := filepath.Join(t.TempDir(), "feat")
	gitRun(t, source, "worktree", "add", "-q", "-b", "feat", featPath, "HEAD")

	if err := os.RemoveAll(featPath); err != nil {
		t.Fatalf("remove worktree dir: %v", err)
	}

	if err := infra.PruneWorktrees(source); err != nil {
		t.Fatalf("PruneWorktrees: %v", err)
	}

	if _, err := infra.FindWorktreeByBranch(infra.FindWorktreeByBranchParams{
		ProjectDir: source,
		Branch:     "feat",
	}); !errors.Is(err, domain.ErrWorktreeNotFound) {
		t.Errorf("expected worktree to be pruned, got %v", err)
	}
}

func TestCleanPurgesWorktreeState(t *testing.T) {
	source := gittest.InitRepo(t)
	stateDir := t.TempDir()

	featPath := filepath.Join(t.TempDir(), "feat")
	gitRun(t, source, "worktree", "add", "-q", "-b", "feat/x", featPath, "HEAD")

	ordinal, err := EnsureOrdinal(WorktreeRef{ProjectDir: source, StateDir: stateDir, Branch: "feat/x"})
	if err != nil {
		t.Fatalf("EnsureOrdinal: %v", err)
	}
	metaDir := rules.WorktreeMetaDir(stateDir, "feat/x")
	if _, statErr := os.Stat(metaDir); statErr != nil {
		t.Fatalf("fixture has no meta dir: %v", statErr)
	}

	if err := Clean(domain.CleanParams{
		ProjectDir: source,
		StateDir:   stateDir,
		Branch:     "feat/x",
		Config:     cleanConfig(nil),
	}); err != nil {
		t.Fatalf("Clean: %v", err)
	}

	if _, err := os.Stat(metaDir); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("meta dir survived the clean, its ordinal %d stays reserved: %v", ordinal, err)
	}
}

func TestCheckAllChecksEachBranchOnItsOwn(t *testing.T) {
	source := gittest.InitRepo(t)
	dirtyPath := filepath.Join(t.TempDir(), "dirty")
	gitRun(t, source, "worktree", "add", "-q", "-b", "feat/dirty", dirtyPath, "HEAD")
	if err := os.WriteFile(filepath.Join(dirtyPath, "wip.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := CheckAll(CheckAllParams{ProjectDir: source, Branches: []string{"feat/dirty", "feat/ghost", "main"}})

	if entry := entries["feat/dirty"]; entry.Err != nil || !entry.Check.IsDirty || entry.Check.Branch != "feat/dirty" {
		t.Errorf("feat/dirty = %+v, want a dirty worktree", entry)
	}
	if !errors.Is(entries["feat/ghost"].Err, domain.ErrWorktreeNotFound) {
		t.Errorf("feat/ghost err = %v, want ErrWorktreeNotFound", entries["feat/ghost"].Err)
	}
	if !errors.Is(entries["main"].Err, domain.ErrCannotCleanParent) {
		t.Errorf("main err = %v, want ErrCannotCleanParent", entries["main"].Err)
	}
}

func TestCheckAllReportsALockedWorktree(t *testing.T) {
	source := gittest.InitRepo(t)
	lockedPath := filepath.Join(t.TempDir(), "locked")
	gitRun(t, source, "worktree", "add", "-q", "-b", "feat/locked", lockedPath, "HEAD")
	gitRun(t, source, "worktree", "lock", lockedPath)

	entry := CheckAll(CheckAllParams{ProjectDir: source, Branches: []string{"feat/locked"}})["feat/locked"]
	if entry.Err != nil || !entry.Check.IsLocked {
		t.Errorf("feat/locked = %+v, want a locked worktree", entry)
	}
}

func TestListMarksALockedWorktree(t *testing.T) {
	source := gittest.InitRepo(t)
	lockedPath := filepath.Join(t.TempDir(), "locked")
	gitRun(t, source, "worktree", "add", "-q", "-b", "feat/locked", lockedPath, "HEAD")
	gitRun(t, source, "worktree", "lock", lockedPath)

	statuses, err := List(domain.ListParams{ProjectDir: source, StateDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	for _, status := range statuses {
		if status.IsLocked != (status.Branch == "feat/locked") {
			t.Errorf("%s IsLocked = %v", status.Branch, status.IsLocked)
		}
	}
}

func TestCleanForcedRemovesALockedWorktree(t *testing.T) {
	source := gittest.InitRepo(t)
	lockedPath := filepath.Join(t.TempDir(), "locked")
	gitRun(t, source, "worktree", "add", "-q", "-b", "feat/locked", lockedPath, "HEAD")
	gitRun(t, source, "worktree", "lock", lockedPath)

	if err := Clean(domain.CleanParams{ProjectDir: source, StateDir: t.TempDir(), Branch: "feat/locked", Force: true}); err != nil {
		t.Fatalf("Clean --force: %v", err)
	}
	if _, err := os.Stat(lockedPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("locked worktree still present after a forced Clean: %v", err)
	}
}

// A truncated list cannot prove a branch has no open pull request, so each
// branch is then asked on its own. The stub answers every `gh pr list` with the
// same payload, which is what makes the per-branch question visible here.
func TestCheckAllAsksEachBranchWhenThePRListIsTruncated(t *testing.T) {
	cases := map[string]struct {
		prs        int
		wantOpenPR bool
	}{
		"complete list":  {prs: 199, wantOpenPR: false},
		"truncated list": {prs: 200, wantOpenPR: true},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			source := gittest.InitRepo(t)
			gitRun(t, source, "worktree", "add", "-q", "-b", "feat/x", filepath.Join(t.TempDir(), "x"), "HEAD")
			prs := make([]ghtest.PR, 0, c.prs)
			for i := 0; i < c.prs; i++ {
				prs = append(prs, ghtest.PR{Number: i + 1, Branch: fmt.Sprintf("other/%d", i), State: "open"})
			}
			ghtest.Stub(t, ghtest.StubParams{PRs: prs})

			entry := CheckAll(CheckAllParams{ProjectDir: source, Branches: []string{"feat/x"}})["feat/x"]

			if entry.Err != nil || entry.Check.HasOpenPR != c.wantOpenPR {
				t.Errorf("entry = %+v, want HasOpenPR %v", entry, c.wantOpenPR)
			}
		})
	}
}
