package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/reparent"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfReparentIsTheLegacyOnesWithNextsContract(t *testing.T) {
	got := runReparent(t, newWorld(seed{}), "--help")
	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "reparent_help", got.Stdout)
}

func TestAMoveRecapsEachWorktreeAndNamesTheSyncThatRebasesIt(t *testing.T) {
	w := newWorld(seed{})
	got := runReparent(t, w, featB, featX, "--to", "main")
	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "reparented", got.Stdout)
	record, err := w.store.Load(featB)
	require.NoError(t, err)
	assert.Equal(t, "main", record.SourceBranch)
}

func TestTheWorktreeTheCallerStandsInMovesWhenNoneIsNamed(t *testing.T) {
	w := newWorld(seed{Cwd: pathOf(featC)})
	got := runReparent(t, w, "--to", "main")
	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	record, err := w.store.Load(featC)
	require.NoError(t, err)
	assert.Equal(t, "main", record.SourceBranch)
}

func TestADryRunShowsThePlanAndTheLineThatAppliesIt(t *testing.T) {
	got := runReparent(t, newWorld(seed{}), featC, "--to", "main", "--dry-run")
	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "dryrun", got.Stdout)
}

func TestACycleIsRefusedAsAnInvalidRequestNamingWhereTheParentSits(t *testing.T) {
	got := runReparent(t, newWorld(seed{}), featA, "--to", featC, "--yes")
	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Empty(t, got.Stdout)
	formtest.Golden(t, "cycle", got.Stderr)
}

func TestNoParentWithoutATerminalIsRefusedNamingTheFlag(t *testing.T) {
	got := runReparent(t, newWorld(seed{}), featC)
	assert.Equal(t, uicli.ExitUsage, got.Code)
	formtest.Golden(t, "no_parent", got.Stderr)
}

func TestTheMainWorktreeIsRefusedAsAnInvalidRequest(t *testing.T) {
	got := runReparent(t, newWorld(seed{}), "main", "--to", featA, "--yes")
	assert.Equal(t, uicli.ExitUsage, got.Code)
	formtest.Golden(t, "main_worktree", got.Stderr)
}

func TestARecordThatCannotBeWrittenSetsTheExitCode(t *testing.T) {
	got := runReparent(t, newWorld(seed{WriteFail: errDiskFull}), featC, "--to", "main")
	assert.Equal(t, uicli.ExitError, got.Code)
	formtest.Golden(t, "write_failed", got.Stdout+"--- stderr\n"+got.Stderr)
}

func TestEachEnvelopeOfReparentHasItsGolden(t *testing.T) {
	for name, c := range map[string]struct {
		Seed  seed
		Words []string
		Code  int
	}{
		"success":      {Words: []string{featB, featX, "--to", "main"}, Code: uicli.ExitOK},
		"dry_run":      {Words: []string{featC, "--to", featA, "--dry-run"}, Code: uicli.ExitOK},
		"cycle":        {Words: []string{featA, "--to", featC}, Code: uicli.ExitUsage},
		"stale":        {Words: []string{featC, "--to", "main", "--expect-fingerprint", fingerprint}, Code: uicli.ExitPrecondition},
		"write_failed": {Seed: seed{WriteFail: errDiskFull}, Words: []string{featC, "--to", "main"}, Code: uicli.ExitError},
	} {
		t.Run(name, func(t *testing.T) {
			got := runReparent(t, newWorld(c.Seed), append(c.Words, "--output", "json")...)
			assert.Equal(t, c.Code, got.Code, got.Stdout)
			assert.Empty(t, got.Stderr)
			formtest.Golden(t, "envelope_"+name, got.Stdout)
		})
	}
}

func TestAFollowUpNamingReparentIsSpelledFromItsSchema(t *testing.T) {
	undo := domain.FollowUp{Command: reparent.Name, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{featC}}, {Path: domain.PathTo, Values: []string{featB}}}}
	wrong := domain.FollowUp{Command: reparent.Name, Answers: []domain.Answered{{Path: "force", Values: []string{"true"}}}}
	assert.Equal(t, []string{"wtm-next", "reparent", featC, "--to", featB}, Line(undo))
	assert.Nil(t, Line(wrong))
}
