package cli

import (
	"context"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runup"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfRunUpSaysWhatItStarts(t *testing.T) {
	got := runUp(t, newWorld(), "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "up_help", got.Stdout)
}

func TestAStartInTheWorktreeYouStandInSaysWhereEachJobAnswers(t *testing.T) {
	got := runUp(t, newWorld(), "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "started", got.Stdout)
	formtest.Golden(t, "started_progress", got.Stderr)
}

func TestAStartThatFailsInOneWorktreeShowsTheJobsLastLinesAndTheLineThatStartsItAgain(t *testing.T) {
	got := runUp(t, newWorld(), featA, featB, "--yes")

	assert.Equal(t, uicli.ExitError, got.Code)
	formtest.Golden(t, "failed", got.Stdout)
}

func TestAStartInSeveralWorktreesListsTheAddressesUnderEachBranch(t *testing.T) {
	got := runUp(t, newWorld(), featA, featC, "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "started_several", got.Stdout)
}

func TestADryRunShowsThePlanAndTheLineThatAppliesIt(t *testing.T) {
	got := runUp(t, newWorld(), featA, "--dry-run")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "dryrun", got.Stdout)
}

func TestAProfileRunTomlDoesNotDeclareIsAProblemOfTheFlag(t *testing.T) {
	got := runUp(t, newWorld(), featA, "--profile", "ghost", "--yes")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	formtest.Golden(t, "no_profile", got.Stderr)
}

func TestEachEnvelopeOfRunUpHasItsGolden(t *testing.T) {
	for name, c := range map[string]struct {
		Words []string
		Code  int
	}{
		"started":   {Words: []string{featA}, Code: uicli.ExitOK},
		"failed":    {Words: []string{featA, featB}, Code: uicli.ExitError},
		"dry_run":   {Words: []string{featA, "--dry-run"}, Code: uicli.ExitOK},
		"not_found": {Words: []string{"nope"}, Code: uicli.ExitUsage},
		"exclusive": {Words: []string{featA, featB, "--exclusive"}, Code: uicli.ExitUsage},
		"stale":     {Words: []string{featA, "--expect-fingerprint", fingerprint}, Code: uicli.ExitPrecondition},
	} {
		t.Run(name, func(t *testing.T) {
			got := runUp(t, newWorld(), append([]string{"--output", "json", "--yes"}, c.Words...)...)

			assert.Equal(t, c.Code, got.Code, got.Stdout)
			assert.Empty(t, got.Stderr)
			formtest.Golden(t, "envelope_"+name, got.Stdout)
		})
	}
}

func TestAFollowUpOfRunUpIsSpelledAsItsCommandLine(t *testing.T) {
	retry := domain.FollowUp{Command: runup.Name, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{featB}}, {Path: runup.PathProfile, Values: []string{"dev"}}, {Path: runup.PathForce, Values: []string{"true"}}}}

	assert.Equal(t, []string{"wtm-next", "run", "up", featB, "--force", "--profile", "dev"}, Line(retry))
}

func TestAtATerminalTheStartOpensTheLiveViewThenDrawsTheOutcome(t *testing.T) {
	got := runUpAt(t, newWorld(), "q", "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.True(t, viewOpened(got))
	formtest.Golden(t, "started", got.Stdout)
}

// B13: the legacy view's listing cleared an error of the start, which then exited 0.
func TestAStartThatFailedInOneWorktreeExitsOneThroughTheLiveView(t *testing.T) {
	got := runUpAt(t, newWorld(), "q", featA, featB, "--yes")

	assert.Equal(t, uicli.ExitError, got.Code)
	assert.True(t, viewOpened(got))
	formtest.Golden(t, "failed", got.Stdout)
}

// B12: the legacy view, left mid-start, exited 0 on a Ctrl+C; the view now opens on a start done, and leaving it keeps the start's exit.
func TestCtrlCInTheLiveViewLeavesItWithTheStartsExit(t *testing.T) {
	got := runUpAt(t, newWorld(), "\x03", featA, featB, "--yes")

	assert.Equal(t, uicli.ExitError, got.Code)
	assert.True(t, viewOpened(got))
}

// B12: a Ctrl+C during the start, once a job is up, ends the run cancelled (19), no view opened on it.
func TestACtrlCDuringTheStartExitsCancelledWithoutTheView(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	w := newWorld()
	w.daemon.interrupt = cancel

	got := runUpIn(ctx, w, "q", featA, "--yes")

	assert.Equal(t, uicli.ExitCancelled, got.Code, got.Stderr)
	assert.False(t, viewOpened(got))
}

func TestTheLiveViewOpensOnlyForAPersonAtATerminalOnJobsLeftUp(t *testing.T) {
	for name, c := range map[string]struct {
		words    []string
		terminal bool
	}{
		"detached":    {words: []string{featA, "--yes", "-d"}, terminal: true},
		"nothing_up":  {words: []string{featB, "--yes"}, terminal: true},
		"json":        {words: []string{featA, "--yes", "--output", "json"}, terminal: true},
		"quiet":       {words: []string{featA, "--yes", "--quiet"}, terminal: true},
		"dry_run":     {words: []string{featA, "--dry-run", "--yes"}, terminal: true},
		"no_terminal": {words: []string{featA, "--yes"}, terminal: false},
	} {
		t.Run(name, func(t *testing.T) {
			got := runUp(t, newWorld(), c.words...)
			if c.terminal {
				got = runUpAt(t, newWorld(), "q", c.words...)
			}

			assert.False(t, viewOpened(got))
		})
	}
}

// Legacy's cursor followed the start; the view opens on the first job the start launched, not on a shared service it may not focus.
func TestTheViewOpensOnTheFirstJobTheStartLaunched(t *testing.T) {
	app, err := newWorld().open(t.Context(), mainPath)
	require.NoError(t, err)
	outcome, err := app.Run(t.Context(), runup.Request{Worktrees: []string{featA}}, domain.RunOptions{})
	require.NoError(t, err)
	streams, _, _, _ := output.Test()

	params := (&upCommand{io: uicli.Run{Streams: streams}}).viewOf(outcome)

	assert.Equal(t, "api", params.Job)
	assert.Equal(t, []string{"migrate", "api", "web"}, params.Worktrees[0].Jobs)
}

func TestTheViewKnowsTheNamespacesThePlanGivesEachWorktree(t *testing.T) {
	outcome := runup.Outcome{
		Plan: runup.Plan{Worktrees: []runup.Target{{Worktree: featB, Path: pathOf(featB), Jobs: []runup.JobPlan{
			{Name: "db", SharedIn: "main", Namespace: "acme_feat-b"},
			{Name: "api"},
		}}}},
		Result: &runup.Result{Items: []runup.Item{{Subject: featB, Detail: &runup.Detail{Jobs: []runup.JobResult{{Name: "db", Status: runup.JobJoined}, {Name: "api", Status: runup.JobStarted}}}}}},
	}
	streams, _, _, _ := output.Test()

	params := (&upCommand{io: uicli.Run{Streams: streams}}).viewOf(outcome)

	assert.Equal(t, map[string]string{"db": "acme_feat-b"}, params.Worktrees[0].Namespaces)
	assert.Equal(t, "main", params.Main)
	assert.Equal(t, "api", params.Job)
}
