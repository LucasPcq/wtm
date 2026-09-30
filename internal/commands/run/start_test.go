package run

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

var (
	apiJob     = domain.JobConfig{Name: "api", Kind: domain.JobKindService, Cmd: "pnpm dev"}
	migrateJob = domain.JobConfig{Name: "migrate", Kind: domain.JobKindTask, Cmd: "pnpm migrate"}
)

func setupStartProject(t *testing.T, daemon *fakeDaemon) *fakeDaemon {
	t.Helper()

	stateDir := setupTestProject(t)
	writeRunTOML(t, stateDir, domain.RunConfig{
		Jobs: []domain.JobConfig{apiJob, migrateJob},
		Profiles: []domain.ProfileConfig{
			{Name: "dev", Jobs: []string{"migrate", "api"}, Default: true},
		},
	})
	return startFakeDaemon(t, daemon)
}

func TestRunStartAttachesAServiceByDefault(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{})
	view := captureRunView(t)
	fakeTTY(t, true)

	if _, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagJob, "api"); err != nil {
		t.Fatalf("run start api: %v", err)
	}

	call := view.only(t)
	if call.Job != "api" {
		t.Errorf("the view opened on %q, want api", call.Job)
	}
	if !call.Attached {
		t.Error("the view was opened without a start sequence to drive")
	}
	if got := daemon.startedJobs(); len(got) != 1 || got[0] != "api" {
		t.Errorf("the daemon was asked to start %v, want [api]", got)
	}
}

func TestRunStartDetachedNeverOpensTheView(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{})
	view := captureRunView(t)
	fakeTTY(t, true)

	stdout, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagJob, "api", "-d")
	if err != nil {
		t.Fatalf("run start api -d: %v", err)
	}

	if len(view.calls) != 0 {
		t.Fatalf("-d opened the view: %+v", view.calls)
	}
	if !strings.Contains(stdout, "api started") {
		t.Errorf("stdout does not report the started service:\n%s", stdout)
	}
	if got := daemon.startedJobs(); len(got) != 1 || got[0] != "api" {
		t.Errorf("the daemon was asked to start %v, want [api]", got)
	}
}

// A task is a foreground command: its output belongs to the scrollback, so it
// runs inline on a terminal that would otherwise have got the view.
func TestRunStartTaskStaysInline(t *testing.T) {
	setupStartProject(t, &fakeDaemon{Answers: map[string][]process.Response{
		"migrate": {
			{Status: process.StatusOutput, Data: []byte("applying 001\n")},
			{Status: process.StatusDone},
		},
	}})
	view := captureRunView(t)
	fakeTTY(t, true)

	stdout, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagJob, "migrate")
	if err != nil {
		t.Fatalf("run start migrate: %v", err)
	}

	if len(view.calls) != 0 {
		t.Fatalf("a task opened the view: %+v", view.calls)
	}
	if !strings.Contains(stdout, "applying 001") {
		t.Errorf("the task's output never reached the scrollback:\n%s", stdout)
	}
	if !strings.Contains(stdout, "migrate done") {
		t.Errorf("stdout does not report the finished task:\n%s", stdout)
	}
}

func TestRunStartJSONNeverOpensTheView(t *testing.T) {
	setupStartProject(t, &fakeDaemon{})
	view := captureRunView(t)
	fakeTTY(t, true)

	stdout, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagJob, "api", "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("run start api --output json: %v", err)
	}

	if len(view.calls) != 0 {
		t.Fatalf("--output json opened the view: %+v", view.calls)
	}
	var result domain.JobActionResult
	if err := json.Unmarshal([]byte(stdout), &result); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	if result.Name != "api" || result.Status != domain.JobActionStarted {
		t.Errorf("result = %+v, want api started", result)
	}
}

func TestRunStartWithoutATerminalNeverOpensTheView(t *testing.T) {
	setupStartProject(t, &fakeDaemon{})
	view := captureRunView(t)
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagJob, "api")
	if err != nil {
		t.Fatalf("run start api: %v", err)
	}

	if len(view.calls) != 0 {
		t.Fatalf("a pipe opened the view: %+v", view.calls)
	}
	if !strings.Contains(stdout, "api started") {
		t.Errorf("stdout does not report the started service:\n%s", stdout)
	}
}

// --yes is the confirmation axis: it runs unattended, so a required selection
// with no safe default is refused by name rather than answered by a picker.
func TestRunStartYesRefusesWithoutAJob(t *testing.T) {
	setupStartProject(t, &fakeDaemon{})
	view := captureRunView(t)
	fakeTTY(t, true)

	_, stderr, err := runCmd(t, domain.CmdStart, "--"+domain.FlagYes)
	if err == nil {
		t.Fatal("run start --yes started something without being told which job")
	}
	if !strings.Contains(err.Error()+stderr, domain.FlagJob) {
		t.Errorf("the refusal %q does not name --%s", err, domain.FlagJob)
	}
	if len(view.calls) != 0 {
		t.Error("a picker ran under --yes")
	}
}

// --yes on a command whose every question has a safe default asks nothing and
// still runs: the worktree is the current one.
func TestRunStartYesWithAJobRunsUnattended(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{})
	captureRunView(t)
	fakeTTY(t, true)

	if _, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagYes, "--"+domain.FlagJob, "api", "-d"); err != nil {
		t.Fatalf("run start --yes --job api -d: %v", err)
	}
	if got := daemon.startedJobs(); len(got) != 1 || got[0] != "api" {
		t.Errorf("the daemon was asked to start %v, want [api]", got)
	}
}

// A run that failed still owes a machine reader a document: an exit code with
// no cause is what `run up`'s `output` field exists to avoid, and `run start`
// was writing nothing at all.
func TestRunStartJSONReportsAFailingJob(t *testing.T) {
	setupStartProject(t, failingMigration())
	captureRunView(t)
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagJob, "migrate", "--"+domain.FlagOutput, domain.OutputJSON)
	if err == nil {
		t.Fatal("a failing job exited zero")
	}

	var result domain.JobActionResult
	if decodeErr := json.Unmarshal([]byte(stdout), &result); decodeErr != nil {
		t.Fatalf("stdout is not a single object: %v\n%s", decodeErr, stdout)
	}
	if result.Name != "migrate" || result.Status != domain.JobActionError {
		t.Errorf("result = %+v, want the job named and marked in error", result)
	}
	if result.Message != "task migrate failed: exit status 1" {
		t.Errorf("message = %q, want the daemon's reason", result.Message)
	}
	if !strings.Contains(result.Output, "does not exist") {
		t.Errorf("output = %q, want what the task wrote before it died", result.Output)
	}
	if result.ExitCode == nil || *result.ExitCode != 1 {
		t.Errorf("exit_code = %v, want the code the daemon reported", result.ExitCode)
	}
}

// stoppedWorktrees names the worktrees the commands asked the daemon to clear.
func (d *fakeDaemon) stoppedWorktrees() []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	var dirs []string
	for _, req := range d.requests {
		if req.Action == process.ActionStopAll {
			dirs = append(dirs, req.WorkDir)
		}
	}
	return dirs
}

var elsewhere = []domain.JobInfo{{Name: "api", WorkDir: "/wt/elsewhere", Status: domain.JobStatusRunning}}

// run start asks run up's question about the other worktrees, and answers it
// the same way from the same flags.
func TestRunStartExclusiveStopsTheOtherWorktrees(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{Jobs: elsewhere})
	fakeTTY(t, false)

	if _, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagJob, "api", "--"+domain.FlagExclusive, "-y"); err != nil {
		t.Fatalf("run start --exclusive: %v", err)
	}

	if got := daemon.stoppedWorktrees(); len(got) != 1 || got[0] != "/wt/elsewhere" {
		t.Errorf("stopped %v, want the other worktree cleared first", got)
	}
}

// The safe default stops nothing.
func TestRunStartUnattendedLeavesTheOtherWorktreesRunning(t *testing.T) {
	daemon := setupStartProject(t, &fakeDaemon{Jobs: elsewhere})
	fakeTTY(t, false)

	if _, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagJob, "api", "-y", "--"+domain.FlagNoProbe); err != nil {
		t.Fatalf("run start -y --no-probe: %v", err)
	}

	if got := daemon.stoppedWorktrees(); len(got) != 0 {
		t.Errorf("stopped %v, want nothing stopped without --exclusive", got)
	}
}

func TestRunStartRefusesExclusiveWithParallel(t *testing.T) {
	setupStartProject(t, &fakeDaemon{})

	_, _, err := runCmd(t, domain.CmdStart, "--"+domain.FlagJob, "api", "--"+domain.FlagExclusive, "--"+domain.FlagParallel)
	if err == nil {
		t.Fatal("--exclusive and --parallel were both accepted")
	}
}
