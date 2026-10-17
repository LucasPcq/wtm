package wt

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func gitWorktreeAdd(t *testing.T, repo, path, branch string) {
	t.Helper()
	cmd := exec.Command("git", "worktree", "add", path, branch)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree add %s %s: %s: %v", path, branch, out, err)
	}
}

func gitWorktreeLock(t *testing.T, repo, path string) {
	t.Helper()
	cmd := exec.Command("git", "worktree", "lock", path)
	cmd.Dir = repo
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git worktree lock %s: %s: %v", path, out, err)
	}
}

func TestRelocateForceMovesLockedWorktree(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	gittest.CreateBranch(t, dir, "feat/locked")
	lockedPath := filepath.Join(filepath.Dir(dir), "locked-wt")
	gitWorktreeAdd(t, dir, lockedPath, "feat/locked")
	gitWorktreeLock(t, dir, lockedPath)

	// Without --force the locked worktree is skipped.
	stdout, _, err := runWtCmd(t, domain.CmdRelocate, "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("wt relocate: %v", err)
	}
	var skipped domain.RelocateResult
	if jsonErr := json.Unmarshal([]byte(stdout), &skipped); jsonErr != nil {
		t.Fatalf("parse json %q: %v", stdout, jsonErr)
	}
	if skipped.Steps[0].Status != domain.RelocateStatusSkippedLocked {
		t.Fatalf("expected skipped_locked, got %q", skipped.Steps[0].Status)
	}

	// With --force it is moved (git worktree move -f -f) and adopted.
	stdout, _, err = runWtCmd(t, domain.CmdRelocate, "--force", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("wt relocate --force: %v", err)
	}
	var forced domain.RelocateResult
	if jsonErr := json.Unmarshal([]byte(stdout), &forced); jsonErr != nil {
		t.Fatalf("parse json %q: %v", stdout, jsonErr)
	}
	if forced.Steps[0].Status != domain.RelocateStatusMovedAdopted {
		t.Fatalf("expected moved_adopted with --force, got %q (detail: %s)", forced.Steps[0].Status, forced.Steps[0].Detail)
	}
	if _, statErr := os.Stat(lockedPath); !os.IsNotExist(statErr) {
		t.Errorf("expected locked worktree to be moved away, stat: %v", statErr)
	}
}

func TestRelocateBlocksOnOccupiedDestination(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	gittest.CreateBranch(t, dir, "feat/conflict")
	srcPath := filepath.Join(filepath.Dir(dir), "conflict-wt")
	gitWorktreeAdd(t, dir, srcPath, "feat/conflict")

	// Squat the target path with an unrelated directory.
	target := filepath.Join(filepath.Dir(dir), ".trees", "feat-conflict")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatalf("mkdir target: %v", err)
	}

	// Even with --force the move is blocked; the source stays put and exit is non-zero.
	_, _, err := runWtCmd(t, domain.CmdRelocate, "--force", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if !errors.Is(err, domain.ErrAborted) {
		t.Fatalf("expected ErrAborted on blocked_dest, got %v", err)
	}
	if _, statErr := os.Stat(srcPath); statErr != nil {
		t.Errorf("expected source worktree to stay put, stat: %v", statErr)
	}
}

func TestRelocateMovesAndAdoptsExternalWorktree(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	// A worktree created by hand outside base_path, with no wtm metadata.
	gittest.CreateBranch(t, dir, "feat/manual")
	manualPath := filepath.Join(filepath.Dir(dir), "manual-wt")
	gitWorktreeAdd(t, dir, manualPath, "feat/manual")

	stdout, _, err := runWtCmd(t, domain.CmdRelocate, "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("wt relocate: %v", err)
	}

	var result domain.RelocateResult
	if jsonErr := json.Unmarshal([]byte(stdout), &result); jsonErr != nil {
		t.Fatalf("parse json %q: %v", stdout, jsonErr)
	}
	if len(result.Steps) != 1 {
		t.Fatalf("expected 1 step, got %+v", result.Steps)
	}
	step := result.Steps[0]
	if step.Status != domain.RelocateStatusMovedAdopted {
		t.Fatalf("expected moved_adopted, got %q (detail: %s)", step.Status, step.Detail)
	}

	// Old path gone, new path under base_path present.
	if _, statErr := os.Stat(manualPath); !os.IsNotExist(statErr) {
		t.Errorf("expected manual path to be moved away, stat: %v", statErr)
	}
	wantPath := filepath.Join(filepath.Dir(dir), ".trees", "feat-manual")
	if _, statErr := os.Stat(wantPath); statErr != nil {
		t.Errorf("expected worktree at %q: %v", wantPath, statErr)
	}

	// Adoption wrote meta.json with the default parent (base branch).
	metaPath := filepath.Join(rules.WorktreeMetaDir(stateDir, "feat/manual"), domain.MetaFileName)
	data, readErr := os.ReadFile(metaPath)
	if readErr != nil {
		t.Fatalf("read meta.json: %v", readErr)
	}
	var meta domain.WorktreeMetadata
	if jsonErr := json.Unmarshal(data, &meta); jsonErr != nil {
		t.Fatalf("parse meta.json: %v", jsonErr)
	}
	if meta.SourceBranch != "main" {
		t.Errorf("expected adopted parent main, got %q", meta.SourceBranch)
	}
}

func TestRelocateToChangesBasePathAndUpdatesConfig(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	// A managed worktree at the current base_path.
	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/move", "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
		t.Fatalf("wt create: %v", err)
	}

	stdout, _, err := runWtCmd(t, domain.CmdRelocate, "--to", "../.worktrees", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("wt relocate --to: %v", err)
	}

	var result domain.RelocateResult
	if jsonErr := json.Unmarshal([]byte(stdout), &result); jsonErr != nil {
		t.Fatalf("parse json %q: %v", stdout, jsonErr)
	}
	if !result.BasePathUpdated {
		t.Fatalf("expected base_path to be updated, got %+v", result)
	}

	wantPath := filepath.Join(filepath.Dir(dir), ".worktrees", "feat-move")
	if _, statErr := os.Stat(wantPath); statErr != nil {
		t.Errorf("expected worktree moved to %q: %v", wantPath, statErr)
	}

	cfg, readErr := os.ReadFile(filepath.Join(stateDir, domain.ConfigFileName))
	if readErr != nil {
		t.Fatalf("read config: %v", readErr)
	}
	if want := `base_path = "../.worktrees"`; !strings.Contains(string(cfg), want) {
		t.Errorf("config not updated, want %q in:\n%s", want, cfg)
	}
}

func TestRelocateToDryRunDoesNotUpdateBasePath(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/move", "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
		t.Fatalf("wt create: %v", err)
	}

	stdout, _, err := runWtCmd(t, domain.CmdRelocate, "--to", "../.worktrees", "--dry-run", "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("wt relocate --to --dry-run: %v", err)
	}

	var result domain.RelocateResult
	if jsonErr := json.Unmarshal([]byte(stdout), &result); jsonErr != nil {
		t.Fatalf("parse json %q: %v", stdout, jsonErr)
	}
	if result.BasePathUpdated {
		t.Fatalf("dry-run must not report base_path_updated, got %+v", result)
	}

	// The config file is untouched and nothing moved.
	cfg, readErr := os.ReadFile(filepath.Join(stateDir, domain.ConfigFileName))
	if readErr != nil {
		t.Fatalf("read config: %v", readErr)
	}
	if strings.Contains(string(cfg), `base_path = "../.worktrees"`) {
		t.Errorf("dry-run rewrote base_path in config:\n%s", cfg)
	}
}

func TestRelocateRejectsInvalidTo(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	for _, to := range []string{"   ", "/abs/worktrees"} {
		_, _, err := runWtCmd(t, domain.CmdRelocate, "--to", to, "--output", domain.OutputJSON, "--"+domain.FlagYes)
		if !errors.Is(err, domain.ErrInvalidBasePath) {
			t.Fatalf("expected ErrInvalidBasePath for --to %q, got %v", to, err)
		}
	}
}
