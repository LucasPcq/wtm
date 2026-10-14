package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestTheHelpListsAFlagPerFieldWithItsDescriptionAndTheEngineFlags(t *testing.T) {
	w := newWorld(t, worldParams{})

	got := w.run(t, args(commandName, "--help"))

	require.Equal(t, domain.ExitCodeOK, got.Code)
	for _, want := range []string{
		"--from string", "The branch a new branch starts from",
		"--isolation string", "How the worktree's ports are isolated",
		"--ff", "Fast-forward the base branch first",
		"--env stringArray", "KEY=VALUE, repeatable",
		"--target-base string", "The base branch",
		"--yes", "--output", "--dry-run", "--expect-fingerprint string", "--ask",
	} {
		assert.Contains(t, got.Stdout, want)
	}
	assert.NotContains(t, got.Stdout, "--branches")
}

func TestARunAppliesThePlanAndConcludesWithTheCount(t *testing.T) {
	w := newWorld(t, worldParams{})

	got := w.run(t, args(commandName, "feat/a", "feat/b", "--yes"))

	require.Equal(t, domain.ExitCodeOK, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, "make: 2 done")
	assert.Contains(t, got.Stdout, "wtm make feat/next --ff")
	assert.Equal(t, []string{"feat/a", "feat/b"}, w.Repo.listed())
}

func TestAnIncompleteRequestIsA422NamingTheFlagsAndAppliesNothing(t *testing.T) {
	w := newWorld(t, worldParams{})

	got := w.run(t, args(commandName, "--yes", "--from", "main", "--target-base", "dev"))

	assert.Equal(t, domain.ExitCodeUsage, got.Code)
	assert.Contains(t, got.Stderr, "the request is invalid")
	assert.Contains(t, got.Stderr, "<branches> is required")
	assert.Contains(t, got.Stderr, "--target-base cannot be used with --from")
	assert.Empty(t, got.Stdout)
	assert.Empty(t, w.Repo.listed())
}

func TestA422InJSONIsTheEnvelopeNamingEachFieldAndItsFlag(t *testing.T) {
	w := newWorld(t, worldParams{})

	got := w.run(t, args(commandName, "--output", "json"))

	assert.Equal(t, domain.ExitCodeUsage, got.Code)
	doc := decode(t, got.Stdout)
	assert.Equal(t, 2, doc.V)
	assert.Equal(t, commandName, doc.Command)
	assert.False(t, doc.OK)
	require.NotNil(t, doc.Error)
	assert.Equal(t, string(kernel.KindInvalid), doc.Error.Kind)
	assert.Equal(t, string(kernel.CodeInvalidRequest), doc.Error.Code)
	assert.Equal(t, "the request is invalid", doc.Error.Message)
	require.Len(t, doc.Error.Fields, 1)
	assert.Equal(t, "branches", doc.Error.Fields[0].Path)
	assert.Equal(t, "<branches>", doc.Error.Fields[0].Flag)
	assert.Equal(t, "<branches> is required", doc.Error.Fields[0].Message)
	assert.NotNil(t, doc.FollowUps)
	assert.NotNil(t, doc.Warnings)
	assert.Empty(t, got.Stderr)
}

func TestADryRunShowsThePlanAndItsFingerprintAndAppliesNothing(t *testing.T) {
	w := newWorld(t, worldParams{})

	got := w.run(t, args(commandName, "feat/a", "--dry-run"))

	require.Equal(t, domain.ExitCodeOK, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, "make · dry run, nothing applied")
	assert.Contains(t, got.Stdout, "Isolation")
	assert.Contains(t, got.Stdout, "fingerprint")
	assert.Contains(t, got.Stdout, "wtm make feat/a --expect-fingerprint ")
	assert.Empty(t, w.Repo.listed())
}

func TestADryRunInJSONCarriesThePlanAndItsFingerprintAndNoItems(t *testing.T) {
	w := newWorld(t, worldParams{})

	doc := decode(t, w.run(t, args(commandName, "feat/a", "--dry-run", "--output", "json")).Stdout)

	assert.True(t, doc.OK)
	assert.JSONEq(t, `{"branches":["feat/a"],"isolation":"isolated"}`, string(doc.Plan))
	assert.Len(t, doc.Fingerprint, 24)
	assert.Nil(t, doc.Items)
	assert.Nil(t, doc.Error)
}

func TestTheFingerprintOfThePreviewAppliesItWhileTheFactsHold(t *testing.T) {
	w := newWorld(t, worldParams{})
	preview := decode(t, w.run(t, args(commandName, "feat/a", "--dry-run", "--output", "json")).Stdout)

	got := w.run(t, args(commandName, "feat/a", "--output", "json", "--expect-fingerprint", preview.Fingerprint))

	assert.Equal(t, domain.ExitCodeOK, got.Code)
	doc := decode(t, got.Stdout)
	assert.True(t, doc.OK)
	assert.Len(t, doc.Items, 1)
	assert.Equal(t, []string{"feat/a"}, w.Repo.listed())
}

func TestAFingerprintTheFactsNoLongerHaveIsA412PointingAtANewPreview(t *testing.T) {
	w := newWorld(t, worldParams{})
	preview := decode(t, w.run(t, args(commandName, "feat/a", "--dry-run", "--output", "json")).Stdout)
	w.Repo.exists("feat/other")

	human := w.run(t, args(commandName, "feat/a", "--expect-fingerprint", preview.Fingerprint))
	machine := w.run(t, args(commandName, "feat/a", "--output", "json", "--expect-fingerprint", preview.Fingerprint))

	assert.Equal(t, exitCodePrecondition, human.Code)
	assert.Contains(t, human.Stderr, "the facts changed since the preview")
	assert.Contains(t, human.Stderr, "wtm make feat/a --dry-run")
	doc := decode(t, machine.Stdout)
	require.NotNil(t, doc.Error)
	assert.Equal(t, string(kernel.CodePlanStale), doc.Error.Code)
	assert.Equal(t, string(kernel.KindPrecondition), doc.Error.Kind)
	assert.Equal(t, exitCodePrecondition, machine.Code)
	assert.Empty(t, w.Repo.listed())
}

func TestTheEnvelopeOfAnAppliedRunCarriesItsItemsAndFollowUps(t *testing.T) {
	w := newWorld(t, worldParams{})

	doc := decode(t, w.run(t, args(commandName, "feat/a", "--output", "json")).Stdout)

	assert.True(t, doc.OK)
	assert.Equal(t, "op-1", doc.Op)
	require.Len(t, doc.Items, 1)
	assert.JSONEq(t, `{"subject":"feat/a","status":"done","detail":"/wt/feat/a"}`, string(doc.Items[0]))
	require.Len(t, doc.FollowUps, 1)
	assert.JSONEq(t, `{"command":"make","request":{"branches":["feat/next"],"fast_forward":true},"code":"unit.interrupted"}`, string(doc.FollowUps[0]))
	assert.Empty(t, doc.Warnings)
	assert.Empty(t, doc.Plan)
}

func TestAFailedItemIsNamedWithItsCauseAndTheRunExitsOnIt(t *testing.T) {
	w := newWorld(t, worldParams{})

	human := w.run(t, args(commandName, "feat/a", failing, "--yes"))
	machine := decode(t, w.run(t, args(commandName, "feat/c", failing, "--output", "json")).Stdout)

	assert.Equal(t, domain.ExitCodeError, human.Code)
	assert.Contains(t, human.Stdout, "make: 1 done · 1 failed")
	assert.Contains(t, human.Stdout, failing+"  worktree failed: "+errStep.Error())
	assert.False(t, machine.OK)
	require.NotNil(t, machine.Error)
	assert.Equal(t, string(kernel.CodeStepFailed), machine.Error.Code)
	assert.Equal(t, []string{"feat/a", "feat/c"}, w.Repo.listed())
}

func TestAFailureOfApplyIsReportedOnStderr(t *testing.T) {
	w := newWorld(t, worldParams{})
	w.Repo.applyErr = errStep

	got := w.run(t, args(commandName, "feat/a", "--yes"))

	assert.Equal(t, domain.ExitCodeError, got.Code)
	assert.Contains(t, got.Stderr, "internal error: "+errStep.Error())
	assert.Empty(t, got.Stdout)
}

func TestAFailureOfTheCommandsOwnIsWordedAndExitsAsItDeclared(t *testing.T) {
	w := newWorld(t, worldParams{})
	w.Repo.taken = "feat/a"

	human := w.run(t, args(commandName, "feat/a", "--yes"))
	machine := decode(t, w.run(t, args(commandName, "feat/a", "--output", "json")).Stdout)
	quiet := w.run(t, args("--quiet", commandName, "feat/a", "--yes"))

	assert.Equal(t, domain.ExitCodeWorktreeExists, human.Code)
	assert.Contains(t, human.Stderr, "feat/a is taken")
	require.NotNil(t, machine.Error)
	assert.Equal(t, "feat/a is taken", machine.Error.Message)
	assert.Equal(t, domain.ExitCodeWorktreeExists, quiet.Code)
	assert.Contains(t, quiet.Stderr, "feat/a is taken")
}

func TestQuietSilencesTheReportButNotTheFailureLineNorTheEnvelope(t *testing.T) {
	w := newWorld(t, worldParams{})

	applied := w.run(t, args(commandName, "feat/a", "--yes", "--quiet"))
	refused := w.run(t, args(commandName, "--yes", "-q"))
	machine := w.run(t, args(commandName, "feat/b", "-q", "--output", "json"))

	assert.Equal(t, domain.ExitCodeOK, applied.Code)
	assert.Empty(t, applied.Stdout)
	assert.Empty(t, applied.Stderr)
	assert.Equal(t, domain.ExitCodeUsage, refused.Code)
	assert.Equal(t, "the request is invalid: <branches> is required", strings.TrimSpace(stripGlyph(refused.Stderr)))
	assert.True(t, decode(t, machine.Stdout).OK)
}

func TestJSONNeedsNoYesAndYesChangesNothingThere(t *testing.T) {
	w := newWorld(t, worldParams{})

	without := decode(t, w.run(t, args(commandName, "feat/a", "--output", "json")).Stdout)
	with := decode(t, w.run(t, args(commandName, "feat/b", "--output", "json", "--yes")).Stdout)

	assert.True(t, without.OK)
	assert.True(t, with.OK)
	assert.Equal(t, []string{"feat/a", "feat/b"}, w.Repo.listed())
}

func TestOnlyAFullyInteractiveRunGoesThroughTheForm(t *testing.T) {
	var asked []string
	form := func(_ context.Context, params FormParams) (dispatch.Reply, error) {
		asked = append(asked, string(params.Request))
		return nil, kernel.Cancelled(kernel.Problem{Code: kernel.CodeInterrupted})
	}
	w := newWorld(t, worldParams{Form: form})

	interactive := w.run(t, runParams{Args: []string{commandName, "feat/a"}, TTY: true})
	unattended := w.run(t, runParams{Args: []string{commandName, "feat/b", "--yes"}, TTY: true})
	machine := w.run(t, runParams{Args: []string{commandName, "feat/c", "--output", "json"}, TTY: true})
	offTerminal := w.run(t, runParams{Args: []string{commandName, "feat/d"}})

	assert.Equal(t, []string{`{"branches":["feat/a"]}`}, asked)
	assert.Equal(t, domain.ExitCodeCancelled, interactive.Code)
	for _, got := range []ran{unattended, machine, offTerminal} {
		assert.Equal(t, domain.ExitCodeOK, got.Code, got.Stderr)
	}
	assert.Equal(t, []string{"feat/b", "feat/c", "feat/d"}, w.Repo.listed())
}

func TestAFlagTheCommandDoesNotTakeIsA422WithItsEnvelope(t *testing.T) {
	w := newWorld(t, worldParams{})

	got := w.run(t, args(commandName, "--output", "json", "--bogus"))

	assert.Equal(t, domain.ExitCodeUsage, got.Code)
	doc := decode(t, got.Stdout)
	require.NotNil(t, doc.Error)
	assert.Equal(t, "the request is invalid: unknown flag: --bogus", doc.Error.Message)
}

func TestAnArgumentACommandTakesNoneOfIsRefused(t *testing.T) {
	w := newWorld(t, worldParams{})

	got := w.run(t, args(queryName, "extra"))

	assert.Equal(t, domain.ExitCodeUsage, got.Code)
	assert.Contains(t, got.Stderr, `unexpected argument "extra"`)
}

func TestAQueryAnswersWithItsView(t *testing.T) {
	w := newWorld(t, worldParams{})
	w.run(t, args(commandName, "feat/a", "--yes"))

	human := w.run(t, args(queryName))
	machine := decode(t, w.run(t, args(queryName, "--output", "json")).Stdout)

	assert.Contains(t, human.Stdout, "worktrees")
	assert.Contains(t, human.Stdout, "feat/a")
	assert.True(t, machine.OK)
	assert.JSONEq(t, `{"worktrees":["feat/a"]}`, string(machine.View))
}

func TestAnInterruptedRunExitsCancelled(t *testing.T) {
	w := newWorld(t, worldParams{})
	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	got := w.run(t, runParams{Args: []string{commandName, "feat/a", "--output", "json"}, Ctx: ctx})

	assert.Equal(t, domain.ExitCodeCancelled, got.Code)
	doc := decode(t, got.Stdout)
	assert.False(t, doc.OK)
	require.Len(t, doc.Items, 1)
	assert.JSONEq(t, `{"subject":"feat/a","status":"skipped","reason":"interrupted","detail":""}`, string(doc.Items[0]))
	require.NotNil(t, doc.Error)
	assert.Equal(t, string(kernel.KindCancelled), doc.Error.Kind)
}

func TestACommandsOwnDetailShowsItsPlanAndItsDoneItems(t *testing.T) {
	w := newWorld(t, worldParams{Detail: pathDetail{}})

	preview := w.run(t, args(commandName, "feat/a", "feat/b", "--dry-run"))
	applied := w.run(t, args(commandName, "feat/a", "--yes"))

	assert.Contains(t, preview.Stdout, "2 to make")
	assert.Contains(t, applied.Stdout, "feat/a  /wt/feat/a")
}
