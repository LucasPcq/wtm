package cli

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfCreateIsTheEnginesWordForWord(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), "--help")

	require.Equal(t, exitOK, got.Code)
	golden(t, "create_help", got.Stdout)
}

func TestCreateConcludesWithEachWorktreeRelativeToWhereItRuns(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), "feat/a", "feat/b")

	require.Equal(t, exitOK, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, "create: 2 done")
	assert.Contains(t, got.Stdout, "feat/a    ../.trees/feat-a")
	assert.Contains(t, got.Stdout, "→ wtm go feat/b")
}

func TestEachFlagReachesTheRequest(t *testing.T) {
	w := newWorld(seed{Behind: true})

	got := runCreate(t, w, " feat/a ", "--from", "main", "--env-from", "main", "--ff", "--remember", "env_from", "--dry-run", "--output", "json")

	require.Equal(t, exitOK, got.Code, got.Stdout)
	plan := decoded(t, got.Stdout).Plan
	require.NotNil(t, plan)
	assert.Equal(t, "feat/a", plan.Branches[0].Branch)
	assert.Equal(t, "main", plan.From)
	assert.Equal(t, "main", plan.EnvFrom)
	assert.Equal(t, []string{"main"}, plan.FastForward)
	assert.Equal(t, []string{"env_from"}, plan.Remember)
}

func TestIfNotExistsLeavesAnOpenWorktreeUnchanged(t *testing.T) {
	got := runCreate(t, newWorld(seed{Linked: []string{branchName}}), branchName, "--if-not-exists", "--output", "json")

	require.Equal(t, exitOK, got.Code, got.Stdout)
	assert.Equal(t, "unchanged", decoded(t, got.Stdout).Items[0].Reason)
}

func TestAskIgnoresTheRememberedAnswers(t *testing.T) {
	w := newWorld(seed{Remembered: map[string]string{project.RememberEnvStrategy: "main"}})

	remembered := decoded(t, runCreate(t, w, branchName, "--dry-run", "--output", "json").Stdout)
	asked := decoded(t, runCreate(t, w, branchName, "--dry-run", "--ask", "--output", "json").Stdout)

	assert.Equal(t, "main", remembered.Plan.EnvFrom)
	assert.Equal(t, "example", asked.Plan.EnvFrom)
}

func TestFFFalseBeatsARememberedFastForward(t *testing.T) {
	w := newWorld(seed{Behind: true, Remembered: map[string]string{project.RememberSourceUpdate: "ff"}})

	remembered := decoded(t, runCreate(t, w, branchName, "--dry-run", "--output", "json").Stdout)
	refused := decoded(t, runCreate(t, w, branchName, "--ff=false", "--dry-run", "--output", "json").Stdout)

	assert.Equal(t, []string{"main"}, remembered.Plan.FastForward)
	assert.Empty(t, refused.Plan.FastForward)
}

func TestABlankBranchIsRequiredAndNamedAsTheCommandLineTakesIt(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), "feat/a", "  ", "--output", "json")

	assert.Equal(t, exitUsage, got.Code)
	doc := decoded(t, got.Stdout)
	require.NotNil(t, doc.Error)
	assert.Equal(t, "required", doc.Error.Fields[0].Code)
	assert.Equal(t, "<branches>[1]", doc.Error.Fields[0].Flag)
}

func TestJSONNeedsNoYesAndIsTheEnvelope(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), branchName, "--output", "json")

	require.Equal(t, exitOK, got.Code, got.Stdout)
	doc := decoded(t, got.Stdout)
	assert.True(t, doc.OK)
	assert.Equal(t, "op-1", doc.Op)
	assert.Equal(t, "done", doc.Items[0].Status)
}

func TestWithoutABranchTheUsageErrorNamesTheArgument(t *testing.T) {
	got := runCreate(t, newWorld(seed{}))

	assert.Equal(t, exitUsage, got.Code)
	assert.Equal(t, "\n  ✗ the request is invalid\n    <branches> is required\n\n", got.Stderr)
	assert.Empty(t, got.Stdout)
}

func TestAnUnknownOutputFormatIsAUsageError(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), branchName, "--output", "xml")

	assert.Equal(t, exitUsage, got.Code)
	assert.Contains(t, got.Stderr, `invalid --output "xml": expected one of text, json`)
}

func TestAnUnknownFlagIsAUsageErrorInTheEnvelopeWhenJSONWasAsked(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), branchName, "--output", "json", "--nope")

	assert.Equal(t, exitUsage, got.Code)
	doc := decoded(t, got.Stdout)
	assert.Equal(t, "the request is invalid: unknown flag: --nope", doc.Error.Message)
	assert.Empty(t, doc.Op)
}

func TestARefusedIsolationIsAUsageError(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), branchName, "--isolation", "keep", "--output", "json")

	assert.Equal(t, exitUsage, got.Code)
	assert.Equal(t, "--isolation", decoded(t, got.Stdout).Error.Fields[0].Flag)
}

func TestWithoutConfigCreateExitsOnItsOwnCode(t *testing.T) {
	w := newWorld(seed{})
	w.store = storetest.NewFake(storetest.FakeParams{})

	got := runCreate(t, w, branchName)

	assert.Equal(t, exitConfigNotFound, got.Code)
	assert.Contains(t, got.Stderr, "run `wtm init` first")
}

func TestAHeldLockIsAConflict(t *testing.T) {
	held := &wtm.Error{Kind: wtm.KindConflict, Code: "lock.held", Message: "repo is held by create (pid 42)"}

	got := runCreate(t, newWorld(seed{Locks: held}), branchName)

	assert.Equal(t, exitConflict, got.Code)
	assert.Contains(t, got.Stderr, "✗ repo is held by create (pid 42)")
}

func TestADryRunEndsOnTheLineThatAppliesExactlyItsPlan(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), branchName, "--from", "main", "--ff=false", "--ask", "--dry-run")

	require.Equal(t, exitOK, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, "create · dry run, nothing applied")
	assert.Regexp(t, `→ wtm-next create feat/x --ff=false --from main --ask --expect-fingerprint [0-9a-f]{24}   apply this plan`, got.Stdout)
}

func TestAStalePlanIsRefusedAndPointsAtThePlanTheFactsGiveNow(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), branchName, "--expect-fingerprint", fingerprint)

	assert.Equal(t, exitPrecondition, got.Code)
	assert.Contains(t, got.Stderr, "✗ the facts changed since the preview")
	assert.Contains(t, got.Stderr, "→ wtm-next create feat/x --dry-run   the plan the facts give now")
}

func TestQuietPrintsOnlyTheOneFailureLine(t *testing.T) {
	got := runCreate(t, newWorld(seed{Linked: []string{branchName}}), "-q", branchName)

	assert.Equal(t, exitUsage, got.Code)
	assert.Empty(t, got.Stdout)
	assert.Equal(t, "\n  ✗ the request is invalid: branch feat/x is checked out at /elsewhere/feat-x — run `wtm go feat/x` to jump in\n\n", got.Stderr)
}

func TestQuietLeavesTheJSONDocument(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), "-q", branchName, "--output", "json")

	require.Equal(t, exitOK, got.Code)
	assert.True(t, decoded(t, got.Stdout).OK)
}

func TestAFailedHookExitsOneAndNamesTheBranch(t *testing.T) {
	w := newWorld(seed{OnCreate: []project.Hook{{Cmd: "make setup"}}, RunHooks: hookFails})

	got := runCreate(t, w, branchName)

	assert.Equal(t, exitError, got.Code)
	assert.Contains(t, got.Stdout, "✗ create: 1 failed")
	assert.Contains(t, got.Stdout, "feat/x  on_create failed")
}

func TestTheProjectThatCannotBeOpenedIsTheFailure(t *testing.T) {
	notARepo := &wtm.Error{Kind: wtm.KindPrecondition, Code: codeNotARepository, Message: "/tmp is not a git repository"}

	got := runCreate(t, newWorld(seed{Open: notARepo}), branchName)

	assert.Equal(t, exitNotGitRepo, got.Code)
	assert.Contains(t, got.Stderr, "✗ /tmp is not a git repository")
}

func TestAWorktreeLeftHalfCreatedPointsAtTheCleanThatRemovesIt(t *testing.T) {
	w := newWorld(seed{GitFails: map[string]error{"RemoveWorktree": errors.New("busy")}, StoreFails: map[string]error{"Update": errors.New("disk full")}})

	got := runCreate(t, w, branchName)

	assert.Equal(t, exitError, got.Code)
	assert.Contains(t, got.Stdout, "→ wtm clean feat/x --force   branch feat/x is left half-created at /repo/.trees/feat-x: remove it with `wtm clean --force feat/x`")
}
