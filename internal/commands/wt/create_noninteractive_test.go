package wt

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// These tests characterize the non-interactive create path (--yes / --output json,
// or simply no terminal): every input resolves from a flag, from a documented safe
// default, or with an error naming the missing flag — never from a picker.

// createRepo wires a repo with the minimal config and returns its directory.
func createRepo(t *testing.T) string {
	t.Helper()
	dir := gittest.InitRepo(t)
	prepareRepo(t, dir)
	return dir
}

// prepareRepo points the wtm env vars at dir and writes the minimal config.
func prepareRepo(t *testing.T, dir string) {
	t.Helper()
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")
	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}
}

// TestWtCreateWithoutBranchNameErrors: the branch name is a required selection
// with no safe default, so a run that cannot prompt errors instead of asking.
func TestWtCreateWithoutBranchNameErrors(t *testing.T) {
	createRepo(t)

	_, _, err := runWtCmd(t, domain.CmdCreate, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("expected a missing branch name to error under --yes")
	}
	if !strings.Contains(err.Error(), "branch name is required") {
		t.Errorf("error %q should say the branch name is required", err)
	}
}

// TestWtCreateJSONRequiresYes: prompts cannot run in JSON mode, so the
// confirmation axis must be bypassed explicitly.
func TestWtCreateJSONRequiresYes(t *testing.T) {
	createRepo(t)

	_, _, err := runWtCmd(t, domain.CmdCreate, "feat/x", "--from", "main", "--output", domain.OutputJSON)
	if err == nil {
		t.Fatal("expected --output json without --yes to be rejected")
	}
	if !strings.Contains(err.Error(), "--"+domain.FlagYes) {
		t.Errorf("error %q should direct to --%s", err, domain.FlagYes)
	}
}

// TestWtCreateMissingBaseBranchErrors: the source defaults to the configured base
// branch, and that default is validated like any explicit --from.
func TestWtCreateMissingBaseBranchErrors(t *testing.T) {
	dir := gittest.InitRepo(t)
	stateDir := filepath.Join(dir, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, dir)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	config := "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"ghost\"\n\n[env]\nstrategy = \"example\"\n\n[hooks]\non_create = []\n"
	if err := os.WriteFile(filepath.Join(stateDir, domain.ConfigFileName), []byte(config), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	_, _, err := runWtCmd(t, domain.CmdCreate, "feat/x", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if !errors.Is(err, domain.ErrBranchNotFound) {
		t.Fatalf("error = %v, want it to wrap ErrBranchNotFound", err)
	}
}

// TestWtCreateValidatesFromBeforeTheParentGuard: --from is checked up front, so a
// nonexistent parent is reported as such even for a branch that already exists
// locally (where --from is the recorded sync parent, not a start-point).
func TestWtCreateValidatesFromBeforeTheParentGuard(t *testing.T) {
	dir := createRepo(t)
	gittest.CreateBranch(t, dir, "feat/existing")

	_, _, err := runWtCmd(t, domain.CmdCreate, "feat/existing", "--from", "ghost",
		"--output", domain.OutputJSON, "--"+domain.FlagYes)
	if !errors.Is(err, domain.ErrBranchNotFound) {
		t.Fatalf("error = %v, want it to wrap ErrBranchNotFound", err)
	}
	if strings.Contains(err.Error(), "--"+domain.FlagFrom+" ") {
		t.Errorf("error %q should report the unknown branch, not ask for the flag again", err)
	}
}

// TestWtCreateEnvFromOverridesTheConfigStrategy: --env-from resolves the env
// decision without a wizard, and the chosen strategy is what gets recorded.
func TestWtCreateEnvFromOverridesTheConfigStrategy(t *testing.T) {
	createRepo(t)

	tests := []struct {
		name string
		args []string
		want domain.EnvStrategy
	}{
		{"config default", nil, domain.EnvStrategyExample},
		{"--env-from main", []string{"--" + domain.FlagEnvFrom, string(domain.EnvStrategyMain)}, domain.EnvStrategyMain},
		{"--env-from parent", []string{"--" + domain.FlagEnvFrom, string(domain.EnvStrategyParent)}, domain.EnvStrategyParent},
	}
	for i, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := append([]string{domain.CmdCreate, "feat/env-" + string(rune('a'+i)), "--from", "main",
				"--output", domain.OutputJSON, "--" + domain.FlagYes}, tt.args...)
			stdout, _, err := runWtCmd(t, args...)
			if err != nil {
				t.Fatalf("wt create: %v", err)
			}
			var got domain.CreateResult
			if err := json.Unmarshal([]byte(stdout), &got); err != nil {
				t.Fatalf("decode create JSON: %v (payload %q)", err, stdout)
			}
			if got.Metadata.EnvStrategy != tt.want {
				t.Errorf("env_strategy = %q, want %q", got.Metadata.EnvStrategy, tt.want)
			}
		})
	}
}

// TestWtCreateFFFastForwardsTheSourceForANewBranch: for a branch git creates, the
// start-point is the source — so --ff updates the source and the worktree starts
// from the refreshed tip.
func TestWtCreateFFFastForwardsTheSourceForANewBranch(t *testing.T) {
	work := repoWithRemote(t)
	prepareRepo(t, work)

	// Origin's main advances without the local main following.
	gitRun(t, work, "commit", "--allow-empty", "-m", "server-commit")
	gitRun(t, work, "push", "origin", "main")
	wantTip := revParse(t, work, "main")
	gitRun(t, work, "reset", "--hard", "HEAD~1")
	if revParse(t, work, "main") == wantTip {
		t.Fatal("fixture: local main should be behind origin")
	}

	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/fresh", "--from", "main", "--"+domain.FlagFF,
		"--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
		t.Fatalf("wt create --ff: %v", err)
	}

	if got := revParse(t, work, "main"); got != wantTip {
		t.Errorf("main = %s, want fast-forwarded to origin %s", got, wantTip)
	}
	if got := revParse(t, resolveWorktreePath(t, work, "feat/fresh"), "HEAD"); got != wantTip {
		t.Errorf("worktree HEAD = %s, want the refreshed source tip %s", got, wantTip)
	}
}

// TestWtCreateFFLeavesADivergedSourceAlone: --ff is best effort. A source that
// cannot be fast-forwarded is left untouched and the creation still proceeds from
// the local branch, mirroring the interactive "keep it as-is" choice.
func TestWtCreateFFLeavesADivergedSourceAlone(t *testing.T) {
	work := repoWithRemote(t)
	prepareRepo(t, work)
	divergedBranch(t, work, "feat/diverged")
	localTip := revParse(t, work, "feat/diverged")

	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/child", "--from", "feat/diverged", "--"+domain.FlagFF,
		"--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
		t.Fatalf("wt create --ff on a diverged source: %v", err)
	}

	if got := revParse(t, work, "feat/diverged"); got != localTip {
		t.Errorf("diverged source moved to %s, want it left at %s", got, localTip)
	}
	if got := revParse(t, resolveWorktreePath(t, work, "feat/child"), "HEAD"); got != localTip {
		t.Errorf("worktree HEAD = %s, want the local source tip %s", got, localTip)
	}
}

// TestWtCreateWithoutFFDoesNotTouchTheSource is the counterpart: no --ff means no
// branch movement at all, even when the source is a clean fast-forward away.
func TestWtCreateWithoutFFDoesNotTouchTheSource(t *testing.T) {
	work := repoWithRemote(t)
	prepareRepo(t, work)
	gitRun(t, work, "commit", "--allow-empty", "-m", "server-commit")
	gitRun(t, work, "push", "origin", "main")
	gitRun(t, work, "reset", "--hard", "HEAD~1")
	staleTip := revParse(t, work, "main")

	if _, _, err := runWtCmd(t, domain.CmdCreate, "feat/stale", "--from", "main",
		"--output", domain.OutputJSON, "--"+domain.FlagYes); err != nil {
		t.Fatalf("wt create: %v", err)
	}

	if got := revParse(t, work, "main"); got != staleTip {
		t.Errorf("main moved to %s under --yes without --ff, want unchanged %s", got, staleTip)
	}
}

// TestWtCreateTextModeWithoutTerminalTakesTheFlagPath: the wizard needs a TTY, so
// a piped human-format run resolves from the flags like --yes does — including the
// missing-branch error.
func TestWtCreateTextModeWithoutTerminalTakesTheFlagPath(t *testing.T) {
	createRepo(t)

	if _, _, err := runWtCmd(t, domain.CmdCreate); err == nil {
		t.Fatal("expected the flag path to require a branch name without a terminal")
	}

	stdout, _, err := runWtCmd(t, domain.CmdCreate, "feat/piped", "--from", "main")
	if err != nil {
		t.Fatalf("wt create in text mode: %v", err)
	}
	if !strings.Contains(stdout, "feat/piped") {
		t.Errorf("stdout %q should recap the created worktree", stdout)
	}
}
