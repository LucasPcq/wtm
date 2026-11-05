package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runstop"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfRunStopSaysWhatItStops(t *testing.T) {
	got := runStop(t, newWorld(), "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "stop_help", got.Stdout)
}

func TestAStopInTheWorktreeYouStandInSaysWhatStopped(t *testing.T) {
	got := runStop(t, newWorld(), "--job", "api", "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "stopped", got.Stdout)
	formtest.Golden(t, "stopped_progress", got.Stderr)
}

func TestAReleaseNamesTheWorktreesKeepingTheService(t *testing.T) {
	got := runStop(t, newWorld(), featA, "--job", "db", "--dry-run")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "dryrun_release", got.Stdout)
}

// B10 of run stop: legacy's failure ended the run, the other worktrees unreported.
func TestAStopThatFailsIsThatWorktreesAndTheOthersAreReported(t *testing.T) {
	got := runStop(t, newWorld(), featA, featB, "--job", "cache", "--yes")

	assert.Equal(t, uicli.ExitError, got.Code)
	formtest.Golden(t, "failed", got.Stdout)
}

// B9: legacy exited 1 for a --job missing without a terminal.
func TestAJobMissingIsAUsageProblemNamingItsFlag(t *testing.T) {
	got := runStop(t, newWorld(), featA, "--yes")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	formtest.Golden(t, "job_missing", got.Stderr)
}

// B2: legacy printed its refusals and failures as text under --output json.
func TestEachEnvelopeOfRunStopHasItsGolden(t *testing.T) {
	for name, c := range map[string]struct {
		Words []string
		Code  int
	}{
		"stopped":     {Words: []string{featA, "--job", "api"}, Code: uicli.ExitOK},
		"not_running": {Words: []string{featA, "--job", "web", "--yes"}, Code: uicli.ExitOK},
		"failed":      {Words: []string{featA, featB, "--job", "cache"}, Code: uicli.ExitError},
		"dry_run":     {Words: []string{featA, "--job", "db", "--dry-run"}, Code: uicli.ExitOK},
		"not_found":   {Words: []string{featA, "--job", "ghost"}, Code: uicli.ExitUsage},
	} {
		t.Run(name, func(t *testing.T) {
			w := newWorld()
			if name == "not_running" {
				w.daemon.listed = nil
			}
			got := runStop(t, w, append([]string{"--output", "json", "--yes"}, c.Words...)...)

			assert.Equal(t, c.Code, got.Code, got.Stdout)
			assert.Empty(t, got.Stderr)
			formtest.Golden(t, "envelope_"+name, got.Stdout)
		})
	}
}

func TestAFollowUpOfRunStopIsSpelledAsItsCommandLine(t *testing.T) {
	stop := domain.FollowUp{Command: runstop.Name, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{featA, featB}}, {Path: runstop.PathJob, Values: []string{"api"}}}}

	assert.Equal(t, []string{"wtm-next", "run", "stop", featA, featB, "--job", "api"}, Line(stop))
}

// The owner's rule: at a terminal with no worktree named, the picker opens on the one you stand in.
func TestAtATerminalWithNoWorktreeNamedThePickerOpensOnYours(t *testing.T) {
	d := wizard(t, newWorld(), runstop.Request{Job: "api"}.From(domain.Caller{Cwd: pathOf(featA)}))

	assert.Contains(t, d.Screen(), "Worktrees")
	assert.Contains(t, d.Screen(), "[x] feat/a", "the caller's worktree is ticked")
	assert.Contains(t, d.Screen(), "[ ] feat/b")
}

// The owner's rule: unattended, outside every worktree, the stop is refused naming the argument.
func TestUnattendedOutsideAWorktreeTheStopIsRefusedNamingTheArgument(t *testing.T) {
	w := newWorld()
	w.cwd = "/elsewhere"
	for name, words := range map[string][]string{
		"yes":  {"--job", "api", "--yes"},
		"json": {"--job", "api", "--yes", "--output", "json"},
	} {
		t.Run(name, func(t *testing.T) {
			got := runStop(t, w, words...)

			assert.Equal(t, uicli.ExitUsage, got.Code)
			assert.Contains(t, got.Stdout+got.Stderr, "<worktree> is required")
		})
	}
}
