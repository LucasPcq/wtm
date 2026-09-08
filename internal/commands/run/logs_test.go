package run

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

func linesCmd() (*cobra.Command, *bytes.Buffer, *bytes.Buffer) {
	cmd := &cobra.Command{}
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	return cmd, &out, &errOut
}

func fedStream(t *testing.T, chunks ...string) *runlogstest.Stream {
	t.Helper()

	stream := runlogstest.NewStream()
	for _, chunk := range chunks {
		stream.Feed([]byte(chunk))
	}
	// The reader stops at the end of the output, which is what lets the command
	// return instead of waiting on a job that never ends.
	stream.Close()
	return stream
}

func TestWriteJobLinesPrefixesEveryJob(t *testing.T) {
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views: []runlogs.JobView{
			{Name: "api", Kind: domain.JobKindService, Status: domain.JobStatusRunning, Attachable: true},
			{Name: "web", Kind: domain.JobKindService, Status: domain.JobStatusRunning, Attachable: true},
		},
		Streams: map[string]runlogs.Stream{
			"api": fedStream(t, "listening on 3000\n"),
			"web": fedStream(t, "ready in 412ms\n"),
		},
	})

	cmd, out, _ := linesCmd()
	if err := writeJobLines(jobLinesParams{Cmd: cmd, Board: board}); err != nil {
		t.Fatalf("writeJobLines: %v", err)
	}

	for _, want := range []string{"[api] listening on 3000", "[web] ready in 412ms"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("stdout is missing %q\n--- stdout ---\n%s", want, out.String())
		}
	}
}

// Every line carries its prefix, not just the first one of a chunk: without it
// a job printing a page is indistinguishable from the job beside it.
func TestWriteJobLinesPrefixesEveryLineOfAChunk(t *testing.T) {
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views:   []runlogs.JobView{{Name: "api", Status: domain.JobStatusRunning, Attachable: true}},
		Streams: map[string]runlogs.Stream{"api": fedStream(t, "first\nsecond\n", "third\n")},
	})

	cmd, out, _ := linesCmd()
	if err := writeJobLines(jobLinesParams{Cmd: cmd, Board: board}); err != nil {
		t.Fatalf("writeJobLines: %v", err)
	}

	if got := strings.Count(out.String(), "[api]"); got != 3 {
		t.Errorf("counted %d prefixes for 3 lines\n--- stdout ---\n%s", got, out.String())
	}
}

// A job that is not running has no stream to attach to, and its log file is the
// only place its output is left.
func TestWriteJobLinesReadsAStoppedJobBackFromItsLogFile(t *testing.T) {
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views: []runlogs.JobView{{Name: "migrate", Kind: domain.JobKindTask, Status: domain.JobStatusStopped}},
		Lines: map[string][]string{"migrate": {"applying 001", "done"}},
	})

	cmd, out, _ := linesCmd()
	if err := writeJobLines(jobLinesParams{Cmd: cmd, Board: board, Job: "migrate"}); err != nil {
		t.Fatalf("writeJobLines: %v", err)
	}

	for _, want := range []string{"[migrate] applying 001", "[migrate] done"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("stdout is missing %q\n--- stdout ---\n%s", want, out.String())
		}
	}
}

func TestWriteJobLinesNarrowsToTheNamedJob(t *testing.T) {
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views: []runlogs.JobView{
			{Name: "api", Status: domain.JobStatusRunning, Attachable: true},
			{Name: "web", Status: domain.JobStatusRunning, Attachable: true},
		},
		Streams: map[string]runlogs.Stream{
			"api": fedStream(t, "listening on 3000\n"),
			"web": fedStream(t, "ready in 412ms\n"),
		},
	})

	cmd, out, _ := linesCmd()
	if err := writeJobLines(jobLinesParams{Cmd: cmd, Board: board, Job: "api"}); err != nil {
		t.Fatalf("writeJobLines: %v", err)
	}

	if strings.Contains(out.String(), "web") {
		t.Errorf("naming a job did not narrow the output:\n%s", out.String())
	}
	if !strings.Contains(out.String(), "[api] listening on 3000") {
		t.Errorf("stdout is missing the named job's line:\n%s", out.String())
	}
}

func TestWriteJobLinesRefusesAJobTheWorktreeDoesNotHave(t *testing.T) {
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views: []runlogs.JobView{{Name: "api", Status: domain.JobStatusRunning, Attachable: true}},
	})

	cmd, _, _ := linesCmd()
	err := writeJobLines(jobLinesParams{Cmd: cmd, Board: board, Job: "ghost"})
	if err == nil || !strings.Contains(err.Error(), "ghost") {
		t.Fatalf("err = %v, want one naming the unknown job", err)
	}
}

func TestWriteJobLinesSaysSoWhenNothingIsRunning(t *testing.T) {
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views: []runlogs.JobView{{Name: "api", Status: domain.JobStatusStopped}},
	})

	cmd, out, _ := linesCmd()
	if err := writeJobLines(jobLinesParams{Cmd: cmd, Board: board}); err != nil {
		t.Fatalf("writeJobLines: %v", err)
	}

	if !strings.Contains(out.String(), domain.RunLogsNoJobs) {
		t.Errorf("stdout does not say the worktree has nothing running:\n%s", out.String())
	}
}

func TestRunLogsOpensTheViewOnATerminal(t *testing.T) {
	setupStartProject(t, &fakeDaemon{})
	view := captureRunView(t)
	fakeTTY(t, true)

	if _, _, err := runCmd(t, domain.CmdLogs, "--"+domain.FlagJob, "api"); err != nil {
		t.Fatalf("run logs api: %v", err)
	}

	call := view.only(t)
	if call.Job != "api" {
		t.Errorf("the view opened on %q, want api", call.Job)
	}
	// `run logs` reports on what is already running: it starts nothing, so it
	// has no recap to print on the way out.
	if call.Attached {
		t.Error("run logs handed the view a start sequence")
	}
}

func TestRunLogsWithoutATerminalWritesPrefixedLines(t *testing.T) {
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	// The daemon keys a job on the worktree it was started from, never on the
	// subdirectory or the spelling the caller happened to use, so the fake has to
	// answer with the same key the command will ask for.
	root, err := infra.Toplevel(dir)
	if err != nil {
		t.Fatalf("toplevel: %v", err)
	}

	setupStartProject(t, &fakeDaemon{
		Jobs: []domain.JobInfo{
			{Name: "api", Kind: domain.JobKindService, Status: domain.JobStatusRunning, WorkDir: root},
		},
		Streams: map[string][]byte{"api": []byte("listening on 3000\nrequest handled\n")},
	})
	view := captureRunView(t)
	fakeTTY(t, false)

	stdout, _, err := runCmd(t, domain.CmdLogs)
	if err != nil {
		t.Fatalf("run logs: %v", err)
	}

	if len(view.calls) != 0 {
		t.Fatalf("a pipe opened the view: %+v", view.calls)
	}
	for _, want := range []string{"[api] listening on 3000", "[api] request handled"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout is missing %q\n--- stdout ---\n%s", want, stdout)
		}
	}
}

func TestWriteJobLogsJSONReplaysEveryJobsHistory(t *testing.T) {
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views: []runlogs.JobView{
			{Name: "api", Kind: domain.JobKindService, Status: domain.JobStatusRunning, Attachable: true},
			{Name: "migrate", Kind: domain.JobKindTask, Status: domain.JobStatusStopped},
		},
		Lines: map[string][]string{
			"api":     {"2026-09-02T10:04:11Z  listening on 3000"},
			"migrate": {"2026-09-02T10:03:58Z  applying 001"},
		},
	})

	cmd, out, _ := linesCmd()
	if err := writeJobLogsJSON(jobLinesParams{Cmd: cmd, Board: board}); err != nil {
		t.Fatalf("writeJobLogsJSON: %v", err)
	}

	var entries []domain.JobLogEntry
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, out.String())
	}
	want := []domain.JobLogEntry{
		{Job: "api", At: "2026-09-02T10:04:11Z", Text: "listening on 3000"},
		{Job: "migrate", At: "2026-09-02T10:03:58Z", Text: "applying 001"},
	}
	if !reflect.DeepEqual(entries, want) {
		t.Errorf("entries = %+v, want %+v", entries, want)
	}
}

func TestWriteJobLogsJSONNeverAttaches(t *testing.T) {
	service := &runlogstest.Service{
		Infos: []domain.JobInfo{{Name: "api", Status: domain.JobStatusRunning}},
		Lines: map[string][]string{"api": {"2026-09-02T10:04:11Z  listening on 3000"}},
	}
	board := runlogs.NewBoard(runlogs.BoardParams{
		Service: service,
		Jobs:    []domain.JobConfig{{Name: "api", Kind: domain.JobKindService}},
		WorkDir: "/wt",
		LogDir:  "/state/logs/wt",
		Logged:  map[string]bool{"api": true},
	})

	cmd, out, _ := linesCmd()
	if err := writeJobLogsJSON(jobLinesParams{Cmd: cmd, Board: board}); err != nil {
		t.Fatalf("writeJobLogsJSON: %v", err)
	}

	if len(service.Attached) != 0 {
		t.Errorf("the JSON surface attached to %+v", service.Attached)
	}
	if !strings.Contains(out.String(), "listening on 3000") {
		t.Errorf("the running job was not replayed:\n%s", out.String())
	}
}

// What makes this a list is the stream, not the arity of the target.
func TestWriteJobLogsJSONStaysAnArrayForOneJob(t *testing.T) {
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views: []runlogs.JobView{
			{Name: "api", Status: domain.JobStatusRunning, Attachable: true},
			{Name: "web", Status: domain.JobStatusRunning, Attachable: true},
		},
		Lines: map[string][]string{
			"api": {"2026-09-02T10:04:11Z  listening on 3000"},
			"web": {"2026-09-02T10:04:12Z  ready in 412ms"},
		},
	})

	cmd, out, _ := linesCmd()
	if err := writeJobLogsJSON(jobLinesParams{Cmd: cmd, Board: board, Job: "api"}); err != nil {
		t.Fatalf("writeJobLogsJSON: %v", err)
	}

	var entries []domain.JobLogEntry
	if err := json.Unmarshal(out.Bytes(), &entries); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, out.String())
	}
	if len(entries) != 1 || entries[0].Job != "api" {
		t.Errorf("entries = %+v, want api's single line", entries)
	}
}

func TestWriteJobLogsJSONOnAWorktreeWithNothingRecordedIsAnEmptyArray(t *testing.T) {
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views: []runlogs.JobView{{Name: "api", Status: domain.JobStatusStopped}},
	})

	cmd, out, _ := linesCmd()
	if err := writeJobLogsJSON(jobLinesParams{Cmd: cmd, Board: board}); err != nil {
		t.Fatalf("writeJobLogsJSON: %v", err)
	}

	if got := strings.TrimSpace(out.String()); got != "[]" {
		t.Errorf("stdout = %q, want an empty array", got)
	}
}

func TestRunLogsJSONNeverOpensTheView(t *testing.T) {
	stateDir := setupTestProject(t)
	// The command resolves its worktree from the working directory, and this one
	// needs a branch to name a log directory with. Standing in the repo the setup
	// just built keeps that off the ambient checkout, which on CI is a detached
	// HEAD with no branch to read.
	t.Chdir(filepath.Dir(filepath.Dir(stateDir)))

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	root, err := infra.Toplevel(cwd)
	if err != nil {
		t.Fatalf("toplevel: %v", err)
	}

	writeRunTOML(t, stateDir, domain.RunConfig{Jobs: []domain.JobConfig{apiJob}})
	startFakeDaemon(t, &fakeDaemon{
		Jobs: []domain.JobInfo{
			{Name: "api", Kind: domain.JobKindService, Status: domain.JobStatusRunning, WorkDir: root},
		},
	})

	// Tail reads the file itself rather than asking the daemon, so the history
	// this replays has to be on disk where the command will look for it.
	logDir := seam.LogDir(seam.LogDirParams{StateDir: stateDir, WorkDir: root})
	if logDir == "" {
		t.Fatal("the test worktree resolved to no log dir")
	}
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("create log dir: %v", err)
	}
	logPath := filepath.Join(logDir, rules.JobLogFileName("api"))
	if err := os.WriteFile(logPath, []byte("2026-09-02T10:04:11Z  listening on 3000\n"), 0o644); err != nil {
		t.Fatalf("write job log: %v", err)
	}

	view := captureRunView(t)
	fakeTTY(t, true)

	stdout, _, err := runCmd(t, domain.CmdLogs, "--output", domain.OutputJSON)
	if err != nil {
		t.Fatalf("run logs --output json: %v", err)
	}

	if len(view.calls) != 0 {
		t.Fatalf("--output json opened the view: %+v", view.calls)
	}
	var entries []domain.JobLogEntry
	if err := json.Unmarshal([]byte(stdout), &entries); err != nil {
		t.Fatalf("parse JSON: %v\noutput: %s", err, stdout)
	}
	want := []domain.JobLogEntry{{Job: "api", At: "2026-09-02T10:04:11Z", Text: "listening on 3000"}}
	if !reflect.DeepEqual(entries, want) {
		t.Errorf("entries = %+v, want %+v", entries, want)
	}
}
