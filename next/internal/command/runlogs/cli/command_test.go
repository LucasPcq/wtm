package cli

import (
	"context"
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runlogs"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfRunLogsSaysWhereTheLinesGo(t *testing.T) {
	got := runLogs(t, newWorld(), "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "logs_help", got.Stdout)
}

func TestWithoutATerminalEachJobsLastLinesArePrintedThenItEnds(t *testing.T) {
	got := runLogs(t, newWorld())

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "lines", got.Stdout)
}

func TestOverSeveralWorktreesEachLineNamesItsWorktree(t *testing.T) {
	got := runLogs(t, newWorld(), featA, featB)

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "lines_several", got.Stdout)
}

func TestAJobNamedThatNeverRanSaysSo(t *testing.T) {
	got := runLogs(t, newWorld(), featA, "--job", "never")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "never_ran", got.Stdout)
}

func TestFollowPrintsTheLastLinesThenTheNewOnesUnframed(t *testing.T) {
	got := runLogs(t, newWorld(), featA, "--job", "api", "--follow")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "followed", got.Stdout)
}

func TestFollowInJSONWritesALineOfJSONPerLine(t *testing.T) {
	got := runLogs(t, newWorld(), featA, "--job", "api", "-f", "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "followed_json", got.Stdout)
}

func TestAnInterruptedFollowExitsZero(t *testing.T) {
	w := newWorld()
	ctx, interrupt := context.WithCancel(t.Context())
	w.daemon.wait = interrupt

	got := runLogsIn(ctx, w, featA, "--job", "api", "-f")

	assert.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, "GET /users 200")
}

func TestEachEnvelopeOfRunLogsHasItsGolden(t *testing.T) {
	for name, c := range map[string]struct {
		Words []string
		Code  int
	}{
		"read":        {Words: []string{featA}, Code: uicli.ExitOK},
		"nothing_ran": {Words: []string{featB}, Code: uicli.ExitOK},
		"job_unknown": {Words: []string{featA, "--job", "ghost"}, Code: uicli.ExitUsage},
		"not_found":   {Words: []string{"nope"}, Code: uicli.ExitUsage},
	} {
		t.Run(name, func(t *testing.T) {
			got := runLogs(t, newWorld(), append([]string{"--output", "json"}, c.Words...)...)

			assert.Equal(t, c.Code, got.Code, got.Stdout)
			assert.Empty(t, got.Stderr)
			formtest.Golden(t, "envelope_"+name, got.Stdout)
		})
	}
}

func TestAtATerminalTheWorktreesArePickedFirst(t *testing.T) {
	got := runLogsAt(t, newWorld(), "\x1b")

	assert.Equal(t, uicli.ExitCancelled, got.Code, "leaving the picker cancels the read")
	assert.False(t, viewOpened(got))
}

func TestThePickerOffersEveryWorktreeTheOneYouStandInPicked(t *testing.T) {
	w := newWorld()
	app, err := w.open(t.Context(), mainPath)
	require.NoError(t, err)
	q, err := app.Questions(t.Context(), runlogs.Request{Caller: w.env().Caller()})
	require.NoError(t, err)
	field, ok := q.Form().Fields[0].(domain.MultiSelectField)
	require.True(t, ok)
	subjects := &form.Subjects{Field: field, Words: words(q, field)}

	d := formtest.Start(form.New(t.Context(), form.SubjectScript(subjects)), tea.WindowSizeMsg{Width: 100, Height: 30})
	formtest.Golden(t, "picker", d.Screen())
	d.Keys("down", "down", "space", "enter")

	assert.Equal(t, form.Finished, d.Wizard.Ending())
	assert.Equal(t, []string{featA, featB}, subjects.Picked())
}

func TestAtATerminalYesSkipsThePickerForTheWorktreeYouStandIn(t *testing.T) {
	got := runLogsAt(t, newWorld(), "q", "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.NotContains(t, got.Stderr, "Select")
	assert.True(t, viewOpened(got))
}

func TestAtATerminalAWorktreeWhereNothingRanOpensNoView(t *testing.T) {
	got := runLogsAt(t, newWorld(), "", featB)

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.False(t, viewOpened(got))
	assert.Contains(t, got.Stdout, "feat/b: no job has run")
}

func TestAJobOnlyItsLogKnowsIsShownStoppedWithTheLinesRead(t *testing.T) {
	streams, _, _, _ := output.Test()
	c := &logsCommand{io: uicli.Run{Streams: streams}, view: View{Daemon: newWorld().daemon}}
	read := runlogs.Outcome{Items: []runlogs.Item{{Subject: featA, Status: domain.StatusDone, Detail: &runlogs.Detail{Path: pathOf(featA), Jobs: []runlogs.Job{
		{Name: "migrate", Kind: project.Task, State: runlogs.JobStopped, Lines: []runlogs.Line{{At: at, Text: "migrated"}}},
	}}}}}

	params, ran := c.viewOf(read, runlogs.Request{})
	jobs, err := params.Daemon.Jobs(t.Context())
	require.NoError(t, err)
	var lines []string
	listed, err := params.Daemon.Lines(t.Context(), run.Tail{Name: "migrate", WorkDir: pathOf(featA)}, func(line run.Line) { lines = append(lines, line.Text) })

	assert.True(t, ran)
	assert.Contains(t, jobs, run.Job{Name: "migrate", WorkDir: pathOf(featA), Kind: project.Task, State: run.Stopped}, "the daemon no longer lists it: listed stopped, not not started")
	require.NoError(t, err)
	assert.Equal(t, run.Stopped, listed.State)
	assert.Equal(t, []string{"migrated"}, lines, "its pane holds the lines read from its log")
}

// The review of #268: under --follow, a worktree where nothing ran and a job named that never ran printed nothing (B6, D6).
func TestFollowSaysWhatNeverRanAsTheReadDoes(t *testing.T) {
	nothing := runLogs(t, newWorld(), featB, "--follow")
	never := runLogs(t, newWorld(), featA, "--job", "never", "--follow")

	require.Equal(t, uicli.ExitOK, nothing.Code, nothing.Stderr)
	assert.Contains(t, nothing.Stdout, "= feat/b: no job has run")
	require.Equal(t, uicli.ExitOK, never.Code, never.Stderr)
	assert.Contains(t, never.Stdout, "= feat/a: never has not run")
}

// The review of #268: a quiet job, then Ctrl+C, exited 130 (D3: an interrupted follow exits 0).
func TestAFollowInterruptedBeforeItsFirstLineExitsZero(t *testing.T) {
	w := newWorld()
	ctx, interrupt := context.WithCancel(t.Context())
	w.daemon.wait = interrupt

	got := runLogsIn(ctx, w, featA, "--job", "web", "-f")

	assert.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.Empty(t, got.Stdout)
}

// The review of #268: the legacy daemon keeps no lines of its jobs, and its history pane showed an error though the read held them (D8).
func TestTheViewShowsTheLinesReadOfAJobTheDaemonHasNoLinesOf(t *testing.T) {
	streams, _, _, _ := output.Test()
	legacy := newWorld().daemon
	legacy.refuse = errors.New("unknown action lines")
	c := &logsCommand{io: uicli.Run{Streams: streams}, view: View{Daemon: legacy}}
	read := runlogs.Outcome{Items: []runlogs.Item{{Subject: featA, Status: domain.StatusDone, Detail: &runlogs.Detail{Path: pathOf(featA), Jobs: []runlogs.Job{
		{Name: "migrate", Kind: project.Task, State: runlogs.JobExited, Hold: run.HoldOwn, Lines: []runlogs.Line{{At: at, Text: "migrated"}}},
	}}}}}

	params, _ := c.viewOf(read, runlogs.Request{})
	var lines []string
	listed, err := params.Daemon.Lines(t.Context(), run.Tail{Name: "migrate", WorkDir: pathOf(featA)}, func(line run.Line) { lines = append(lines, line.Text) })

	require.NoError(t, err)
	assert.Equal(t, []string{"migrated"}, lines)
	assert.Equal(t, run.Exited, listed.State)
}

func TestAFollowInJSONTypesEachLineItWrites(t *testing.T) {
	w := newWorld()
	w.daemon.skewed = &run.Skewed{Skew: run.SkewBuild, Daemon: "0.30.9"}

	got := runLogs(t, w, featA, "--job", "api", "-f", "--output", "json")
	nothing := runLogs(t, newWorld(), featB, "-f", "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.Empty(t, got.Stderr, "a warning is a line of the stream, never text on stderr")
	formtest.Golden(t, "followed_json_typed", got.Stdout)
	assert.JSONEq(t, `{"type":"absent","worktree":"feat/b"}`, nothing.Stdout)
}

// The review of #268: a failure once lines were written ended a JSON stream on a human frame on stderr.
func TestAFailureAfterTheFirstLineEndsAJSONStreamOnAnErrorLine(t *testing.T) {
	streams, _, stdout, stderr := output.Test()
	c := &logsCommand{io: uicli.Run{Command: runlogs.Name, Streams: streams, Output: uicli.OutputJSON}, started: true}

	err := c.fail(&domain.Error{Kind: domain.KindInternal, Code: "run-logs.broken", Message: "the daemon went away"})

	assert.Error(t, err)
	assert.Empty(t, stderr.String())
	assert.JSONEq(t, `{"type":"error","error":{"kind":"internal","code":"run-logs.broken","message":"the daemon went away"}}`, stdout.String())
}

// The review of #268: the view repeated a job's lines when the daemon broke off after sending them.
func TestTheViewNeverRepeatsLinesTheDaemonSentBeforeItBrokeOff(t *testing.T) {
	streams, _, _, _ := output.Test()
	daemon := newWorld().daemon
	daemon.broke = errors.New("connection reset")
	c := &logsCommand{io: uicli.Run{Streams: streams}, view: View{Daemon: daemon}}
	read := runlogs.Outcome{Items: []runlogs.Item{{Subject: featA, Status: domain.StatusDone, Detail: &runlogs.Detail{Path: pathOf(featA), Jobs: []runlogs.Job{
		{Name: "api", Kind: project.Service, State: runlogs.JobRunning, Hold: run.HoldOwn, Lines: []runlogs.Line{{At: at, Text: "api ready"}}},
	}}}}}

	params, _ := c.viewOf(read, runlogs.Request{})
	var lines []string
	_, err := params.Daemon.Lines(t.Context(), run.Tail{Name: "api", WorkDir: pathOf(featA)}, func(line run.Line) { lines = append(lines, line.Text) })

	assert.Error(t, err)
	assert.Equal(t, []string{"api ready"}, lines, "the daemon's lines alone, never the read's after them")
}

func TestASearchReadsTheWholeLogForTheLinesItMatches(t *testing.T) {
	got := runLogs(t, newWorld(), featA, "--search", "MIGRAT")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "searched", got.Stdout)
}
