package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/sync"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfSyncStatesItsDefaults(t *testing.T) {
	got := runSync(t, newWorld(seed{}), "--help")
	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "sync_help", got.Stdout)
}

func TestAStackSyncedNamesEachWorktreeRebasedAndTheDescendantsIncluded(t *testing.T) {
	w := newWorld(seed{})
	got := runSync(t, w, featA, "--yes")
	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "stack", got.Stdout)
	assert.Contains(t, w.git.Calls(), "Rebase "+pathOf(featC))
}

func TestADryRunShowsThePlanAndTheLineThatAppliesIt(t *testing.T) {
	got := runSync(t, newWorld(seed{}), featA, "--dry-run")
	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "dryrun", got.Stdout)
}

func TestWithoutDescendantsTheRecapNamesThoseLeftOut(t *testing.T) {
	got := runSync(t, newWorld(seed{}), featA, "--descendants=false", "--dry-run")
	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "only_named", got.Stdout)
}

func TestAConflictMidStackExitsOnItsKindAndSaysHowToResolveIt(t *testing.T) {
	got := runSync(t, newWorld(seed{Conflicts: map[string][]string{featB: {"shared.txt"}}}), featA, "--yes")
	assert.Equal(t, uicli.ExitConflict, got.Code)
	formtest.Golden(t, "conflict", got.Stdout+got.Stderr)
}

func TestANamedDirtyWorktreeIsRefusedWithTheForcedLine(t *testing.T) {
	got := runSync(t, newWorld(seed{}), dirty, "--yes")
	assert.Equal(t, uicli.ExitRefused, got.Code)
	assert.Empty(t, got.Stdout)
	formtest.Golden(t, "unsafe", got.Stderr)
}

func TestTheBaseBranchNamedIsRefusedWithTheFastForwardLine(t *testing.T) {
	got := runSync(t, newWorld(seed{}), "main", "--yes")
	assert.Equal(t, uicli.ExitUsage, got.Code)
	formtest.Golden(t, "base", got.Stderr)
}

func TestNoWorktreeWithoutATerminalSyncsThoseThatNeedIt(t *testing.T) {
	w := newWorld(seed{})
	got := runSync(t, w, "--yes")
	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "no_worktree", got.Stdout)
	assert.NotContains(t, w.git.Calls(), "Rebase "+pathOf(dirty))
}

func TestEachEnvelopeOfSyncHasItsGolden(t *testing.T) {
	for name, c := range map[string]struct {
		Words []string
		Seed  seed
		Code  int
	}{
		"stack":    {Words: []string{featA, "--push"}, Code: uicli.ExitOK},
		"dry_run":  {Words: []string{featA, "--dry-run"}, Code: uicli.ExitOK},
		"conflict": {Words: []string{featA, "--keep-conflict"}, Seed: seed{Conflicts: map[string][]string{featB: {"shared.txt"}}}, Code: uicli.ExitConflict},
		"unsafe":   {Words: []string{dirty}, Code: uicli.ExitRefused},
		"missing":  {Words: []string{"feat/nope"}, Code: uicli.ExitUsage},
		"stale":    {Words: []string{featA, "--expect-fingerprint", fingerprint}, Code: uicli.ExitPrecondition},
	} {
		t.Run(name, func(t *testing.T) {
			got := runSync(t, newWorld(c.Seed), append(c.Words, "--yes", "--output", "json")...)
			assert.Equal(t, c.Code, got.Code, got.Stdout)
			assert.Empty(t, got.Stderr)
			formtest.Golden(t, "envelope_"+name, got.Stdout)
		})
	}
}

func TestAFollowUpNamingSyncIsSpelledFromItsSchema(t *testing.T) {
	push := domain.FollowUp{Command: sync.Name, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{featA, featB}}, {Path: sync.PathDescendants, Values: []string{"false"}}, {Path: sync.PathPush, Values: []string{"true"}}}}
	assert.Equal(t, []string{"wtm-next", "sync", featA, featB, "--descendants=false", "--push"}, Line(push))
}
