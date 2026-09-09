package run

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

func exitCode(code int) *int { return &code }

// failingMigration is the profile every abort test runs: a service that comes
// up, then a task that writes a page of output before the daemon reports it
// failed, then a service the run never reaches.
func failingMigration() *fakeDaemon {
	return &fakeDaemon{Answers: map[string][]process.Response{
		"migrate": {
			{Status: process.StatusOutput, Data: []byte("applying 001\n")},
			{Status: process.StatusOutput, Data: []byte("ERROR: relation \"users\" does not exist\n")},
			{Status: process.StatusError, Message: "task migrate failed: exit status 1", ExitCode: exitCode(1)},
		},
	}}
}

func setupUpProject(t *testing.T, daemon *fakeDaemon) *fakeDaemon {
	t.Helper()

	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{
			{Name: "docker", Kind: domain.JobKindService, Cmd: "docker compose up -d", Stop: "docker compose down"},
			migrateJob,
			apiJob,
		},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"docker", "migrate", "api"}, Default: true},
		},
	})
	return startFakeDaemon(t, daemon)
}

func TestRunUpOpensTheViewOnATerminal(t *testing.T) {
	daemon := setupUpProject(t, &fakeDaemon{})
	view := captureRunView(t)
	fakeTTY(t, true)

	if _, _, err := runCmd(t, domain.CmdUp); err != nil {
		t.Fatalf("run up: %v", err)
	}

	call := view.only(t)
	if !call.Attached {
		t.Fatal("the view was opened without the start sequence to drive")
	}
	if call.Job != "" {
		t.Errorf("the view opened focused on %q, want the profile's first job", call.Job)
	}
	if got := strings.Join(daemon.startedJobs(), ","); got != "docker,migrate,api" {
		t.Errorf("the daemon started %q, want the profile in declared order", got)
	}
}

func TestRunUpDetachedNeverOpensTheView(t *testing.T) {
	daemon := setupUpProject(t, &fakeDaemon{})
	view := captureRunView(t)
	fakeTTY(t, true)

	stdout, _, err := runCmd(t, domain.CmdUp, "-d")
	if err != nil {
		t.Fatalf("run up -d: %v", err)
	}

	if len(view.calls) != 0 {
		t.Fatalf("-d opened the view: %+v", view.calls)
	}
	for _, want := range []string{"docker started", "api started", domain.RunStreamAttachHint} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout is missing %q\n--- stdout ---\n%s", want, stdout)
		}
	}
	if got := strings.Join(daemon.startedJobs(), ","); got != "docker,migrate,api" {
		t.Errorf("the daemon started %q, want the profile in declared order", got)
	}
}

func TestRunUpJSONNeverOpensTheView(t *testing.T) {
	setupUpProject(t, &fakeDaemon{})
	view := captureRunView(t)
	fakeTTY(t, true)

	if _, _, err := runCmd(t, domain.CmdUp, "--output", domain.OutputJSON); err != nil {
		t.Fatalf("run up --output json: %v", err)
	}

	if len(view.calls) != 0 {
		t.Fatalf("--output json opened the view: %+v", view.calls)
	}
}

// A machine consumer never saw the live stream, so the failing job's own output
// is the only thing that says why the profile stopped.
func TestRunUpJSONCarriesTheFailingJobsOutput(t *testing.T) {
	setupUpProject(t, failingMigration())
	captureRunView(t)
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdUp, "--output", domain.OutputJSON)
	if !errors.Is(err, domain.ErrAborted) {
		t.Fatalf("err = %v, want ErrAborted", err)
	}

	var results []domain.JobActionResult
	if err := json.Unmarshal([]byte(stdout), &results); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want docker started and migrate failed:\n%s", len(results), stdout)
	}

	if results[0].Name != "docker" || results[0].Status != domain.JobActionStarted {
		t.Errorf("first result = %+v, want docker started", results[0])
	}

	failed := results[1]
	if failed.Name != "migrate" || failed.Status != domain.JobActionError {
		t.Fatalf("second result = %+v, want migrate error", failed)
	}
	if failed.Message != "task migrate failed: exit status 1" {
		t.Errorf("message = %q, want the daemon's reason on its own", failed.Message)
	}
	if !strings.Contains(failed.Output, "applying 001") || !strings.Contains(failed.Output, "does not exist") {
		t.Errorf("output = %q, want everything the task wrote", failed.Output)
	}
	if failed.ExitCode == nil || *failed.ExitCode != 1 {
		t.Errorf("exit_code = %v, want the 1 the daemon reported", failed.ExitCode)
	}
}

// An aborted run exits non-zero like every other run command, and still writes
// its whole document: the two were never in tension — an exit code has never
// made a document unreadable (LUC-198).
func TestRunUpJSONExitsNonZeroOnAnAbortWithACompleteDocument(t *testing.T) {
	setupUpProject(t, failingMigration())
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdUp, "--output", domain.OutputJSON)
	if !errors.Is(err, domain.ErrAborted) {
		t.Fatalf("err = %v, want ErrAborted", err)
	}

	var results []domain.JobActionResult
	if err := json.Unmarshal([]byte(stdout), &results); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want the whole run:\n%s", len(results), stdout)
	}
}

func TestRunUpOnAStreamReportsTheAbortAndFails(t *testing.T) {
	setupUpProject(t, failingMigration())
	fakeTTY(t, false)

	stdout, stderr, err := runCmd(t, domain.CmdUp)
	if !errors.Is(err, domain.ErrAborted) {
		t.Fatalf("err = %v, want ErrAborted", err)
	}

	if !strings.Contains(stdout, "does not exist") {
		t.Errorf("the task's output never reached the scrollback:\n%s", stdout)
	}
	for _, want := range []string{"step 2/3", "Left running", "docker", "Not started", "api"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the abort report is missing %q\n--- stderr ---\n%s", want, stderr)
		}
	}
}
