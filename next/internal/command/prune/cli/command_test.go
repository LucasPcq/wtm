package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/prune"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfPruneIsTheLegacyOnesWithNextsContract(t *testing.T) {
	got := runPrune(t, newWorld(seed{}), "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "prune_help", got.Stdout)
}

func TestAPruneCountsWhatWentAndNamesWhatItKeptAndLeftBehind(t *testing.T) {
	w := newWorld(seed{})

	got := runPrune(t, w)

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "pruned", got.Stdout)
	assert.Contains(t, w.git.Calls(), "DeleteBranch "+merged)
	assert.NotContains(t, w.git.Calls(), "DeleteBranch "+dirty)
}

// B1, B7: legacy's dry run said it reparented, and its forced dry run hid what forcing loses.
func TestADryRunShowsWhyEachIsDoneWithWhatIsKeptAndWhereEachChildStays(t *testing.T) {
	got := runPrune(t, newWorld(seed{}), "--dry-run")
	forced := runPrune(t, newWorld(seed{}), "--dry-run", "--force", "--reparent-children")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "dryrun", got.Stdout)
	formtest.Golden(t, "dryrun_forced", forced.Stdout)
}

func TestAnUnsafeWorktreeNamedIsRefusedWithEachReasonAndTheForcedLine(t *testing.T) {
	got := runPrune(t, newWorld(seed{}), merged, dirty)

	assert.Equal(t, uicli.ExitRefused, got.Code)
	assert.Empty(t, got.Stdout)
	formtest.Golden(t, "unsafe", got.Stderr)
}

func TestAWorktreeNamedThatIsNotDoneWithIsAnInvalidRequest(t *testing.T) {
	got := runPrune(t, newWorld(seed{}), plain)

	assert.Equal(t, uicli.ExitUsage, got.Code)
	formtest.Golden(t, "not_finished", got.Stderr)
}

func TestNothingToPruneSaysSoAndExitsZero(t *testing.T) {
	got := runPrune(t, newWorld(seed{NoPRs: true}), "--merged")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "nothing", got.Stdout)
}

func TestAFailingHookKeepsTheWorktreeAndSetsTheExitCode(t *testing.T) {
	w := newWorld(seed{OnClean: []project.Hook{{Cmd: "make down"}}, RunHooks: hookFails})

	got := runPrune(t, w, merged)

	assert.Equal(t, uicli.ExitError, got.Code)
	formtest.Golden(t, "hook_failed", got.Stdout+"--- stderr\n"+got.Stderr)
}

func TestEachEnvelopeOfPruneHasItsGolden(t *testing.T) {
	for name, c := range map[string]struct {
		Seed  seed
		Words []string
		Code  int
	}{
		"success":         {Code: uicli.ExitOK},
		"dry_run":         {Words: []string{"--reparent-children", "--dry-run"}, Code: uicli.ExitOK},
		"unsafe":          {Words: []string{dirty}, Code: uicli.ExitRefused},
		"forced":          {Words: []string{dirty, "--force"}, Code: uicli.ExitOK},
		"not_finished":    {Words: []string{plain}, Code: uicli.ExitUsage},
		"nothing":         {Seed: seed{NoPRs: true}, Words: []string{"--merged"}, Code: uicli.ExitOK},
		"stale":           {Words: []string{merged, "--expect-fingerprint", fingerprint}, Code: uicli.ExitPrecondition},
		"hook_failed":     {Seed: seed{OnClean: []project.Hook{{Cmd: "make down"}}, RunHooks: hookFails}, Words: []string{merged}, Code: uicli.ExitError},
		"prs_unavailable": {Seed: seed{PRErr: worktree.ErrPullRequestsUnavailable}, Words: []string{"--dry-run"}, Code: uicli.ExitOK},
	} {
		t.Run(name, func(t *testing.T) {
			got := runPrune(t, newWorld(c.Seed), append(c.Words, "--output", "json")...)

			assert.Equal(t, c.Code, got.Code, got.Stdout)
			assert.Empty(t, got.Stderr)
			formtest.Golden(t, "envelope_"+name, got.Stdout)
		})
	}
}

func TestAFollowUpNamingPruneIsSpelledFromItsSchema(t *testing.T) {
	forced := domain.FollowUp{Command: prune.Name, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{dirty}}, {Path: prune.PathForce, Values: []string{"true"}}, {Path: prune.PathGone, Values: []string{"true"}}}, DryRun: true}
	wrong := domain.FollowUp{Command: prune.Name, Answers: []domain.Answered{{Path: "keep_data", Values: []string{"true"}}}}

	assert.Equal(t, []string{"wtm-next", "prune", dirty, "--force", "--gone", "--dry-run"}, Line(forced))
	assert.Nil(t, Line(wrong))
}
