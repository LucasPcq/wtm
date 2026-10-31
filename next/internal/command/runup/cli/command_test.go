package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runup"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

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
