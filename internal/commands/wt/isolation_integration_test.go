package wt

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

const mainEnv = "WEB_PORT=3000\nREALM=main-realm\n"

// isolationRepo is a project whose .env is copied from main, with one port
// linked and one [[env]] value — the two things a verbatim worktree must keep.
func isolationRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv("WTM_PROJECT_DIR", dir)
	t.Setenv("WTM_STATE_DIR", stateDir)
	t.Setenv(domain.EnvGoFile, "")
	t.Setenv(domain.EnvComposeProjectName, "")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	project := "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n\n[env]\nstrategy = \"main\"\n\n[[env.file]]\ntarget = \".env\"\n\n[hooks]\non_create = []\n"
	if err := os.WriteFile(filepath.Join(stateDir, domain.ConfigFileName), []byte(project), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := config.WriteRun(config.WriteRunParams{StateDir: stateDir, Force: true, Config: domain.RunConfig{
		Jobs: []domain.JobConfig{{
			Name: "web", Kind: domain.JobKindService, Cmd: "pnpm dev",
			Ports: map[string]int{"PORT": 3000},
		}},
		EnvPorts:  []domain.EnvPortLink{{File: ".env", Key: "WEB_PORT", Job: "web", Port: "PORT"}},
		EnvValues: []domain.EnvValueLink{{File: ".env", Key: "REALM", Job: "web", Value: "app-{worktree}"}},
	}}); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(mainEnv), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func worktreeEnv(t *testing.T, dir, branch string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(dir, "..", ".trees", rules.SanitizeBranchName(branch), ".env"))
	if err != nil {
		t.Fatalf("read %s .env: %v", branch, err)
	}
	return string(body)
}

func TestCreateVerbatimCopiesTheEnvExactly(t *testing.T) {
	dir := isolationRepo(t)

	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/v", "--from", "main", "--yes", "--"+domain.FlagIsolation, "verbatim"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if got := worktreeEnv(t, dir, "feat/v"); got != mainEnv {
		t.Errorf(".env = %q, want main's exactly", got)
	}
}

func TestCreateIsolatedMovesPortsAndSlices(t *testing.T) {
	dir := isolationRepo(t)

	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/i", "--from", "main", "--yes"); err != nil {
		t.Fatalf("create: %v", err)
	}
	got := worktreeEnv(t, dir, "feat/i")
	if strings.Contains(got, "WEB_PORT=3000") || !strings.Contains(got, "REALM=app-feat-i") {
		t.Errorf(".env = %q, want the port shifted and the realm named for the worktree", got)
	}
}

func TestCreateRefusesAnUnknownIsolation(t *testing.T) {
	isolationRepo(t)
	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/x", "--from", "main", "--yes", "--"+domain.FlagIsolation, "keep"); err == nil {
		t.Fatal("expected an unknown --isolation to be refused")
	}
}

// Switching a verbatim worktree to isolated is the way back: the next pass
// moves everything the creation left alone.
func TestEnvSwitchesAWorktreesIsolation(t *testing.T) {
	dir := isolationRepo(t)
	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/v", "--from", "main", "--yes", "--"+domain.FlagIsolation, "verbatim"); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Nothing given: a verbatim worktree stays as copied, whatever `wtm env` runs.
	if _, _, err := runWtCmd(t, domain.CmdEnv, "feat/v", "--yes"); err != nil {
		t.Fatalf("env: %v", err)
	}
	if got := worktreeEnv(t, dir, "feat/v"); got != mainEnv {
		t.Fatalf(".env = %q after a plain `wtm env`, want it untouched", got)
	}

	if _, _, err := runWtCmd(t, domain.CmdEnv, "feat/v", "--yes", "--"+domain.FlagIsolation, "isolated"); err != nil {
		t.Fatalf("env --isolation isolated: %v", err)
	}
	if got := worktreeEnv(t, dir, "feat/v"); strings.Contains(got, "WEB_PORT=3000") || !strings.Contains(got, "REALM=app-feat-v") {
		t.Errorf(".env = %q, want it isolated", got)
	}
}

func TestEnvRefusesIsolationWithCheck(t *testing.T) {
	isolationRepo(t)
	_, _, err := runWtCmd(t, domain.CmdEnv, "main", "--check", "--"+domain.FlagIsolation, "verbatim")
	if err == nil || !strings.Contains(err.Error(), "--check") {
		t.Errorf("err = %v, want --isolation refused under --check", err)
	}
}

// G1: run.toml is the run module's. A broken one leaves the key
// reconciliation as it always was and only skips the port pass, saying so.
func TestEnvReconcilesKeysOverAnInvalidRunToml(t *testing.T) {
	dir := isolationRepo(t)
	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/e", "--from", "main", "--yes"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".env"), []byte(mainEnv+"NEW_KEY=1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".git", "wtm", domain.RunFileName), []byte("bogus_key = 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, _, err := runWtCmd(t, domain.CmdEnv, "feat/e", "--yes", "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("env must not fail over run.toml: %v", err)
	}
	if got := worktreeEnv(t, dir, "feat/e"); !strings.Contains(got, "NEW_KEY=1") {
		t.Errorf(".env = %q, want the missing key added", got)
	}
	var result domain.EnvSyncResult
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("decode env result: %v\n%s", err, out)
	}
	if len(result.Warnings) != 1 || !strings.Contains(result.Warnings[0], "bogus_key") || !strings.Contains(result.Warnings[0], "wtm env feat/e") {
		t.Errorf("warnings = %v, want the skipped port pass named", result.Warnings)
	}

	human, _, err := runWtCmd(t, domain.CmdEnv, "feat/e", "--yes")
	if err != nil {
		t.Fatalf("env: %v", err)
	}
	if !strings.Contains(human, "bogus_key") {
		t.Errorf("report = %q, want the skipped port pass named", human)
	}
}
