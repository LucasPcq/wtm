package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/fastforward"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfFastForwardIsTheLegacyOnesWithNextsContract(t *testing.T) {
	got := runFastForward(t, newWorld(), "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "fast_forward_help", got.Stdout)
}

func TestEveryWorktreeCountsWhatMovedAndNamesWhatIsLeft(t *testing.T) {
	w := newWorld()

	got := runFastForward(t, w, "--all", "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "all", got.Stdout)
	assert.Contains(t, w.git.Calls(), "MergeFFOnly "+pathOf(behind))
	assert.NotContains(t, w.git.Calls(), "MergeFFOnly "+pathOf(dirty))
}

func TestTheAliasFFRunsIt(t *testing.T) {
	got := runFastForward(t, newWorld(), behind, "--yes")
	alias := runFastForwardAs(t, newWorld(), "ff", behind, "--yes")

	assert.Equal(t, got, alias)
}

func TestADryRunShowsThePlanAndTheLineThatAppliesIt(t *testing.T) {
	got := runFastForward(t, newWorld(), behind, latest, diverged, "--dry-run")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "dryrun", got.Stdout)
}

func TestANamedDirtyWorktreeIsRefusedWithTheForcedLine(t *testing.T) {
	got := runFastForward(t, newWorld(), behind, dirty, "--yes")

	assert.Equal(t, uicli.ExitRefused, got.Code)
	assert.Empty(t, got.Stdout)
	formtest.Golden(t, "unsafe", got.Stderr)
}

func TestForceMovesADirtyWorktreeAndTheRecapSaysSo(t *testing.T) {
	got := runFastForward(t, newWorld(), dirty, "--force", "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "forced", got.Stdout)
}

func TestNoWorktreeWithoutATerminalIsAUsageError(t *testing.T) {
	got := runFastForward(t, newWorld())

	assert.Equal(t, uicli.ExitUsage, got.Code)
	formtest.Golden(t, "no_worktree", got.Stderr)
}

func TestEachEnvelopeOfFastForwardHasItsGolden(t *testing.T) {
	for name, c := range map[string]struct {
		Words []string
		Code  int
	}{
		"all":     {Words: []string{"--all"}, Code: uicli.ExitOK},
		"dry_run": {Words: []string{behind, "--dry-run"}, Code: uicli.ExitOK},
		"unsafe":  {Words: []string{behind, dirty}, Code: uicli.ExitRefused},
		"forced":  {Words: []string{dirty, "--force"}, Code: uicli.ExitOK},
		"missing": {Words: []string{"feat/nope"}, Code: uicli.ExitUsage},
		"stale":   {Words: []string{behind, "--expect-fingerprint", fingerprint}, Code: uicli.ExitPrecondition},
	} {
		t.Run(name, func(t *testing.T) {
			got := runFastForward(t, newWorld(), append(c.Words, "--yes", "--output", "json")...)

			assert.Equal(t, c.Code, got.Code, got.Stdout)
			assert.Empty(t, got.Stderr)
			formtest.Golden(t, "envelope_"+name, got.Stdout)
		})
	}
}

func TestAFollowUpNamingFastForwardIsSpelledFromItsSchema(t *testing.T) {
	forced := domain.FollowUp{Command: fastforward.Name, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{dirty}}, {Path: fastforward.PathForce, Values: []string{"true"}}}}

	assert.Equal(t, []string{"wtm-next", "fast-forward", dirty, "--force"}, Line(forced))
}
