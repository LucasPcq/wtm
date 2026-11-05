package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/rundown"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfRunDownSaysWhatItStops(t *testing.T) {
	got := runDown(t, newWorld(), "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "down_help", got.Stdout)
}

func TestAStopInTheWorktreeYouStandInSaysWhatStoppedAndWhatWasReleased(t *testing.T) {
	got := runDown(t, newWorld(), "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "stopped", got.Stdout)
	formtest.Golden(t, "stopped_progress", got.Stderr)
}

// B1: legacy's stop_all lost every other job's report once one stop failed.
func TestAStopThatFailsNamesItsJobAndTheOthersStillStop(t *testing.T) {
	got := runDown(t, newWorld(), featA, featB, "--yes")

	assert.Equal(t, uicli.ExitError, got.Code)
	formtest.Golden(t, "failed", got.Stdout)
}

func TestADryRunShowsWhatStopsAndWhatIsReleasedForWhom(t *testing.T) {
	got := runDown(t, newWorld(), featA, "--dry-run")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "dryrun", got.Stdout)
}

// B3: legacy said "this worktree" of a worktree named from another.
func TestAWorktreeRunningNothingIsNamed(t *testing.T) {
	w := newWorld()
	w.daemon.listed = nil

	got := runDown(t, w, featB, "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "nothing_running", got.Stdout)
}

func TestAllBesideAWorktreeIsAProblemOfTheFlag(t *testing.T) {
	got := runDown(t, newWorld(), featA, "--all", "--yes")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	formtest.Golden(t, "all_narrowed", got.Stderr)
}

// B2: legacy printed its refusals and failures as text under --output json.
func TestEachEnvelopeOfRunDownHasItsGolden(t *testing.T) {
	for name, c := range map[string]struct {
		Words []string
		Code  int
	}{
		"stopped":   {Words: []string{featA}, Code: uicli.ExitOK},
		"failed":    {Words: []string{featA, featB}, Code: uicli.ExitError},
		"dry_run":   {Words: []string{featA, "--dry-run"}, Code: uicli.ExitOK},
		"not_found": {Words: []string{"nope"}, Code: uicli.ExitUsage},
		"stale":     {Words: []string{featA, "--expect-fingerprint", fingerprint}, Code: uicli.ExitPrecondition},
	} {
		t.Run(name, func(t *testing.T) {
			got := runDown(t, newWorld(), append([]string{"--output", "json", "--yes"}, c.Words...)...)

			assert.Equal(t, c.Code, got.Code, got.Stdout)
			assert.Empty(t, got.Stderr)
			formtest.Golden(t, "envelope_"+name, got.Stdout)
		})
	}
}

func TestAFollowUpOfRunDownIsSpelledAsItsCommandLine(t *testing.T) {
	down := domain.FollowUp{Command: rundown.Name, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{featA, featB}}, {Path: rundown.PathProfile, Values: []string{"dev"}}}}

	assert.Equal(t, []string{"wtm-next", "run", "down", featA, featB, "--profile", "dev"}, Line(down))
}
