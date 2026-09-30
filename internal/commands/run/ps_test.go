package run

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/testutil/gittest"
)

// `run ps` lists the jobs of every repository the daemon knows, so requiring the
// caller to stand inside a run-initialized one refused a listing it was going to
// produce regardless — the bug found while testing LUC-211.
func TestPsListsFromARepositoryWithNoRunModule(t *testing.T) {
	shortHome(t)
	dir := gittest.InitRepo(t)
	t.Setenv("WTM_PROJECT_DIR", dir)
	t.Setenv("WTM_STATE_DIR", dir+"/.git/wtm")
	fakeTTY(t, true)

	if _, _, err := runCmd(t, domain.CmdPs); err != nil {
		t.Fatalf("run ps: %v", err)
	}
}

// It answers with a table whether or not it owns a terminal: the picker it used
// to open in one is what the run view does now, over as many worktrees as the
// reader selects.
func TestPsAnswersWithATableOnATerminal(t *testing.T) {
	setupStartProject(t, &fakeDaemon{})
	fakeTTY(t, true)

	stdout, _, err := runCmd(t, domain.CmdPs)
	if err != nil {
		t.Fatalf("run ps: %v", err)
	}
	if strings.TrimSpace(stdout) == "" {
		t.Error("run ps printed nothing")
	}
}

func TestPsAnswersWithADocumentForAMachine(t *testing.T) {
	setupStartProject(t, &fakeDaemon{})
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdPs, "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("run ps: %v", err)
	}
	var jobs []domain.JobInfo
	if err := json.Unmarshal([]byte(stdout), &jobs); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
}

// The listing is machine-wide, so its empty state says nothing about a worktree.
func TestPsEmptyStateIsNotAboutThisWorktree(t *testing.T) {
	setupStartProject(t, &fakeDaemon{})
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdPs)
	if err != nil {
		t.Fatalf("run ps: %v", err)
	}
	if !strings.Contains(stdout, domain.RunNoJobsRunning) || strings.Contains(stdout, domain.RunNoJobsHere) {
		t.Errorf("stdout = %q, want %q", stdout, domain.RunNoJobsRunning)
	}
}
