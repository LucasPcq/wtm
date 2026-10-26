package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/exec"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfExecSaysWhereTheCommandGoes(t *testing.T) {
	got := runExec(t, newWorld(), "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "exec_help", got.Stdout)
}

func TestARunThatPassesEverywhereCountsIt(t *testing.T) {
	got := runExec(t, newWorld(), featA, featB, "--", "echo", "hi")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "passed", got.Stdout)
}

func TestARunThatFailsInOneWorktreeShowsItsLastLinesAndTheLineThatRunsItAgain(t *testing.T) {
	got := runExec(t, newWorld(), featA, featB, "--", failsInB)

	assert.Equal(t, uicli.ExitError, got.Code)
	formtest.Golden(t, "failed", got.Stdout)
}

func TestPrintShowsEachWorktreesWholeOutputBeforeTheConclusion(t *testing.T) {
	got := runExec(t, newWorld(), featA, featB, "--print", "--", "echo", "hi")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "printed", got.Stdout)
}

func TestQuietOnAFailureStillSaysWhereToRunItAgain(t *testing.T) {
	got := runExec(t, newWorld(), "-q", featA, featB, "--", failsInB)

	assert.Equal(t, uicli.ExitError, got.Code)
	assert.Empty(t, got.Stdout)
	formtest.Golden(t, "quiet_failed", got.Stderr)
}

func TestADryRunShowsThePlanAndTheLineThatAppliesIt(t *testing.T) {
	got := runExec(t, newWorld(), "--all", "--dry-run", "--", failsInB)

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "dryrun", got.Stdout)
}

func TestNoWorktreeWithoutATerminalIsOneProblemNamingTheArgument(t *testing.T) {
	got := runExec(t, newWorld(), "--", "true")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	formtest.Golden(t, "no_worktree", got.Stderr)
}

func TestEachProblemOfTheRequestIsALineOfItsFailure(t *testing.T) {
	got := runExec(t, newWorld(), featA, "nope", gone, "--jobs", "-1", "--", "if then fi")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	formtest.Golden(t, "invalid", got.Stderr)
}

func TestEachEnvelopeOfExecHasItsGolden(t *testing.T) {
	for name, c := range map[string]struct {
		Words []string
		Code  int
	}{
		"passed":     {Words: []string{featA, featB, "--", "echo", "hi"}, Code: uicli.ExitOK},
		"failed":     {Words: []string{featA, featB, "--", failsInB}, Code: uicli.ExitError},
		"printed":    {Words: []string{featA, "--print", "--", "echo", "hi"}, Code: uicli.ExitOK},
		"dry_run":    {Words: []string{"--all", "--dry-run", "--", failsInB}, Code: uicli.ExitOK},
		"not_found":  {Words: []string{"nope", "--", "true"}, Code: uicli.ExitUsage},
		"no_command": {Words: []string{featA}, Code: uicli.ExitUsage},
		"invalid":    {Words: []string{featA, "--", "if then fi"}, Code: uicli.ExitUsage},
		"stale":      {Words: []string{featA, "--expect-fingerprint", fingerprint, "--", "true"}, Code: uicli.ExitPrecondition},
	} {
		t.Run(name, func(t *testing.T) {
			got := runExec(t, newWorld(), append([]string{"--output", "json"}, c.Words...)...)

			assert.Equal(t, c.Code, got.Code, got.Stdout)
			assert.Empty(t, got.Stderr)
			formtest.Golden(t, "envelope_"+name, got.Stdout)
		})
	}
}

func TestTheRetryIsSpelledWithTheCommandAfterTheDash(t *testing.T) {
	retry := domain.FollowUp{Command: exec.Name, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{featB}}, {Path: exec.PathCommand, Values: []string{failsInB}}, {Path: exec.PathJobs, Values: []string{"2"}}}}

	assert.Equal(t, []string{"wtm-next", "exec", featB, "--jobs", "2", "--", failsInB}, Line(retry))
}
