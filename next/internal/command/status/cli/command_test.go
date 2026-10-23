package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfStatus(t *testing.T) {
	got := runStatus(t, world{}, "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "status_help", got.Stdout)
}

func TestStatusDrawsOneWorktreeHowItRunsThenItsBranchItsJobsAndItsProblems(t *testing.T) {
	got := runStatus(t, world{}, "feat/a", "--with-prs")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "status_text", got.Stdout)
}

func TestWithNoWorktreeNamedAndNoTerminalTheOneYouAreInIsRead(t *testing.T) {
	got := runStatus(t, world{Cwd: "/repo/.trees/feat-old/src"})

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "status_pending", got.Stdout)
}

func TestAllDrawsATableThenTheProblemsOfEachWorktree(t *testing.T) {
	got := runStatus(t, world{}, "--all")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "status_all", got.Stdout)
}

func TestWithoutRunTomlTheTableKeepsOnlyTheEnvColumn(t *testing.T) {
	got := runStatus(t, world{NoRun: true}, "--all")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "status_all_no_run", got.Stdout)
}

func TestWhatCannotBeAskedIsAWarningUnderTheState(t *testing.T) {
	got := runStatus(t, world{JobsErr: run.ErrUnknown, RunErr: errInvalidRun}, "feat/a")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "status_warnings", got.Stdout)
}

func TestTheJSONOfStatusIsTheV2EnvelopeOfARead(t *testing.T) {
	got := runStatus(t, world{}, "feat/a", "--with-prs", "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "envelope_status", got.Stdout)
	doc := formtest.Decoded[struct{}](t, got.Stdout)
	assert.Empty(t, doc.Op)
	assert.Empty(t, doc.Fingerprint)
	assert.Equal(t, status.Name, doc.Command)
}

func TestARefusalWithJSONIsTheEnvelopeOnStdout(t *testing.T) {
	got := runStatus(t, world{ConfigErr: project.ErrConfigNotFound}, "--output", "json")

	assert.Equal(t, uicli.ExitConfigNotFound, got.Code)
	assert.Empty(t, got.Stderr)
	formtest.Golden(t, "envelope_config_not_found", got.Stdout)
}

func TestAllWithAWorktreeNamedIsAUsageError(t *testing.T) {
	got := runStatus(t, world{}, "feat/a", "--all")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Contains(t, got.Stderr, "--all reads every worktree")
}

func TestANameNoWorktreeAnswersIsAUsageError(t *testing.T) {
	got := runStatus(t, world{}, "nope")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Contains(t, got.Stderr, "nope names no worktree")
}

func TestOnATerminalTheWorktreeIsPickedFirstTheCursorOnTheOneYouAreIn(t *testing.T) {
	got := runOn(t, terminal, world{Cwd: "/repo/.trees/feat-old"})

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, "feat/old — 1 problem")
}

func TestThePickerOffersEveryWorktreeADetachedOneSaidSo(t *testing.T) {
	app, err := world{}.open(t.Context(), mainPath)
	require.NoError(t, err)
	q, err := app.Questions(t.Context(), status.Request{Caller: domain.Caller{Cwd: "/repo/.trees/feat-a"}})
	require.NoError(t, err)
	field, ok := q.Form().Fields[0].(domain.SelectField)
	require.True(t, ok)
	subject := &form.Subject{Field: field}

	d := formtest.Start(form.New(t.Context(), form.SubjectScript(subject)), tea.WindowSizeMsg{Width: 100, Height: 30})
	formtest.Golden(t, "picker", d.Screen())
	d.Keys("esc")

	assert.Equal(t, form.Cancelled, d.Wizard.Ending())
}

func TestAWorktreeThatCannotBeReadIsDrawnAsItsErrorAlone(t *testing.T) {
	got := runStatus(t, world{Broken: true}, "feat/a")

	assert.Equal(t, uicli.ExitError, got.Code)
	formtest.Golden(t, "status_unread", got.Stdout)
}

func TestAllCountsTheWorktreesNotReadAndNeverSaysNothingToFixThen(t *testing.T) {
	got := runStatus(t, world{Broken: true, NoRun: true}, "--all")

	assert.Equal(t, uicli.ExitError, got.Code)
	formtest.Golden(t, "status_all_unread", got.Stdout)
}

func TestAWorktreeWhoseDirectoryIsGoneIsMissingAndToldToBeCleaned(t *testing.T) {
	one := runStatus(t, world{Gone: true}, "feat/old")
	all := runStatus(t, world{Gone: true, NoRun: true}, "--all")
	json := runStatus(t, world{Gone: true, NoRun: true}, "feat/old", "--output", "json")

	require.Equal(t, uicli.ExitOK, one.Code, one.Stderr)
	require.Equal(t, uicli.ExitOK, all.Code, all.Stderr)
	formtest.Golden(t, "status_gone", one.Stdout)
	formtest.Golden(t, "status_all_gone", all.Stdout)
	formtest.Golden(t, "envelope_status_gone", json.Stdout)
}

func TestAllDrawsTheTableEvenOfOneWorktree(t *testing.T) {
	got := runStatus(t, world{Alone: true, NoRun: true}, "--all")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "status_all_one", got.Stdout)
}

func TestAllWithPRsAddsAReviewColumn(t *testing.T) {
	got := runStatus(t, world{}, "--all", "--with-prs")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "status_all_with_prs", got.Stdout)
}

func TestAMergedPullRequestIsShortAndItsCleanComesFirst(t *testing.T) {
	text := runStatus(t, world{Merged: true}, "feat/a", "--with-prs")
	json := runStatus(t, world{Merged: true}, "feat/a", "--with-prs", "--output", "json")
	all := runStatus(t, world{Merged: true}, "--all", "--with-prs")

	require.Equal(t, uicli.ExitOK, text.Code, text.Stderr)
	formtest.Golden(t, "status_merged", text.Stdout)
	formtest.Golden(t, "envelope_status_merged", json.Stdout)
	formtest.Golden(t, "status_all_merged", all.Stdout)
}
