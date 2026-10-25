package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/clean"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfCleanIsTheLegacyOnesWithNextsContract(t *testing.T) {
	got := runClean(t, newWorld(seed{}), "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "clean_help", got.Stdout)
}

func TestARemovalCountsWhatWentAndNamesTheChildrenLeftBehind(t *testing.T) {
	w := newWorld(seed{})

	got := runClean(t, w, plain, parent)

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "removed", got.Stdout)
	assert.Contains(t, w.git.Calls(), "DeleteBranch "+parent)
}

func TestReparentedChildrenAreNamedWithWhereTheyWent(t *testing.T) {
	got := runClean(t, newWorld(seed{}), parent, "--reparent-children")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "reparented", got.Stdout)
}

func TestADryRunShowsThePlanAndTheLineThatAppliesIt(t *testing.T) {
	got := runClean(t, newWorld(seed{}), parent, "feat/nope", "--dry-run")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "dryrun", got.Stdout)
}

func TestAnUnsafeWorktreeIsRefusedWithEachReasonAndTheForcedLine(t *testing.T) {
	got := runClean(t, newWorld(seed{}), plain, dirty, "--yes")

	assert.Equal(t, uicli.ExitRefused, got.Code)
	assert.Empty(t, got.Stdout)
	formtest.Golden(t, "unsafe", got.Stderr)
}

func TestForceRemovesAnUnsafeWorktreeAndTheRecapSaysSo(t *testing.T) {
	got := runClean(t, newWorld(seed{}), dirty, "--force", "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "forced", got.Stdout)
}

func TestNoWorktreeWithoutATerminalIsRefusedNamingTheArgument(t *testing.T) {
	got := runClean(t, newWorld(seed{}))

	assert.Equal(t, uicli.ExitUsage, got.Code)
	formtest.Golden(t, "no_worktree", got.Stderr)
}

func TestTheMainWorktreeIsRefusedAsAnInvalidRequest(t *testing.T) {
	got := runClean(t, newWorld(seed{}), "main", "--yes")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	formtest.Golden(t, "main_worktree", got.Stderr)
}

func TestAFailingHookKeepsTheWorktreeAndSetsTheExitCode(t *testing.T) {
	w := newWorld(seed{OnClean: []project.Hook{{Cmd: "make down"}}, RunHooks: hookFails})

	got := runClean(t, w, plain, "--yes")

	assert.Equal(t, uicli.ExitError, got.Code)
	formtest.Golden(t, "hook_failed", got.Stdout+"--- stderr\n"+got.Stderr)
}

func TestQuietPrintsNothingButARefusal(t *testing.T) {
	removed := runClean(t, newWorld(seed{}), "-q", plain, "--yes")
	refused := runClean(t, newWorld(seed{}), "-q", dirty, "--yes")

	assert.Equal(t, uicli.ExitOK, removed.Code)
	assert.Empty(t, removed.Stdout+removed.Stderr)
	assert.Equal(t, uicli.ExitRefused, refused.Code)
	formtest.Golden(t, "quiet_unsafe", refused.Stderr)
}

func TestEachEnvelopeOfCleanHasItsGolden(t *testing.T) {
	for name, c := range map[string]struct {
		Seed  seed
		Words []string
		Code  int
	}{
		"success":      {Words: []string{plain, parent}, Code: uicli.ExitOK},
		"dry_run":      {Words: []string{parent, "--reparent-children", "--dry-run"}, Code: uicli.ExitOK},
		"unsafe":       {Words: []string{plain, dirty}, Code: uicli.ExitRefused},
		"forced":       {Words: []string{dirty, "--force"}, Code: uicli.ExitOK},
		"absent":       {Words: []string{"feat/nope"}, Code: uicli.ExitOK},
		"main":         {Words: []string{"main"}, Code: uicli.ExitUsage},
		"stale":        {Words: []string{plain, "--expect-fingerprint", fingerprint}, Code: uicli.ExitPrecondition},
		"hook_failed":  {Seed: seed{OnClean: []project.Hook{{Cmd: "make down"}}, RunHooks: hookFails}, Words: []string{plain}, Code: uicli.ExitError},
		"jobs_unknown": {Seed: seed{JobsErr: run.ErrUnknown}, Words: []string{plain}, Code: uicli.ExitOK},
	} {
		t.Run(name, func(t *testing.T) {
			got := runClean(t, newWorld(c.Seed), append(c.Words, "--output", "json")...)

			assert.Equal(t, c.Code, got.Code, got.Stdout)
			assert.Empty(t, got.Stderr)
			formtest.Golden(t, "envelope_"+name, got.Stdout)
		})
	}
}

func TestAFollowUpNamingCleanIsSpelledFromItsSchema(t *testing.T) {
	forced := domain.FollowUp{Command: clean.Name, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{dirty}}, {Path: clean.PathForce, Values: []string{"true"}}}, DryRun: true}
	wrong := domain.FollowUp{Command: clean.Name, Answers: []domain.Answered{{Path: "keep_data", Values: []string{"true"}}}}

	assert.Equal(t, []string{"wtm-next", "clean", dirty, "--force", "--dry-run"}, Line(forced))
	assert.Nil(t, Line(wrong))
}
