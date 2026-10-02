package checkout

import (
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/commands/shared"
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/output"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

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
	t.Setenv(domain.EnvProjectDir, projectDir)
	t.Setenv(domain.EnvStateDir, stateDir)
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

// G1: a run.toml the port pass refuses leaves the PR worktree created and its
// hooks run, with the pass left undone named in the JSON.
func TestCheckoutGoesAheadOverAnInvalidRunToml(t *testing.T) {
	repo := newCheckoutRepo(t)
	repo.pushBranch(t, "feat/thing")
	repo.writeConfig(t, `#:schema ./schemas/project.schema.json
[worktrees]
base_path = "../.trees"
base_branch = "main"

[env]
strategy = "example"

[hooks]
on_create = ["touch hook-ran"]
`)
	if err := os.WriteFile(filepath.Join(repo.stateDir, domain.RunFileName), []byte("bogus_key = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := runCheckoutCmd(t, "42", "--yes", "--output", "json")
	if err != nil {
		t.Fatalf("checkout must not fail over run.toml: %v", err)
	}
	var got output.PRCheckoutJSON
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode checkout JSON: %v", err)
	}
	if _, err := os.Stat(filepath.Join(got.Path, "hook-ran")); err != nil {
		t.Errorf("on_create hooks did not run: %v", err)
	}
	if len(got.Warnings) != 1 || !strings.Contains(got.Warnings[0], "bogus_key") {
		t.Errorf("warnings = %v, want the refused run.toml named", got.Warnings)
	}
}

func TestCheckoutJSONReportsIsolationAndEnvPorts(t *testing.T) {
	repo := newCheckoutRepo(t)
	repo.pushBranch(t, "feat/thing")
	repo.writeConfig(t, `#:schema ./schemas/project.schema.json
[worktrees]
base_path = "../.trees"
base_branch = "main"

[env]
strategy = "main"

[[env.file]]
target = ".env"
`)
	if err := config.WriteRun(config.WriteRunParams{StateDir: repo.stateDir, Force: true, Config: domain.RunConfig{
		Jobs:     []domain.JobConfig{{Name: "web", Kind: domain.JobKindService, Cmd: "true", Ports: map[string]int{"PORT": 3000}}},
		EnvPorts: []domain.EnvPortLink{{File: ".env", Key: "WEB_PORT", Job: "web", Port: "PORT"}},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo.work, ".env"), []byte("WEB_PORT=3000\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, _, err := runCheckoutCmd(t, "42", "--yes", "--isolation", string(domain.IsolationIsolated), "--output", "json")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	var got output.PRCheckoutJSON
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode checkout JSON: %v", err)
	}
	if got.Isolation != domain.IsolationIsolated {
		t.Errorf("isolation = %q, want isolated", got.Isolation)
	}
	if len(got.EnvPorts.Entries) != 1 || got.EnvPorts.Entries[0].Status != domain.EnvPortStatusRewrite {
		t.Errorf("env_ports = %+v, want WEB_PORT settled", got.EnvPorts)
	}
}

func TestCheckoutRefusesAnUnknownEnvStrategyBeforeCreating(t *testing.T) {
	repo := newCheckoutRepo(t)
	repo.pushBranch(t, "feat/thing")

	_, _, err := runCheckoutCmd(t, "42", "--yes", "--env-from", "bogus")
	if err == nil || !strings.Contains(err.Error(), `invalid --env-from value "bogus"`) {
		t.Fatalf("err = %v, want --env-from refused", err)
	}
	if _, statErr := os.Stat(filepath.Join(repo.work, "..", ".trees", "feat-thing")); statErr == nil {
		t.Error("a refused --env-from must leave no worktree behind")
	}
}

func TestCheckoutRefusesAnUnknownFrom(t *testing.T) {
	repo := newCheckoutRepo(t)
	repo.pushBranch(t, "feat/thing")

	_, _, err := runCheckoutCmd(t, "42", "--yes", "--from", "nope")
	if !errors.Is(err, domain.ErrBranchNotFound) {
		t.Fatalf("err = %v, want the unknown parent refused", err)
	}
	if _, statErr := os.Stat(filepath.Join(repo.work, "..", ".trees", "feat-thing")); statErr == nil {
		t.Error("a refused --from must leave no worktree behind")
	}
}

// Interactively the recap warns of it; unattended, the run says it afterwards.
func TestCheckoutWarnsAnUnattendedParentFallback(t *testing.T) {
	repo := newCheckoutRepo(t)
	repo.pushBranch(t, "feat/thing")
	git(t, repo.work, "branch", "develop")
	repo.writeConfig(t, `#:schema ./schemas/project.schema.json
[worktrees]
base_path = "../.trees"
base_branch = "main"

[env]
strategy = "example"

[[env.file]]
target = ".env"
`)
	if err := os.WriteFile(filepath.Join(repo.work, ".env"), []byte("A=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	stdout, stderr, err := runCheckoutCmd(t, "42", "--yes", "--from", "develop", "--env-from", "parent", "--output", "json")
	if err != nil {
		t.Fatalf("checkout: %v", err)
	}
	var got output.PRCheckoutJSON
	if err := json.Unmarshal([]byte(stdout), &got); err != nil {
		t.Fatalf("decode checkout JSON: %v", err)
	}
	if len(got.Warnings) != 1 || got.Warnings[0] != domain.EnvParentFallbackWarning {
		t.Errorf("warnings = %v, want the fallback named", got.Warnings)
	}
	if !strings.Contains(stderr, domain.EnvParentFallbackWarning) {
		t.Errorf("stderr = %q, want the fallback said", stderr)
	}
}
