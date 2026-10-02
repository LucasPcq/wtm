package wt

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

func runWtCmd(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	root := &cobra.Command{Use: domain.AppName}
	root.AddGroup(
		&cobra.Group{ID: domain.CmdGroupWorktrees, Title: domain.CmdGroupWorktreesTitle},
		&cobra.Group{ID: domain.CmdGroupNavigate, Title: domain.CmdGroupNavigateTitle},
		&cobra.Group{ID: domain.CmdGroupStack, Title: domain.CmdGroupStackTitle},
	)
	for _, c := range NewCmds() {
		root.AddCommand(c)
	}
	var outBuf, errBuf bytes.Buffer
	root.SetOut(&outBuf)
	root.SetErr(&errBuf)
	root.SetArgs(args)
	err = root.Execute()
	return outBuf.String(), errBuf.String(), err
}

func TestWtCreateAndClean(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	branch := "feat/wt-e2e-test"

	_, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("wt create: %v", err)
	}

	wtPath := filepath.Join(filepath.Dir(dir), ".trees", branch)
	if _, err := os.Stat(wtPath); err != nil {
		wtPath = filepath.Join(filepath.Dir(dir), ".trees", "feat-wt-e2e-test")
		if _, err2 := os.Stat(wtPath); err2 != nil {
			t.Fatalf("worktree dir not found at expected path: %v / %v", err, err2)
		}
	}

	_, _, err = runWtCmd(t, domain.CmdClean, branch, "--yes", "--force", "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("wt clean: %v", err)
	}

	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("expected worktree dir to be removed, stat returned: %v", err)
	}
}

// TestCleanAxesStrictlySeparated verifies the LUC-119 bypass taxonomy for clean:
// --force is the safety axis, --yes is the confirmation axis. In JSON mode the
// confirmation cannot run, so --yes is required; --force alone is rejected. On an
// unsafe (dirty) worktree, --yes still refuses (safety kept) and directs to --force,
// while --yes --force removes it.
func TestCleanAxesStrictlySeparated(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")
	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	branch := "feat/axes"
	if _, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
		t.Fatalf("wt create: %v", err)
	}
	wtPath := resolveWorktreePath(t, dir, branch)

	// --force alone no longer satisfies JSON mode: it is the safety axis, not a
	// confirmation bypass. The guard must ask for --yes.
	if _, _, err := runWtCmd(t, domain.CmdClean, branch, "--force", "--output", domain.OutputJSON); err == nil {
		t.Fatal("expected clean --force (no --yes) in JSON mode to be rejected")
	} else if !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("expected the error to direct to --yes, got: %v", err)
	}

	// Make the worktree dirty so the safety check would refuse it.
	if err := os.WriteFile(filepath.Join(wtPath, "dirty.txt"), []byte("wip\n"), 0o644); err != nil {
		t.Fatalf("dirty the worktree: %v", err)
	}

	// --yes keeps the safety check: a dirty worktree is refused, pointing at --force.
	if _, _, err := runWtCmd(t, domain.CmdClean, branch, "--yes", "--output", domain.OutputJSON); err == nil {
		t.Fatal("expected clean --yes on a dirty worktree to be refused")
	} else if !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected the refusal to direct to --force, got: %v", err)
	}
	if _, err := os.Stat(wtPath); err != nil {
		t.Fatalf("worktree must still exist after a refused clean: %v", err)
	}

	// --yes --force lifts safety and removes the dirty worktree.
	if _, _, err := runWtCmd(t, domain.CmdClean, branch, "--yes", "--force", "--output", domain.OutputJSON); err != nil {
		t.Fatalf("clean --yes --force on a dirty worktree: %v", err)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("expected the dirty worktree to be removed, stat: %v", err)
	}

	// The other direction of the same separation: --force is never *required* on the
	// confirmation axis. A safe worktree goes away with --yes alone.
	safe := "feat/axes-safe"
	if _, _, err := runWtCmd(t, domain.CmdCreate, safe, "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
		t.Fatalf("wt create: %v", err)
	}
	safePath := resolveWorktreePath(t, dir, safe)
	if _, _, err := runWtCmd(t, domain.CmdClean, safe, "--yes", "--output", domain.OutputJSON); err != nil {
		t.Fatalf("clean --yes on a safe worktree: %v", err)
	}
	if _, err := os.Stat(safePath); !os.IsNotExist(err) {
		t.Errorf("expected the safe worktree to be removed with --yes alone, stat: %v", err)
	}
}

// TestCleanYesRefusesUnpushedCommitsUntilForce covers the second safety dimension
// (the first being a dirty tree): unpushed commits are lost with the branch, so
// --yes keeps refusing until the safety axis is lifted explicitly.
func TestCleanYesRefusesUnpushedCommitsUntilForce(t *testing.T) {
	work := repoWithRemote(t)
	prepareRepo(t, work)

	branch := "feat/unpushed"
	if _, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
		t.Fatalf("wt create: %v", err)
	}
	wtPath := resolveWorktreePath(t, work, branch)
	// origin/<branch> must exist for the unpushed count to be measurable.
	gitRun(t, wtPath, "push", "-u", "origin", branch)
	gitRun(t, wtPath, "commit", "--allow-empty", "-m", "local work")

	_, _, err := runWtCmd(t, domain.CmdClean, branch, "--yes", "--output", domain.OutputJSON)
	if err == nil {
		t.Fatal("expected clean --yes to refuse a branch with unpushed commits")
	}
	if !strings.Contains(err.Error(), "--force") {
		t.Fatalf("expected the refusal to direct to --force, got: %v", err)
	}
	if !strings.Contains(err.Error(), "unpushed") {
		t.Errorf("expected the refusal to name the reason, got: %v", err)
	}
	if _, statErr := os.Stat(wtPath); statErr != nil {
		t.Fatalf("worktree must still exist after a refused clean: %v", statErr)
	}

	if _, _, err := runWtCmd(t, domain.CmdClean, branch, "--yes", "--force", "--output", domain.OutputJSON); err != nil {
		t.Fatalf("clean --yes --force with unpushed commits: %v", err)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("expected the worktree to be removed, stat: %v", err)
	}
}

// TestCleanYesReportsTheReparentDecisionInJSON is the "safe default" clause of the
// confirmation axis, seen from the output contract: --yes resolves the reparent
// decision without prompting and leaves the children alone, naming them in
// orphaned_children; --reparent-children moves them into reparented. The metadata
// side of the same decision is covered by TestCleanReparentsChildrenWithFlag and
// TestCleanLeavesChildrenOrphanedWithoutFlag.
func TestCleanYesReportsTheReparentDecisionInJSON(t *testing.T) {
	createRepo(t)

	buildStack := func(t *testing.T, parent, child string) {
		t.Helper()
		if _, _, err := runWtCmd(t, domain.CmdCreate, parent, "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
			t.Fatalf("wt create %s: %v", parent, err)
		}
		if _, _, err := runWtCmd(t, domain.CmdCreate, child, "--from", parent, "--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
			t.Fatalf("wt create %s: %v", child, err)
		}
	}

	// Default: the child keeps pointing at the removed parent.
	buildStack(t, "feat/parent", "feat/child")
	stdout, _, err := runWtCmd(t, domain.CmdClean, "feat/parent", "--yes", "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("wt clean: %v", err)
	}
	var got domain.CleanBatchResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode clean JSON: %v (payload %q)", err, stdout)
	}
	if len(got.Reparented) != 0 {
		t.Errorf("reparented = %v, want none under --yes without --%s", got.Reparented, domain.FlagReparentChildren)
	}
	if len(got.OrphanedChildren) != 1 || got.OrphanedChildren[0].Branch != "feat/child" {
		t.Fatalf("orphaned_children = %v, want the child left dangling", got.OrphanedChildren)
	}

	// Opt in: the child is reparented onto the grandparent.
	buildStack(t, "feat/parent2", "feat/child2")
	stdout, _, err = runWtCmd(t, domain.CmdClean, "feat/parent2", "--yes",
		"--"+domain.FlagReparentChildren, "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("wt clean --%s: %v", domain.FlagReparentChildren, err)
	}
	got = domain.CleanBatchResult{}
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode clean JSON: %v (payload %q)", err, stdout)
	}
	if len(got.OrphanedChildren) != 0 {
		t.Errorf("orphaned_children = %v, want none once reparenting was authorized", got.OrphanedChildren)
	}
	if len(got.Reparented) != 1 || got.Reparented[0].NewParent != "main" {
		t.Fatalf("reparented = %v, want the child moved onto main", got.Reparented)
	}
}

// TestCleanAbsentWorktreeIsIdempotent: cleaning what is already gone is a success,
// so an agent can retry safely.
func TestCleanAbsentWorktreeIsIdempotent(t *testing.T) {
	createRepo(t)

	stdout, _, err := runWtCmd(t, domain.CmdClean, "feat/ghost", "--yes", "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("cleaning an absent worktree must succeed: %v", err)
	}
	var got domain.CleanBatchResult
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode clean JSON: %v (payload %q)", err, stdout)
	}
	if len(got.Results) != 1 || !got.Results[0].AlreadyAbsent {
		t.Errorf("already_absent = false, want true (payload %q)", stdout)
	}
}

// TestCleanYesWithoutBranchErrors guards the hardened bypass: under --yes a missing
// worktree branch errors naming the flag (no picker fallback), even in text mode
// where the output format alone would otherwise read as "interactive".
func TestCleanYesWithoutBranchErrors(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")
	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	if _, _, err := runWtCmd(t, domain.CmdClean, "--yes"); !errors.Is(err, domain.ErrCleanBranchRequired) {
		t.Fatalf("expected ErrCleanBranchRequired, got %v", err)
	}
}

func TestCleanRedirectsToBaseWhenInsideWorktree(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)

	goFile := filepath.Join(t.TempDir(), "go-file")
	t.Setenv(domain.EnvGoFile, goFile)

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	branch := "feat/redirect-test"
	if _, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
		t.Fatalf("wt create: %v", err)
	}

	wtPath := resolveWorktreePath(t, dir, branch)

	restore := chdir(t, wtPath)
	if _, _, err := runWtCmd(t, domain.CmdClean, branch, "--yes", "--force", "--output", domain.OutputJSON); err != nil {
		restore()
		t.Fatalf("wt clean: %v", err)
	}
	restore()

	got, err := os.ReadFile(goFile)
	if err != nil {
		t.Fatalf("read go-file: %v", err)
	}
	if string(got) != dir {
		t.Errorf("go-file = %q, want base repo %q", string(got), dir)
	}
}

func TestCleanDoesNotRedirectFromOtherWorktree(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)

	goFile := filepath.Join(t.TempDir(), "go-file")
	t.Setenv(domain.EnvGoFile, goFile)

	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}

	branch := "feat/other-test"
	if _, _, err := runWtCmd(t, domain.CmdCreate, branch, "--from", "main", "--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
		t.Fatalf("wt create: %v", err)
	}

	// Stay in the base repo while cleaning a different worktree.
	restore := chdir(t, dir)
	if _, _, err := runWtCmd(t, domain.CmdClean, branch, "--yes", "--force", "--output", domain.OutputJSON); err != nil {
		restore()
		t.Fatalf("wt clean: %v", err)
	}
	restore()

	if _, err := os.Stat(goFile); !os.IsNotExist(err) {
		t.Errorf("expected no go-file to be written, stat returned: %v", err)
	}
}

// resolveWorktreePath returns the on-disk path of the worktree for branch,
// tolerating the sanitized-name fallback used when the raw name has slashes.
func resolveWorktreePath(t *testing.T, dir, branch string) string {
	t.Helper()
	candidate := filepath.Join(filepath.Dir(dir), ".trees", branch)
	if _, err := os.Stat(candidate); err == nil {
		return candidate
	}
	sanitized := filepath.Join(filepath.Dir(dir), ".trees", strings.ReplaceAll(branch, "/", "-"))
	if _, err := os.Stat(sanitized); err != nil {
		t.Fatalf("worktree dir not found at %q or %q", candidate, sanitized)
	}
	return sanitized
}

// chdir switches into path and returns a function restoring the previous cwd.
func chdir(t *testing.T, path string) func() {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(path); err != nil {
		t.Fatalf("chdir %q: %v", path, err)
	}
	return func() { _ = os.Chdir(prev) }
}

func setupMinimalConfig(t *testing.T, stateDir string) error {
	t.Helper()
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return err
	}
	content := `#:schema ./schemas/project.schema.json
[worktrees]
base_path = "../.trees"
base_branch = "main"

[env]
strategy = "example"

[hooks]
on_create = []
`
	return os.WriteFile(filepath.Join(stateDir, domain.ConfigFileName), []byte(content), 0o644)
}
