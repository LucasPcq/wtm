package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runstart"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfRunStartSaysWhatItStarts(t *testing.T) {
	got := runStart(t, newWorld(), "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "start_help", got.Stdout)
}

func TestAStartInTheWorktreeYouStandInSaysWhereTheJobAnswers(t *testing.T) {
	got := runStart(t, newWorld(), "--job", "api", "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "started", got.Stdout)
	formtest.Golden(t, "started_progress", got.Stderr)
}

func TestARestartsDryRunSaysTheJobStopsThenStartsAgain(t *testing.T) {
	w := newWorld()
	w.daemon.listed = []run.Job{{Name: "api", WorkDir: pathOf(featA), Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn}}

	got := runStart(t, w, featA, "--job", "api", "--restart", "--dry-run")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "dryrun_restart", got.Stdout)
}

// B5: legacy said "Profile aborted" of a task started alone, and offered to stop what did start when nothing did.
func TestATaskThatFailsShowsItsLastLinesAndNoStop(t *testing.T) {
	got := runStart(t, newWorld(), featB, "--job", "migrate", "--yes")

	assert.Equal(t, uicli.ExitError, got.Code)
	formtest.Golden(t, "failed", got.Stdout)
}

// B9: legacy exited 1 for a --job missing without a terminal.
func TestAJobMissingIsAUsageProblemNamingItsFlag(t *testing.T) {
	got := runStart(t, newWorld(), featA, "--yes")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	formtest.Golden(t, "job_missing", got.Stderr)
}

// B2, B6: legacy printed a bare object, and its refusals as text, under --output json.
func TestEachEnvelopeOfRunStartHasItsGolden(t *testing.T) {
	for name, c := range map[string]struct {
		Words []string
		Code  int
	}{
		"started":   {Words: []string{featA, "--job", "api"}, Code: uicli.ExitOK},
		"failed":    {Words: []string{featB, "--job", "migrate"}, Code: uicli.ExitError},
		"dry_run":   {Words: []string{featA, "--job", "api", "--dry-run"}, Code: uicli.ExitOK},
		"not_found": {Words: []string{featA, "--job", "ghost"}, Code: uicli.ExitUsage},
		"stale":     {Words: []string{featA, "--job", "api", "--expect-fingerprint", fingerprint}, Code: uicli.ExitPrecondition},
	} {
		t.Run(name, func(t *testing.T) {
			got := runStart(t, newWorld(), append([]string{"--output", "json", "--yes"}, c.Words...)...)

			assert.Equal(t, c.Code, got.Code, got.Stdout)
			assert.Empty(t, got.Stderr)
			formtest.Golden(t, "envelope_"+name, got.Stdout)
		})
	}
}

func TestAFollowUpOfRunStartIsSpelledAsItsCommandLine(t *testing.T) {
	start := domain.FollowUp{Command: runstart.Name, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{featB}}, {Path: runstart.PathJob, Values: []string{"api"}}, {Path: runstart.PathRestart, Values: []string{"true"}}}}

	assert.Equal(t, []string{"wtm-next", "run", "start", featB, "--job", "api", "--restart"}, Line(start))
}

func TestAtATerminalTheStartOpensTheLiveViewOnTheJob(t *testing.T) {
	got := runStartAt(t, newWorld(), "q", "--job", "api", "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.True(t, viewOpened(got))
}

func TestTheLiveViewOpensOnlyForAPersonAtATerminalOnAServiceLeftUp(t *testing.T) {
	for name, c := range map[string]struct {
		words    []string
		terminal bool
	}{
		"detached":    {words: []string{featA, "--job", "api", "--yes", "-d"}, terminal: true},
		"task":        {words: []string{featA, "--job", "migrate", "--yes"}, terminal: true},
		"json":        {words: []string{featA, "--job", "api", "--yes", "--output", "json"}, terminal: true},
		"quiet":       {words: []string{featA, "--job", "api", "--yes", "--quiet"}, terminal: true},
		"dry_run":     {words: []string{featA, "--job", "api", "--dry-run", "--yes"}, terminal: true},
		"no_terminal": {words: []string{featA, "--job", "api", "--yes"}, terminal: false},
	} {
		t.Run(name, func(t *testing.T) {
			got := runStart(t, newWorld(), c.words...)
			if c.terminal {
				got = runStartAt(t, newWorld(), "q", c.words...)
			}

			assert.False(t, viewOpened(got))
		})
	}
}

// The owner's rule: at a terminal with no worktree named, the picker opens on the one you stand in.
func TestAtATerminalWithNoWorktreeNamedThePickerOpensOnYours(t *testing.T) {
	d := wizard(t, newWorld(), runstart.Request{Job: "api"}.From(domain.Caller{Cwd: pathOf(featA)}))

	assert.Contains(t, d.Screen(), "Worktree")
	assert.Contains(t, d.Screen(), "› feat/a", "the caller's worktree is the one selected")
}

// The owner's rule: unattended, outside every worktree, the start is refused naming the argument.
func TestUnattendedOutsideAWorktreeTheStartIsRefusedNamingTheArgument(t *testing.T) {
	w := newWorld()
	w.cwd = "/elsewhere"
	for name, words := range map[string][]string{
		"yes":  {"--job", "api", "--yes"},
		"json": {"--job", "api", "--yes", "--output", "json"},
	} {
		t.Run(name, func(t *testing.T) {
			got := runStart(t, w, words...)

			assert.Equal(t, uicli.ExitUsage, got.Code)
			assert.Contains(t, got.Stdout+got.Stderr, "<worktree> is required")
		})
	}
}
