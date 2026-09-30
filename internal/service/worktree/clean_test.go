package worktree

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
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
