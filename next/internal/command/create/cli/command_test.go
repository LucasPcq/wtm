package cli

import (
	"errors"
	"strings"
	"testing"

	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

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
	formtest.Golden(t, "create_help", got.Stdout)
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

	assert.Equal(t, uicli.ExitConfigNotFound, got.Code)
	assert.Contains(t, got.Stderr, "→ wtm init")
}

func TestAnInvalidConfigIsReportedAsSuch(t *testing.T) {
	w := newWorld(seed{ConfigErr: &project.InvalidConfigError{Reason: errors.New("base_path must be relative to the repository: /tmp/trees")}})

	human := runCreate(t, w, branchName, "--yes")
	machine := runCreate(t, w, branchName, "--yes", "--output", "json")

	assert.Equal(t, uicli.ExitError, human.Code)
	assert.Contains(t, human.Stderr, "✗ invalid config.toml: base_path must be relative to the repository: /tmp/trees")
	assert.NotContains(t, human.Stderr, "internal error")
	assert.Equal(t, uicli.ExitError, machine.Code)
	doc := decoded(t, machine.Stdout)
	require.NotNil(t, doc.Error)
	assert.Equal(t, formtest.Error{Kind: "precondition", Code: "create.config_invalid", Message: "invalid config.toml: base_path must be relative to the repository: /tmp/trees"}, *doc.Error)
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
	assert.Equal(t, "\n  ✗ the request is invalid: branch feat/x is checked out at /elsewhere/feat-x\n  → wtm go feat/x   jump in\n\n", got.Stderr)
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
	assert.NotContains(t, got.Stdout, "wtm clean")
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
	assert.Contains(t, got.Stdout, "→ wtm clean feat/x --force   branch feat/x is left half-created at /repo/.trees/feat-x")
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

func TestAFlagErrorIsInTheEnvelopeOnlyWhenTheArgumentsAskForJSON(t *testing.T) {
	cases := map[string]struct {
		Words []string
		JSON  bool
	}{
		"--output json after the bad flag":       {Words: []string{branchName, "--nope", "--output", "json"}, JSON: true},
		"--output=json after the bad flag":       {Words: []string{branchName, "--nope", "--output=json"}, JSON: true},
		"an unknown format before the bad flag":  {Words: []string{branchName, "--output", "yaml", "--nope"}},
		"json overridden by a later --output":    {Words: []string{branchName, "--nope", "--output", "json", "--output", "yaml"}},
		"--output json only after the separator": {Words: []string{branchName, "--nope", "--", "--output", "json"}},
	}
	for name, c := range cases {
		got := runCreate(t, newWorld(seed{}), c.Words...)

		assert.Equal(t, uicli.ExitUsage, got.Code, name)
		if !c.JSON {
			assert.Empty(t, got.Stdout, name)
			assert.Equal(t, "\n  ✗ the request is invalid: unknown flag: --nope\n  → wtm-next create --help\n\n", got.Stderr, name)
			continue
		}
		assert.Empty(t, got.Stderr, name)
		doc := decoded(t, got.Stdout)
		require.NotNil(t, doc.Error, name)
		assert.Equal(t, "the request is invalid: unknown flag: --nope", doc.Error.Message, name)
	}
}

func TestTheCommandOpensTheProjectDirAndRunsWithTheCorrelationID(t *testing.T) {
	w := newWorld(seed{Env: uicli.Env{Cwd: "/elsewhere", ProjectDir: "/named", CorrelationID: "corr-1"}})

	got := runCreate(t, w, "feat/a", "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.Equal(t, []string{"/named"}, w.seen.dirs)
	require.Len(t, w.seen.ops, 1)
	assert.Equal(t, "corr-1", w.seen.ops[0].CorrelationID)
	assert.Contains(t, got.Stdout, "../repo/.trees/feat-a", "paths are shown from Env.Cwd")
}

func TestAnInvalidCorrelationIDIsRefusedBeforeTheProjectIsOpened(t *testing.T) {
	w := newWorld(seed{Env: uicli.Env{Cwd: mainPath, ProjectDir: mainPath, CorrelationID: "bad\x01id"}})

	got := runCreate(t, w, "feat/a", "--yes")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Contains(t, got.Stderr, "$WTM_CORRELATION_ID contains a control character: invalid correlation id")
	assert.Empty(t, w.seen.dirs)
}

func TestTheEnvelopeOfEachEndingIsByteForByte(t *testing.T) {
	hooked := seed{OnCreate: []project.Hook{{Cmd: "make setup"}}, RunHooks: hookFails}
	for name, c := range map[string]struct {
		Seed  seed
		Words []string
		Code  int
	}{
		"success":     {Words: []string{branchName}, Code: uicli.ExitOK},
		"invalid":     {Words: []string{"feat/a", "  ", "--env-from", "copy"}, Code: uicli.ExitUsage},
		"hook_failed": {Seed: hooked, Words: []string{branchName}, Code: uicli.ExitError},
		"dry_run":     {Words: []string{branchName, "--dry-run"}, Code: uicli.ExitOK},
		"stale":       {Words: []string{branchName, "--expect-fingerprint", fingerprint}, Code: uicli.ExitPrecondition},
		"usage":       {Words: []string{branchName, "--nope"}, Code: uicli.ExitUsage},
	} {
		t.Run(name, func(t *testing.T) {
			got := runCreate(t, newWorld(c.Seed), append(c.Words, "--output", "json")...)

			assert.Equal(t, c.Code, got.Code, got.Stdout)
			assert.Empty(t, got.Stderr)
			formtest.Golden(t, "envelope_"+name, got.Stdout)
		})
	}
}
