package cli

import (
	"context"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runlogs"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

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
