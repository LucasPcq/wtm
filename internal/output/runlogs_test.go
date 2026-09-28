package output

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

func emit(events ...runlogs.Event) (stdout string, stderr string) {
	var out, errOut bytes.Buffer
	printer := NewRunPrinter(RunPrinterParams{Out: &out, Err: &errOut})
	for _, event := range events {
		printer.Emit(event)
	}
	printer.Conclude(nil)
	return out.String(), errOut.String()
}

func TestRunPrinterReportsEachStepOfTheSequence(t *testing.T) {
	stdout, _ := emit(
		runlogs.Event{Phase: runlogs.PhaseStarting, Job: "migrate", Step: 1, Steps: 2},
		runlogs.Event{Phase: runlogs.PhaseOutput, Job: "migrate", Chunk: []byte("applying 001\n")},
		runlogs.Event{Phase: runlogs.PhaseDone, Job: "migrate", Step: 1, Steps: 2},
		runlogs.Event{Phase: runlogs.PhaseStarting, Job: "api", Step: 2, Steps: 2},
		runlogs.Event{Phase: runlogs.PhaseStarted, Job: "api", Step: 2, Steps: 2},
		runlogs.Event{Phase: runlogs.PhaseReady, Outcome: runlogs.Outcome{Started: []string{"api"}, Completed: []string{"migrate"}}},
	)

	for _, want := range []string{"[1/2] migrate", "applying 001", "migrate done", "[2/2] api", "api started", domain.RunStreamAttachHint} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout is missing %q\n--- stdout ---\n%s", want, stdout)
		}
	}
}

func TestRunPrinterMarksARepeatStartAsAlreadyRunning(t *testing.T) {
	stdout, _ := emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "api", AlreadyRunning: true})

	if !strings.Contains(stdout, "api already running") {
		t.Errorf("stdout does not say the job was already up:\n%s", stdout)
	}
	if strings.Contains(stdout, "api started") {
		t.Errorf("a repeat start read as a fresh one:\n%s", stdout)
	}
}

// Nothing that failed goes to stdout: a caller piping a run's output reads what
// it produced, and the report on what it left behind belongs on stderr.
func TestRunPrinterReportsAnAbortOnStderr(t *testing.T) {
	outcome := runlogs.Outcome{
		Started:    []string{"docker"},
		Failed:     "migrate",
		FailedStep: 2,
		Steps:      3,
		NotStarted: []string{"api"},
	}

	stdout, stderr := emit(
		runlogs.Event{Phase: runlogs.PhaseFailed, Job: "migrate", Reason: "task migrate failed: exit status 1"},
		runlogs.Event{Phase: runlogs.PhaseAborted, Job: "migrate", Outcome: outcome},
	)

	for _, want := range []string{"task migrate failed", "step 2/3", "migrate", "Left running", "docker", "Not started", "api", "run down"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("the abort report is missing %q\n--- stderr ---\n%s", want, stderr)
		}
	}
	if stdout != "" {
		t.Errorf("an aborted run wrote to stdout:\n%s", stdout)
	}
}

func TestRunPrinterSaysNothingAboutWhatDidNotHappen(t *testing.T) {
	outcome := runlogs.Outcome{Failed: "migrate", FailedStep: 2, Steps: 2}
	_, stderr := emit(runlogs.Event{Phase: runlogs.PhaseAborted, Job: "migrate", Outcome: outcome})

	if strings.Contains(stderr, domain.RunAbortRunningLabel) {
		t.Errorf("the report claims something was left running:\n%s", stderr)
	}
	if strings.Contains(stderr, domain.RunAbortNotStartedLabel) {
		t.Errorf("the report claims a job was never reached when the last one failed:\n%s", stderr)
	}
}

func TestRunPrinterHintsAtNothingWhenNothingIsRunning(t *testing.T) {
	stdout, _ := emit(runlogs.Event{Phase: runlogs.PhaseReady, Outcome: runlogs.Outcome{Completed: []string{"migrate"}}})

	if strings.Contains(stdout, domain.RunStreamAttachHint) {
		t.Errorf("a run that left nothing up still offered `run down`:\n%s", stdout)
	}
}

// The reason a machine consumer is given the failing job's output at all: it
// never saw the live stream, and the daemon's message alone does not say why.
func TestWriteRunOutcomeJSONCarriesTheFailedJobsOutput(t *testing.T) {
	code := 1
	outcome := runlogs.Outcome{
		Results: []domain.JobActionResult{
			{Name: "docker", Status: domain.JobActionStarted},
			{Name: "migrate", Status: domain.JobActionError, Message: "task migrate failed: exit status 1"},
		},
		Failed:         "migrate",
		FailedStep:     2,
		Steps:          3,
		FailedOutput:   []byte("applying 001\nERROR: relation \"users\" does not exist\n"),
		FailedExitCode: &code,
	}

	var buf bytes.Buffer
	if err := WriteRunOutcomeJSON(&buf, outcome); err != nil {
		t.Fatalf("WriteRunOutcomeJSON: %v", err)
	}

	var results []domain.JobActionResult
	if err := json.Unmarshal(buf.Bytes(), &results); err != nil {
		t.Fatalf("parse JSON: %v\n%s", err, buf.String())
	}
	if len(results) != 2 {
		t.Fatalf("got %d results, want the two the run produced", len(results))
	}

	failed := results[1]
	if !strings.Contains(failed.Output, "does not exist") {
		t.Errorf("output = %q, want the lines the job wrote before it failed", failed.Output)
	}
	if failed.ExitCode == nil || *failed.ExitCode != 1 {
		t.Errorf("exit_code = %v, want 1", failed.ExitCode)
	}
	// The output is its own field: folding it into the message was how it used
	// to travel, and it made the message unreadable as a reason.
	if strings.Contains(failed.Message, "does not exist") {
		t.Errorf("message = %q, want the concise reason only", failed.Message)
	}

	if started := results[0]; started.Output != "" || started.ExitCode != nil {
		t.Errorf("a job that did not fail carries %+v", started)
	}
}

func TestWriteRunOutcomeJSONLeavesASuccessfulRunAlone(t *testing.T) {
	outcome := runlogs.Outcome{
		Results: []domain.JobActionResult{{Name: "api", Status: domain.JobActionStarted}},
		Started: []string{"api"},
	}

	var buf bytes.Buffer
	if err := WriteRunOutcomeJSON(&buf, outcome); err != nil {
		t.Fatalf("WriteRunOutcomeJSON: %v", err)
	}
	if strings.Contains(buf.String(), "output") || strings.Contains(buf.String(), "exit_code") {
		t.Errorf("a successful run carries failure fields:\n%s", buf.String())
	}
}

// N sequences interleave on one stream, so every line has to say where it came
// from — two jobs called `web` are otherwise the same line twice.
func TestRunPrinterNamesTheWorktreeAboveSeveralOfThem(t *testing.T) {
	var out, errOut bytes.Buffer
	printer := NewRunPrinter(RunPrinterParams{
		Out: &out, Err: &errOut,
		Profile:   "dev",
		Worktrees: []string{"main", "feature"},
	})

	printer.Emit(runlogs.Event{Phase: runlogs.PhaseStarting, Job: "web", Worktree: "main", Step: 1, Steps: 1})
	printer.Emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "web", Worktree: "main", Step: 1, Steps: 1})
	printer.Emit(runlogs.Event{Phase: runlogs.PhaseStarting, Job: "web", Worktree: "feature", Step: 1, Steps: 1})
	printer.Emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "web", Worktree: "feature", Step: 1, Steps: 1})

	stdout := out.String()
	for _, want := range []string{"2 worktrees", "[1/1] web · main", "web started · main", "[1/1] web · feature", "web started · feature"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout is missing %q\n--- stdout ---\n%s", want, stdout)
		}
	}
}

// Above a single worktree, naming it would only repeat what the command was told.
func TestRunPrinterSaysNothingAboutASingleWorktree(t *testing.T) {
	var out, errOut bytes.Buffer
	printer := NewRunPrinter(RunPrinterParams{Out: &out, Err: &errOut, Worktrees: []string{"main"}})

	printer.Emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "web", Worktree: "main", Step: 1, Steps: 1})

	if strings.Contains(out.String(), "main") {
		t.Errorf("stdout named the only worktree there is:\n%s", out.String())
	}
}

// The hint is about the run, not about any one worktree, so N of them reporting
// ready must not print it N times.
func TestRunPrinterClosesTheRunOnce(t *testing.T) {
	var out, errOut bytes.Buffer
	printer := NewRunPrinter(RunPrinterParams{Out: &out, Err: &errOut, Worktrees: []string{"main", "feature"}})

	printer.Emit(runlogs.Event{Phase: runlogs.PhaseReady, Outcome: runlogs.Outcome{Worktree: "main", Started: []string{"web"}}})
	printer.Emit(runlogs.Event{Phase: runlogs.PhaseReady, Outcome: runlogs.Outcome{Worktree: "feature", Started: []string{"web"}}})
	printer.Conclude(nil)

	if got := strings.Count(out.String(), domain.RunStreamAttachHint); got != 1 {
		t.Errorf("the hint was printed %d times, want once", got)
	}
}

func TestRunPrinterSaysASharedJobWasAttachedAndWhatItCarved(t *testing.T) {
	stdout, _ := emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "postgres", Attached: true, Namespace: "app_feat_x"})

	if !strings.Contains(stdout, "postgres attached") || strings.Contains(stdout, "postgres started") {
		t.Errorf("stdout = %q, want the job attached, not started", stdout)
	}
	if !strings.Contains(stdout, "postgres attached · app_feat_x ready") {
		t.Errorf("stdout = %q, want the namespace on the job's own line", stdout)
	}
}

// The run concludes where every job is reached: one block, the URLs a runner
// holds by their apps' names and never under the runner's line, then the .env
// out of step in one line, then what to do next.
func TestRunPrinterConcludesOnWhereToReachIt(t *testing.T) {
	var out, errOut bytes.Buffer
	printer := NewRunPrinter(RunPrinterParams{Out: &out, Err: &errOut})

	printer.Emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "postgres", Ports: map[string]int{"POSTGRES_PORT": 5432}, Namespace: "app_main"})
	printer.Emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "dev", Held: []domain.JobURLEntry{
		{Job: "web", URL: "http://web.main.app.localhost:11080"},
		{Job: "api", URL: "http://api.main.app.localhost:11080"},
	}})
	printer.Emit(runlogs.Event{Phase: runlogs.PhaseReady, Outcome: runlogs.Outcome{Started: []string{"postgres", "dev"}}})
	printer.Conclude([]string{"main's .env still spells ports — `wtm env main` aligns it"})

	stdout := out.String()
	for _, want := range []string{
		"postgres started · :5432 · app_main ready",
		"dev started · 2 urls",
		domain.ReachTitle,
		"web       http://web.main.app.localhost:11080",
		"postgres  :5432 · app_main",
		"wtm env main",
		domain.RunStreamAttachHint,
	} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout is missing %q\n%s", want, stdout)
		}
	}
	if strings.Index(stdout, "wtm env main") > strings.Index(stdout, domain.RunStreamAttachHint) {
		t.Errorf("the .env line came after the hints:\n%s", stdout)
	}
}

// A service this worktree only holds says where it runs, on its line and in a
// section of its own: stopping the worktree leaves it up.
func TestRunPrinterSetsAServiceHeldInMainApart(t *testing.T) {
	var out, errOut bytes.Buffer
	printer := NewRunPrinter(RunPrinterParams{Out: &out, Err: &errOut})

	printer.Emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "compose", Worktree: "feat/x", Ports: map[string]int{"REDIS_PORT": 6389, "MINIO_PORT": 9010}})
	printer.Emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "postgres", Worktree: "feat/x", Attached: true, SharedIn: "main", Ports: map[string]int{"POSTGRES_PORT": 5432}})
	printer.Emit(runlogs.Event{Phase: runlogs.PhaseReady, Outcome: runlogs.Outcome{Started: []string{"compose", "postgres"}}})
	printer.Conclude(nil)

	stdout := out.String()
	if !strings.Contains(stdout, "postgres attached to main · :5432") {
		t.Errorf("stdout = %q, want the line to say where postgres runs", stdout)
	}
	own, shared := strings.Index(stdout, domain.ReachTitle), strings.Index(stdout, "Shared, running in main")
	if own < 0 || shared < own || strings.LastIndex(stdout, "postgres") < shared {
		t.Errorf("stdout = %q, want postgres in a section of its own after the worktree's", stdout)
	}
}
