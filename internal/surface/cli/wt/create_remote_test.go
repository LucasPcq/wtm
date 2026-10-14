package wt

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	createflow "github.com/LucasPcq/wtm/internal/flow/create"
	"github.com/LucasPcq/wtm/internal/testutil/flowtest"
)

// pushedOnlyRepo is a project whose branch exists only on origin, one commit
// past main, as a branch someone else pushed is after a fetch.
func pushedOnlyRepo(t *testing.T, branchName string) (work, tip string) {
	t.Helper()
	work, err := filepath.EvalSymlinks(repoWithRemote(t))
	if err != nil {
		t.Fatal(err)
	}
	stateDir := filepath.Join(work, ".git", "wtm")
	t.Setenv(domain.EnvProjectDir, work)
	t.Setenv(domain.EnvStateDir, stateDir)
	t.Setenv(domain.EnvGoFile, "")
	if err := setupMinimalConfig(t, stateDir); err != nil {
		t.Fatalf("setup config: %v", err)
	}
	gitRun(t, work, "checkout", "-b", branchName)
	gitRun(t, work, "commit", "--allow-empty", "-m", "pushed work")
	gitRun(t, work, "push", "origin", branchName)
	gitRun(t, work, "checkout", "main")
	gitRun(t, work, "branch", "-D", branchName)
	gitRun(t, work, "fetch", "origin")
	return work, revParse(t, work, domain.RemoteBranchPrefix+branchName)
}

func assertTracksOrigin(t *testing.T, work, branchName, tip string) {
	t.Helper()
	path := resolveWorktreePath(t, work, branchName)
	if head := revParse(t, path, "HEAD"); head != tip {
		t.Errorf("worktree HEAD = %s, want origin's tip %s", head, tip)
	}
	out, err := exec.Command("git", "-C", path, "rev-parse", "--abbrev-ref", branchName+"@{upstream}").Output()
	if got, want := strings.TrimSpace(string(out)), domain.RemoteBranchPrefix+branchName; err != nil || got != want {
		t.Errorf("upstream = %q (%v), want %q", got, err, want)
	}
}

// LUC-305: a branch only on origin was created from main, without upstream.
func TestWtCreateResumesARemoteOnlyBranch(t *testing.T) {
	work, tip := pushedOnlyRepo(t, "feat/remote")

	stdout, _, err := runWtCmd(t, domain.CmdCreate, "feat/remote", "--output", domain.OutputJSON, "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if got := decodeCreated(t, stdout); got.Upstream != "origin/feat/remote" || got.ExistingBranch {
		t.Errorf("result = %+v, want upstream origin/feat/remote on a branch created locally", got)
	}
	assertTracksOrigin(t, work, "feat/remote", tip)
}

func TestWtCreateWizardResumesARemoteOnlyBranch(t *testing.T) {
	work, tip := pushedOnlyRepo(t, "feat/remote")
	prompter := &flowtest.ScriptedPrompter{Answers: map[string]string{
		createflow.KeyBranch:    "feat/remote",
		createflow.KeySource:    "main",
		createflow.KeyEnv:       "",
		createflow.KeyIsolation: string(domain.IsolationIsolated),
		createflow.KeyRecap:     "create",
	}}
	onATerminal(t, prompter)

	stdout, _, err := runWtCmd(t, domain.CmdCreate)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	assertTracksOrigin(t, work, "feat/remote", tip)
	if !strings.Contains(stdout, "from origin/feat/remote") {
		t.Errorf("output %q should say the branch resumes origin/feat/remote", stdout)
	}
}
