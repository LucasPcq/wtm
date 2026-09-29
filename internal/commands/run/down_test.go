package run

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// A job the daemon could not stop is a failure of the command, on either
// surface: the array still lists every job, and the exit code says the run did
// not do what it was asked (LUC-198).
func TestRunDownJSONExitsNonZeroWhenAJobIsLeftStanding(t *testing.T) {
	setupStartProject(t, &fakeDaemon{
		StopErrors: map[string]string{"api": "job api is not running"},
	})
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdDown, "--"+domain.FlagProfile, "dev", "--output", domain.OutputJSON)
	if !errors.Is(err, domain.ErrAborted) {
		t.Fatalf("err = %v, want ErrAborted", err)
	}

	var results []domain.JobActionResult
	if err := json.Unmarshal([]byte(stdout), &results); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want the whole profile:\n%s", len(results), stdout)
	}
	var failed *domain.JobActionResult
	for i := range results {
		if results[i].Name == "api" {
			failed = &results[i]
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

// The shape follows the arity of the command, never the branch it took: `run
// stop` names one job, so it answers with one object even when there is no
// daemon to ask.
func TestRunStopJSONStaysAnObjectWithNoDaemon(t *testing.T) {
	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{apiJob}})
	shortHome(t)
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdStop, "--"+domain.FlagJob, "api", "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("run stop: %v", err)
	}

	if strings.HasPrefix(strings.TrimSpace(stdout), "[") {
		t.Fatalf("run stop answered with an array:\n%s", stdout)
	}
	var result domain.JobActionResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if result.Name != "api" || result.Status != domain.JobActionNotRunning {
		t.Errorf("result = %+v, want api not_running", result)
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
	var result domain.JobActionResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if result.Status != domain.JobActionNotRunning {
		t.Errorf("status = %q, want not_running", result.Status)
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
	setupStartProject(t, &fakeDaemon{})
	fakeTTY(t, false)

	if _, _, err := runCmd(t, domain.CmdUp, "--"+domain.FlagProfile, "dev", "-d"); err != nil {
		t.Fatalf("run up: %v", err)
	}
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
	setupStartProject(t, &fakeDaemon{})
	fakeTTY(t, false)

	if _, _, err := runCmd(t, domain.CmdUp, "--"+domain.FlagProfile, "dev", "-d"); err != nil {
		t.Fatalf("run up: %v", err)
	}
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
