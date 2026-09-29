package checkout

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// createFromPR is exercised directly (white-box, same package) rather than through
// runCheckout: resolving a domain.PRInfo is the one step that needs GitHub, and
// createFromPR takes it as a plain param, so the reuse path is fully testable
// against a real local repo without a network dependency.

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %s: %v", args, out, err)
	}
}

func revParse(t *testing.T, dir, ref string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", ref)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse %s: %v", ref, err)
	}
	return strings.TrimSpace(string(out))
}

// repoWithRemote returns a work repo (on main) wired to a fresh bare origin, with
// main pushed so origin-tracking refs exist.
func repoWithRemote(t *testing.T) string {
	t.Helper()
	work := gittest.InitRepo(t)
	origin := t.TempDir()
	if out, err := exec.Command("git", "init", "--bare", origin).CombinedOutput(); err != nil {
		t.Fatalf("git init --bare: %s: %v", out, err)
	}
	git(t, work, "remote", "add", "origin", origin)
	git(t, work, "push", "-u", "origin", "main")
	return work
}

func loadResult(t *testing.T, projectDir string) shared.ConfigResult {
	t.Helper()
	stateDir := filepath.Join(projectDir, ".git", "wtm")
	t.Setenv("WTM_PROJECT_DIR", projectDir)
	t.Setenv("WTM_STATE_DIR", stateDir)
	t.Setenv(domain.EnvGoFile, "")

	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
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
	if err := os.WriteFile(filepath.Join(stateDir, domain.ConfigFileName), []byte(content), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cmd := &cobra.Command{}
	result, err := shared.LoadConfig(cmd, projectDir)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return result
}

// runCmd returns a bare command with its stdout captured in out, for decoding
// the JSON createFromPR writes.
func runCmd() (cmd *cobra.Command, out *bytes.Buffer) {
	cmd = &cobra.Command{}
	out = &bytes.Buffer{}
	cmd.SetOut(out)
	cmd.SetErr(&bytes.Buffer{})
	return cmd, out
}

// LUC-172: a local branch of the PR's name is reused as-is, not created from
// origin — createFromPR reports the reuse and keeps the branch's own commits.
func TestCreateFromPRReusesExistingLocalBranch(t *testing.T) {
	work := repoWithRemote(t)
	result := loadResult(t, work)

	branch := "feat/pr-branch"
	git(t, work, "branch", branch)
	git(t, work, "push", "origin", branch)
	// Local-only commit the PR's origin branch does not have.
	git(t, work, "checkout", branch)
	git(t, work, "commit", "--allow-empty", "-m", "local-only")
	want := revParse(t, work, branch)
	git(t, work, "checkout", "main")

	cmd, out := runCmd()
	err := createFromPR(cmd, result, createFromPRParams{
		pr:           domain.PRInfo{Number: 1, Branch: branch, BaseBranch: "main"},
		parent:       "main",
		jsonMode:     true,
		interactive:  false,
		envConfirmed: true,
	})
	if err != nil {
		t.Fatalf("createFromPR: %v", err)
	}

	var got output.PRCheckoutJSON
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode checkout JSON: %v", err)
	}
	if !got.ExistingBranch {
		t.Error("existing_branch should report the reuse")
	}

	if head := revParse(t, work, branch); head != want {
		t.Errorf("branch %s = %s, want unchanged local tip %s — commits were lost", branch, head, want)
	}
}

// LUC-172: --yes / JSON never touch refs — a reused branch behind origin is left
// exactly where it is, and origin_state says so.
func TestCreateFromPRNonInteractiveDoesNotFastForward(t *testing.T) {
	work := repoWithRemote(t)
	result := loadResult(t, work)

	branch := "feat/behind"
	git(t, work, "branch", branch)
	git(t, work, "push", "origin", branch)
	local := revParse(t, work, branch)
	// Origin advances without the local branch moving, so it falls behind.
	git(t, work, "commit", "--allow-empty", "-m", "server-commit")
	git(t, work, "push", "origin", "main:"+branch)

	cmd, out := runCmd()
	err := createFromPR(cmd, result, createFromPRParams{
		pr:           domain.PRInfo{Number: 2, Branch: branch, BaseBranch: "main"},
		parent:       "main",
		jsonMode:     true,
		interactive:  false,
		envConfirmed: true,
	})
	if err != nil {
		t.Fatalf("createFromPR: %v", err)
	}

	if got := revParse(t, work, branch); got != local {
		t.Errorf("branch %s moved to %s under non-interactive checkout, want unchanged %s", branch, got, local)
	}

	var got output.PRCheckoutJSON
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode checkout JSON: %v", err)
	}
	if got.OriginState != domain.DivergenceLabelBehind {
		t.Errorf("origin_state = %q, want %q", got.OriginState, domain.DivergenceLabelBehind)
	}
}

// G1: a run.toml the port pass refuses leaves the PR worktree created and
// its hooks run, with the pass left undone named in the JSON.
func TestCreateFromPRGoesAheadOverAnInvalidRunToml(t *testing.T) {
	work := repoWithRemote(t)
	result := loadResult(t, work)
	result.Config.Project.Hooks.OnCreate = []domain.HookCommand{{Cmd: "touch hook-ran"}}
	if err := os.WriteFile(filepath.Join(result.StateDir, domain.RunFileName), []byte("bogus_key = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	branch := "feat/pr-broken-run"
	git(t, work, "branch", branch)
	git(t, work, "push", "origin", branch)
	git(t, work, "branch", "-D", branch)

	cmd, out := runCmd()
	if err := createFromPR(cmd, result, createFromPRParams{
		pr:           domain.PRInfo{Number: 3, Branch: branch, BaseBranch: "main"},
		parent:       "main",
		jsonMode:     true,
		envConfirmed: true,
	}); err != nil {
		t.Fatalf("createFromPR must not fail over run.toml: %v", err)
	}

	var got output.PRCheckoutJSON
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("decode checkout JSON: %v", err)
	}
	if _, err := os.Stat(filepath.Join(got.Path, "hook-ran")); err != nil {
		t.Errorf("on_create hooks did not run: %v", err)
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "bogus_key") {
		t.Errorf("warnings = %v, want the refused run.toml named", got.Warnings)
	}
}
