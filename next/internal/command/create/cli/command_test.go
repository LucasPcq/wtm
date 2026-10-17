package cli

import (
	"cmp"
	"errors"
	"strings"
	"testing"

	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"

	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfCreateIsTheEnginesWordForWord(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	golden(t, "create_help", got.Stdout)
}

func TestCreateConcludesWithEachWorktreeRelativeToWhereItRuns(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), "feat/a", "feat/b")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, "create: 2 done")
	assert.Contains(t, got.Stdout, "feat/a    ../.trees/feat-a")
	assert.Contains(t, got.Stdout, "→ wtm go feat/b")
}

func TestEachFlagReachesTheRequest(t *testing.T) {
	w := newWorld(seed{Behind: true})

	got := runCreate(t, w, " feat/a ", "--from", "main", "--env-from", "main", "--ff", "--remember", "env_from", "--dry-run", "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stdout)
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

	require.Equal(t, uicli.ExitOK, got.Code, got.Stdout)
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

	assert.Equal(t, uicli.ExitUsage, got.Code)
	doc := decoded(t, got.Stdout)
	require.NotNil(t, doc.Error)
	assert.Equal(t, "required", doc.Error.Fields[0].Code)
	assert.Equal(t, "<branches>[1]", doc.Error.Fields[0].Flag)
}

func TestJSONNeedsNoYesAndIsTheEnvelope(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), branchName, "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stdout)
	doc := decoded(t, got.Stdout)
	assert.True(t, doc.OK)
	assert.Equal(t, "op-1", doc.Op)
	assert.Equal(t, "done", doc.Items[0].Status)
}

func TestWithoutABranchTheUsageErrorNamesTheArgument(t *testing.T) {
	got := runCreate(t, newWorld(seed{}))

	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Equal(t, "\n  ✗ the request is invalid\n    <branches> is required\n\n", got.Stderr)
	assert.Empty(t, got.Stdout)
}

func TestAnUnknownOutputFormatIsAUsageError(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), branchName, "--output", "xml")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Contains(t, got.Stderr, `invalid --output "xml": expected one of text, json`)
}

func TestAnUnknownFlagIsAUsageErrorInTheEnvelopeWhenJSONWasAsked(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), branchName, "--output", "json", "--nope")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	doc := decoded(t, got.Stdout)
	assert.Equal(t, "the request is invalid: unknown flag: --nope", doc.Error.Message)
	assert.Empty(t, doc.Op)
}

func TestARefusedIsolationIsAUsageError(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), branchName, "--isolation", "keep", "--output", "json")

	assert.Equal(t, uicli.ExitUsage, got.Code)
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
	held := &domain.Error{Kind: domain.KindConflict, Code: "lock.held", Message: "repo is held by create (pid 42)"}

	got := runCreate(t, newWorld(seed{Locks: held}), branchName)

	assert.Equal(t, uicli.ExitConflict, got.Code)
	assert.Contains(t, got.Stderr, "✗ repo is held by create (pid 42)")
}

func TestADryRunEndsOnTheLineThatAppliesExactlyItsPlan(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), branchName, "--from", "main", "--ff=false", "--ask", "--dry-run")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, "create · dry run, nothing applied")
	assert.Regexp(t, `→ wtm-next create feat/x --ff=false --from main --ask --expect-fingerprint [0-9a-f]{24}   apply this plan`, got.Stdout)
}

func TestAStalePlanIsRefusedAndPointsAtThePlanTheFactsGiveNow(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), branchName, "--expect-fingerprint", fingerprint)

	assert.Equal(t, uicli.ExitPrecondition, got.Code)
	assert.Contains(t, got.Stderr, "✗ the facts changed since the preview")
	assert.Contains(t, got.Stderr, "→ wtm-next create feat/x --dry-run   the plan the facts give now")
}

func TestQuietPrintsOnlyTheOneFailureLine(t *testing.T) {
	got := runCreate(t, newWorld(seed{Linked: []string{branchName}}), "-q", branchName)

	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Empty(t, got.Stdout)
	assert.Equal(t, "\n  ✗ the request is invalid: branch feat/x is checked out at /elsewhere/feat-x — run `wtm go feat/x` to jump in\n\n", got.Stderr)
}

func TestQuietLeavesTheJSONDocument(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), "-q", branchName, "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code)
	assert.True(t, decoded(t, got.Stdout).OK)
}

func TestAFailedHookExitsOneAndNamesTheBranch(t *testing.T) {
	w := newWorld(seed{OnCreate: []project.Hook{{Cmd: "make setup"}}, RunHooks: hookFails})

	got := runCreate(t, w, branchName)

	assert.Equal(t, uicli.ExitError, got.Code)
	assert.Contains(t, got.Stdout, "✗ create: 1 failed")
	assert.Contains(t, got.Stdout, "feat/x  on_create failed")
}

func TestTheProjectThatCannotBeOpenedIsTheFailure(t *testing.T) {
	notARepo := &domain.Error{Kind: domain.KindPrecondition, Code: domain.CodeNotARepository, Message: "/tmp is not a git repository"}

	got := runCreate(t, newWorld(seed{Open: notARepo}), branchName)

	assert.Equal(t, uicli.ExitNotGitRepo, got.Code)
	assert.Contains(t, got.Stderr, "✗ /tmp is not a git repository")
}

func TestAWorktreeLeftHalfCreatedPointsAtTheCleanThatRemovesIt(t *testing.T) {
	w := newWorld(seed{GitFails: map[string]error{"RemoveWorktree": errors.New("busy")}, StoreFails: map[string]error{"Update": errors.New("disk full")}})

	got := runCreate(t, w, branchName)

	assert.Equal(t, uicli.ExitError, got.Code)
	assert.Contains(t, got.Stdout, "→ wtm clean feat/x --force   branch feat/x is left half-created at /repo/.trees/feat-x: remove it with `wtm clean feat/x --force`")
}

func TestARunAsksOnlyWhenTheUserCanSeeTheQuestionsAndAnswerThem(t *testing.T) {
	cases := map[string]struct {
		In, Err, Dumb, Yes bool
		Output, Expect     string
		Asks               bool
	}{
		"stdin and stderr terminals": {In: true, Err: true, Asks: true},
		"stdin piped":                {Err: true},
		"stderr redirected":          {In: true},
		"TERM=dumb":                  {In: true, Err: true, Dumb: true},
		"--yes":                      {In: true, Err: true, Yes: true},
		"--output json":              {In: true, Err: true, Output: uicli.OutputJSON},
		"--expect-fingerprint":       {In: true, Err: true, Expect: fingerprint},
	}
	for name, c := range cases {
		streams, _, _, _ := output.Test()
		streams.SetStdinTerminal(c.In)
		streams.Err.SetTerminal(c.Err)
		streams.SetDumbTerminal(c.Dumb)
		cmd := &createCommand{io: streams, output: cmp.Or(c.Output, uicli.OutputText), yes: c.Yes, expect: c.Expect}

		assert.Equal(t, c.Asks, cmd.interactive(), name)
	}
}

func TestTheProgressDrawsLiveOnlyOnAStderrTerminalOfAKnownKind(t *testing.T) {
	cases := map[string]struct {
		SetUp func(*output.Streams)
		Live  bool
	}{
		"stderr a terminal":       {SetUp: func(s *output.Streams) { s.Err.SetTerminal(true) }, Live: true},
		"stderr a dumb terminal":  {SetUp: func(s *output.Streams) { s.Err.SetTerminal(true); s.SetDumbTerminal(true) }},
		"stderr redirected":       {SetUp: func(*output.Streams) {}},
		"stdout alone a terminal": {SetUp: func(s *output.Streams) { s.Out.SetTerminal(true) }},
	}
	for name, c := range cases {
		got := runOn(t, newWorld(seed{}), c.SetUp, "create", branchName)

		require.Equal(t, uicli.ExitOK, got.Code, name)
		assert.Equal(t, c.Live, strings.Contains(got.Stderr, "› feat/x"), name)
	}
}

func TestAFlagErrorPointsAtTheHelp(t *testing.T) {
	cases := map[string][]string{
		"an unknown flag":     {"--bogus"},
		"a bad value":         {branchName, "--ff=maybe"},
		"an unknown --output": {branchName, "--output", "yaml"},
	}
	for name, words := range cases {
		got := runCreate(t, newWorld(seed{}), words...)

		assert.Equal(t, uicli.ExitUsage, got.Code, name)
		assert.True(t, strings.HasSuffix(got.Stderr, "\n  → wtm-next create --help\n\n"), name+": "+got.Stderr)
	}
}

func TestDashNIsTheDryRun(t *testing.T) {
	got := runCreate(t, newWorld(seed{}), branchName, "-n")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, "create · dry run, nothing applied")
}
