package run

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// runningHere makes the fake daemon report the jobs as up in the project's
// main worktree, the one every command under test runs from.
func runningHere(t *testing.T, daemon *fakeDaemon, names ...string) string {
	t.Helper()
	main := gitToplevel(t, projectDirOf(os.Getenv("WTM_STATE_DIR")))
	jobs := make([]domain.JobInfo, 0, len(names))
	for _, name := range names {
		jobs = append(jobs, domain.JobInfo{Name: name, Status: domain.JobStatusRunning, WorkDir: main})
	}
	daemon.setJobs(jobs)
	return main
}

func decodeWorktreeResults(t *testing.T, stdout string) []domain.WorktreeJobResults {
	t.Helper()
	var results []domain.WorktreeJobResults
	if err := json.Unmarshal([]byte(stdout), &results); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	return results
}

// A job the daemon could not stop is a failure of the command, on either
// surface: the document still lists every job, and the exit code says the run
// did not do what it was asked (LUC-198).
func TestRunDownJSONExitsNonZeroWhenAJobIsLeftStanding(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{
		StopErrors: map[string]string{"api": "job api refused to stop"},
	})
	runningHere(t, daemon, "api", "migrate")
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdDown, "--"+domain.FlagProfile, "dev", "--output", domain.OutputJSON)
	if !errors.Is(err, domain.ErrAborted) {
		t.Fatalf("err = %v, want ErrAborted", err)
	}

	results := decodeWorktreeResults(t, stdout)
	if len(results) != 1 || len(results[0].Jobs) != 2 {
		t.Fatalf("got %+v, want one worktree holding the whole profile", results)
	}
	var failed *domain.JobActionResult
	for i := range results[0].Jobs {
		if results[0].Jobs[i].Name == "api" {
			failed = &results[0].Jobs[i]
		}
	}
	if failed == nil || failed.Status != domain.JobActionError {
		t.Fatalf("api = %+v, want an error entry", failed)
	}
}

func TestRunDownExitsZeroWhenEverythingStopped(t *testing.T) {
	setupStartProject(t, &fakeDaemon{})
	fakeTTY(t, false)

	if _, _, err := runCmd(t, domain.CmdDown, "--"+domain.FlagProfile, "dev", "--output", domain.OutputJSON); err != nil {
		t.Fatalf("run down: %v", err)
	}
}

// A profile job that was not up was not stopped: saying so claimed an act that
// never happened, and the daemon is not asked to stop it.
func TestRunDownProfileReportsAJobThatWasNotUpAsNotRunning(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{})
	main := runningHere(t, daemon, "api")
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdDown, "--"+domain.FlagProfile, "dev", "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("run down: %v", err)
	}
	results := decodeWorktreeResults(t, stdout)
	if len(results) != 1 || results[0].Branch != "main" || results[0].Path != main {
		t.Fatalf("results = %+v, want main by branch and path", results)
	}
	statuses := map[string]string{}
	for _, job := range results[0].Jobs {
		statuses[job.Name] = job.Status
	}
	if statuses["api"] != domain.JobActionStopped || statuses["migrate"] != domain.JobActionNotRunning {
		t.Errorf("statuses = %v, want api stopped and migrate not_running", statuses)
	}
	if strings.Contains(strings.Join(daemon.actions(), ","), string(process.ActionStop)+" migrate") {
		t.Errorf("daemon was asked to stop a job that was not up: %v", daemon.actions())
	}
}

// One shape whatever the arity: a stop over one worktree is an array of one
// document, even when there is no daemon to ask.
func TestRunStopJSONIsAnArrayOfWorktreesWithNoDaemon(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{apiJob}})
	shortHome(t)
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdStop, "--"+domain.FlagJob, "api", "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("run stop: %v", err)
	}

	results := decodeWorktreeResults(t, stdout)
	if len(results) != 1 || results[0].Branch != "main" || results[0].Path == "" {
		t.Fatalf("results = %+v, want main by branch and path", results)
	}
	if len(results[0].Jobs) != 1 || results[0].Jobs[0].Name != "api" || results[0].Jobs[0].Status != domain.JobActionNotRunning {
		t.Errorf("jobs = %+v, want api not_running", results[0].Jobs)
	}
}

// A daemon that does not hold the job in this worktree stopped nothing, and the
// command must not say it did.
func TestRunStopReportsAJobTheDaemonDoesNotHoldAsNotRunning(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{Jobs: []domain.JobInfo{{Name: "api", Status: domain.JobStatusRunning, WorkDir: "/elsewhere"}}})
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdStop, "--"+domain.FlagJob, "api", "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("run stop: %v", err)
	}
	results := decodeWorktreeResults(t, stdout)
	if len(results) != 1 || len(results[0].Jobs) != 1 || results[0].Jobs[0].Status != domain.JobActionNotRunning {
		t.Errorf("results = %+v, want api not_running", results)
	}
	if strings.Contains(strings.Join(daemon.actions(), ","), string(process.ActionStop)+" api") {
		t.Errorf("daemon was asked to stop a job it does not hold here: %v", daemon.actions())
	}

	human, _, err := runCmd(t, domain.CmdStop, "--"+domain.FlagJob, "api", "--"+domain.FlagYes)
	if err != nil {
		t.Fatalf("run stop: %v", err)
	}
	if !strings.Contains(human, "api not running") || strings.Contains(human, "stopped") {
		t.Errorf("human output = %q, want api not running", human)
	}
}

// Stopping a profile and starting it are two halves of one command, and read as
// two different programs when only one of them has a shape.
func TestRunDownConcludesInTheSameBoxRunUpDoes(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{})
	fakeTTY(t, false)

	if _, _, err := runCmd(t, domain.CmdUp, "--"+domain.FlagProfile, "dev", "-d"); err != nil {
		t.Fatalf("run up: %v", err)
	}
	runningHere(t, daemon, "api")
	stdout, _, err := runCmd(t, domain.CmdDown, "--"+domain.FlagProfile, "dev")
	if err != nil {
		t.Fatalf("run down: %v", err)
	}

	body := ansi.Strip(stdout)
	for _, want := range []string{
		domain.RunViewRecapTitle,
		fmt.Sprintf(domain.RunViewRecapProfileFmt, "dev"),
		domain.RunStreamUpHint,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("run down is missing %q:\n%s", want, body)
		}
	}
}

// The worktree is named whatever the arity: the run they do most is exactly the
// one that must not leave them guessing which worktree it emptied.
func TestRunDownNamesTheWorktreeItEmptied(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{})
	fakeTTY(t, false)

	if _, _, err := runCmd(t, domain.CmdUp, "--"+domain.FlagProfile, "dev", "-d"); err != nil {
		t.Fatalf("run up: %v", err)
	}
	runningHere(t, daemon, "api")
	stdout, _, err := runCmd(t, domain.CmdDown, "--"+domain.FlagProfile, "dev")
	if err != nil {
		t.Fatalf("run down: %v", err)
	}

	body := ansi.Strip(stdout)
	if !strings.Contains(body, "Stopped:") {
		t.Errorf("run down never says what it took down:\n%s", body)
	}
	if !strings.Contains(body, "api") {
		t.Errorf("run down does not name the jobs it stopped:\n%s", body)
	}
}

// The daemon is machine-wide: --all empties every worktree of this repository,
// one by one, and never reaches into another one.
func TestRunDownAllStaysInThisRepository(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{})
	stateDir := os.Getenv("WTM_STATE_DIR")
	main := gitToplevel(t, projectDirOf(stateDir))
	linked := gitToplevel(t, addWorktree(t, main, "feat/all"))
	daemon.setJobs([]domain.JobInfo{
		{Name: "api", Status: domain.JobStatusRunning, WorkDir: main},
		{Name: "api", Status: domain.JobStatusDetached, WorkDir: linked},
		{Name: "web", Status: domain.JobStatusRunning, WorkDir: "/some/other/repo"},
	})
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdDown, "--"+domain.FlagAll, "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("run down --all: %v", err)
	}

	daemon.mu.Lock()
	var emptied []string
	for _, req := range daemon.requests {
		if req.Action == process.ActionStopAll {
			emptied = append(emptied, req.WorkDir)
		}
	}
	daemon.mu.Unlock()
	if len(emptied) != 2 || emptied[0] == "" || emptied[1] == "" {
		t.Fatalf("stop_all sent for %q, want this repository's two worktrees by name", emptied)
	}
	for _, dir := range emptied {
		if dir == "/some/other/repo" {
			t.Errorf("run down --all reached another repository")
		}
	}

	var results []domain.WorktreeJobResults
	if err := json.Unmarshal([]byte(stdout), &results); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if len(results) != 2 {
		t.Errorf("results = %+v, want one per worktree emptied", results)
	}
}
