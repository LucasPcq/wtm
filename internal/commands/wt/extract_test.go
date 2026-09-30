package wt

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// LUC-172: extract's --to mirrors create's guard on a pre-existing target branch
// — its parent can't be inferred (it was created outside wtm), so a non-interactive
// run without --from errors naming the flag instead of silently guessing.
func TestWtExtractExistingBranchRequiresFrom(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	source := "feat/source"
	gittest.CreateBranch(t, dir, source)
	sourcePath := filepath.Join(t.TempDir(), "source")
	gitRun(t, dir, "worktree", "add", sourcePath, source)
	if err := os.WriteFile(filepath.Join(sourcePath, "file.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	target := "feat/orphan"
	gittest.CreateBranch(t, dir, target)

	_, _, err := runWtCmd(t, domain.CmdExtract, source, "--"+domain.FlagFiles, "file.txt", "--"+domain.FlagTo, target,
		"--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("extract onto an existing branch must not silently guess its parent")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagFrom) {
		t.Errorf("error %q should name the --%s flag", err, domain.FlagFrom)
	}
}

// A brand-new target branch keeps its default parent — the guard applies only to
// a branch that already exists, mirroring create.
func TestWtExtractNewBranchDoesNotRequireFrom(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	source := "feat/source2"
	gittest.CreateBranch(t, dir, source)
	sourcePath := filepath.Join(t.TempDir(), "source2")
	gitRun(t, dir, "worktree", "add", sourcePath, source)
	if err := os.WriteFile(filepath.Join(sourcePath, "file.txt"), []byte("hi\n"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	_, _, err := runWtCmd(t, domain.CmdExtract, source, "--"+domain.FlagFiles, "file.txt", "--"+domain.FlagTo, "feat/fresh",
		"--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("extract onto a new branch should default its parent: %v", err)
	}
}
