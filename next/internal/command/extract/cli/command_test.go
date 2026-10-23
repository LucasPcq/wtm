package cli

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var conflicting = map[string]worktree.ConflictKind{"apps/api/a.txt": worktree.ConflictModified}

func TestTheHelpOfExtractIsTheLegacyOnesWithNextsContract(t *testing.T) {
	got := runExtract(t, newWorld(seed{}), "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "extract_help", got.Stdout)
}

func TestADryRunOfANewTargetShowsItsConflictsAndTheLineThatAppliesIt(t *testing.T) {
	got := runExtract(t, newWorld(seed{Conflicts: conflicting}), source, "--files", "apps/api", "--to", split, "--on-conflict", "resolve", "--dry-run")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "dryrun", got.Stdout)
}

func TestARenamedFileKeepsItsOldPathInTheRecap(t *testing.T) {
	got := runExtract(t, newWorld(seed{}), source, "--files", "DOC.md", "--to", other, "--dry-run", "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.Contains(t, got.Stdout, `"note": "renamed",
            "params": {
              "orig_path": "README.md"
            },`)
}

func TestAMoveToAnExistingWorktreeListsEachFileAndPointsAtIt(t *testing.T) {
	got := runExtract(t, newWorld(seed{}), source, "--files", "apps/api,DOC.md,old.txt", "--to", other)

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "moved", got.Stdout)
}

func TestACopyToANewBranchKeepsTheSourceAndShowsWhereTheTargetIs(t *testing.T) {
	w := newWorld(seed{})

	got := runExtract(t, w, source, "--files", "apps/api/new.txt", "--to", split, "--from", source, "--keep")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "copied", got.Stdout)
	assert.Equal(t, []string{"apps/api/new.txt"}, paths(w.git.Moves()[0].Changes))
	assert.True(t, w.git.Moves()[0].Keep)
}

func TestAResolvedConflictNamesWhatToResolveAndHowToUndo(t *testing.T) {
	got := runExtract(t, newWorld(seed{Conflicts: conflicting}), source, "--files", "apps/api", "--to", other, "--on-conflict", "resolve")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "resolved", got.Stdout)
}

func TestACleanSourceHasNothingToExtract(t *testing.T) {
	got := runExtract(t, newWorld(seed{Clean: true}), source, "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "nothing", got.Stdout)
}

func TestAConflictIsRefusedOnItsOwnExitCodeBeforeAnythingMoves(t *testing.T) {
	w := newWorld(seed{Conflicts: conflicting})

	got := runExtract(t, w, source, "--files", "apps/api", "--to", other)

	assert.Equal(t, exitConflict, got.Code)
	assert.Empty(t, got.Stdout)
	assert.Equal(t, "\n  ✗ cannot apply the selected changes: apps/api/a.txt was also modified in \"feat/other\" — resolve it there, extract to another worktree, or re-run with --on-conflict resolve\n\n", got.Stderr)
	assert.Empty(t, w.git.Moves())
}

func TestAConflictRefusedInJSONIsTheEnvelopesError(t *testing.T) {
	got := runExtract(t, newWorld(seed{Conflicts: conflicting}), source, "--files", "apps/api", "--to", other, "--output", "json")

	assert.Equal(t, exitConflict, got.Code)
	doc := decoded(t, got.Stdout)
	require.NotNil(t, doc.Error)
	assert.False(t, doc.OK)
	assert.Equal(t, "extract.conflict", doc.Error.Code)
	assert.Equal(t, "conflict", doc.Error.Kind)
}

func TestMissingFilesAreRequiredAndNamedByTheirFlag(t *testing.T) {
	got := runExtract(t, newWorld(seed{}), source, "--to", other)

	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Equal(t, "\n  ✗ the request is invalid\n    --files is required\n\n", got.Stderr)
}

func TestMissingFilesInJSONNameTheFlag(t *testing.T) {
	got := runExtract(t, newWorld(seed{}), source, "--to", other, "--output", "json")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	doc := decoded(t, got.Stdout)
	require.NotNil(t, doc.Error)
	assert.Equal(t, "required", doc.Error.Fields[0].Code)
	assert.Equal(t, "--files", doc.Error.Fields[0].Flag)
}

func TestASuccessInJSONIsOneItemWithItsDetail(t *testing.T) {
	got := runExtract(t, newWorld(seed{}), source, "--files", "apps/api/new.txt", "--to", other, "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stdout)
	doc := decoded(t, got.Stdout)
	assert.Equal(t, 2, doc.V)
	assert.Equal(t, "extract", doc.Command)
	assert.True(t, doc.OK)
	require.Len(t, doc.Items, 1)
	assert.Equal(t, other, doc.Items[0].Subject)
	assert.Equal(t, "done", doc.Items[0].Status)
	assert.JSONEq(t, `{"path": "/repo/wt-feat-other", "files": [{"path": "apps/api/new.txt", "status": "untracked"}], "kept": false, "conflicts": [], "isolation": "isolated", "existing_branch": false}`, string(doc.Items[0].Detail))
}

func TestADryRunInJSONIsThePlanAndItsFingerprint(t *testing.T) {
	got := runExtract(t, newWorld(seed{}), source, "--files", "old.txt", "--to", split, "--keep", "--dry-run", "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stdout)
	doc := decoded(t, got.Stdout)
	require.NotNil(t, doc.Plan)
	assert.Equal(t, source, doc.Plan.Source)
	assert.Equal(t, split, doc.Plan.Target.Branch)
	assert.Equal(t, "create", doc.Plan.Target.Action)
	assert.True(t, doc.Plan.Keep)
	assert.Len(t, doc.Fingerprint, len(fingerprint))
	assert.Empty(t, doc.Items)
}

func TestWithoutConfigExtractExitsOnItsOwnCode(t *testing.T) {
	got := runExtract(t, newWorld(seed{NoConfig: true}), source, "--files", "old.txt", "--to", other)

	assert.Equal(t, uicli.ExitConfigNotFound, got.Code)
	assert.Contains(t, got.Stderr, "→ wtm init")
}

func TestAnUnknownConflictModeIsAUsageErrorNamingItsFlag(t *testing.T) {
	got := runExtract(t, newWorld(seed{}), source, "--files", "old.txt", "--to", other, "--on-conflict", "theirs")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Contains(t, got.Stderr, "--on-conflict must be one of abort, resolve, not theirs")
}

func TestASecondPositionalIsAUsageError(t *testing.T) {
	got := runExtract(t, newWorld(seed{}), source, other)

	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Contains(t, got.Stderr, "accepts at most 1 arg(s), received 2")
}

func TestAStalePlanIsRefusedAndPointsAtThePlanTheFactsGiveNow(t *testing.T) {
	got := runExtract(t, newWorld(seed{}), source, "--files", "old.txt", "--to", other, "--expect-fingerprint", fingerprint)

	assert.Equal(t, uicli.ExitPrecondition, got.Code)
	assert.Contains(t, got.Stderr, "✗ the facts changed since the preview")
	assert.Contains(t, got.Stderr, "→ wtm-next extract feat/login --files old.txt --to feat/other --dry-run   the plan the facts give now")
}

func TestAFailedHookInTheCreatedTargetExitsOneAndNamesIt(t *testing.T) {
	w := newWorld(seed{OnCreate: []project.Hook{{Cmd: "make setup"}}, RunHooks: hookFails})

	got := runExtract(t, w, source, "--files", "old.txt", "--to", split)

	assert.Equal(t, uicli.ExitError, got.Code)
	assert.Contains(t, got.Stdout, "✗ feat/split  on_create failed")
	assert.NotContains(t, got.Stdout, "wtm clean")
	assert.Empty(t, w.git.Moves())
}

func TestATargetLeftHalfCreatedPointsAtTheCleanThatRemovesIt(t *testing.T) {
	w := newWorld(seed{GitFails: map[string]error{"RemoveWorktree": errors.New("busy")}, StoreFails: map[string]error{"Update": errors.New("disk full")}})

	got := runExtract(t, w, source, "--files", "old.txt", "--to", split)

	assert.Equal(t, uicli.ExitError, got.Code)
	assert.Contains(t, got.Stdout, "→ wtm clean feat/split --force   branch feat/split is left half-created at /repo/.trees/feat-split")
	assert.Empty(t, w.git.Moves())
}

func TestQuietSilencesASuccess(t *testing.T) {
	got := runExtract(t, newWorld(seed{}), "-q", source, "--files", "old.txt", "--to", other)

	require.Equal(t, uicli.ExitOK, got.Code)
	assert.Empty(t, got.Stdout)
	assert.Empty(t, got.Stderr)
}

func TestAFlagErrorPointsAtTheHelp(t *testing.T) {
	got := runExtract(t, newWorld(seed{}), "--bogus")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Equal(t, "\n  ✗ the request is invalid: unknown flag: --bogus\n  → wtm-next extract --help\n\n", got.Stderr)
}

func TestAUsageErrorIsInTheEnvelopeOnlyWhenTheArgumentsAskForJSON(t *testing.T) {
	cases := map[string]struct {
		Words   []string
		JSON    bool
		Message string
	}{
		"--output json after an unknown flag":      {Words: []string{source, "--nope", "--output", "json"}, JSON: true, Message: "unknown flag: --nope"},
		"a second positional with --output json":   {Words: []string{source, other, "--output", "json"}, JSON: true, Message: "accepts at most 1 arg(s), received 2"},
		"an unknown format before an unknown flag": {Words: []string{source, "--output", "yaml", "--nope"}, Message: "unknown flag: --nope"},
	}
	for name, c := range cases {
		got := runExtract(t, newWorld(seed{}), c.Words...)

		assert.Equal(t, uicli.ExitUsage, got.Code, name)
		if !c.JSON {
			assert.Empty(t, got.Stdout, name)
			assert.Equal(t, "\n  ✗ the request is invalid: "+c.Message+"\n  → wtm-next extract --help\n\n", got.Stderr, name)
			continue
		}
		assert.Empty(t, got.Stderr, name)
		doc := decoded(t, got.Stdout)
		assert.Equal(t, "extract", doc.Command, name)
		require.NotNil(t, doc.Error, name)
		assert.Equal(t, "the request is invalid: "+c.Message, doc.Error.Message, name)
	}
}

func TestFFAndKeepEnterTheRequestOnlyWhenGiven(t *testing.T) {
	cases := map[string]struct {
		Flags []string
		Line  string
	}{
		"neither":  {Line: "→ wtm-next extract feat/login --files old.txt --to feat/other --expect-fingerprint"},
		"both off": {Flags: []string{"--ff=false", "--keep=false"}, Line: "→ wtm-next extract feat/login --ff=false --files old.txt --keep=false --to feat/other --expect-fingerprint"},
		"both on":  {Flags: []string{"--ff", "--keep"}, Line: "→ wtm-next extract feat/login --ff --files old.txt --keep --to feat/other --expect-fingerprint"},
	}
	for name, c := range cases {
		words := append([]string{source, "--files", "old.txt", "--to", other, "--dry-run"}, c.Flags...)
		got := runExtract(t, newWorld(seed{}), words...)

		require.Equal(t, uicli.ExitOK, got.Code, name+": "+got.Stderr)
		assert.Contains(t, got.Stdout, c.Line, name)
	}
}

func TestAnUnknownOutputFormatIsRefusedAsText(t *testing.T) {
	got := runExtract(t, newWorld(seed{}), source, "--files", "old.txt", "--to", other, "--output", "yaml")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Empty(t, got.Stdout)
	assert.Equal(t, "\n  ✗ invalid --output \"yaml\": expected one of text, json\n  → wtm-next extract --help\n\n", got.Stderr)
}

func TestTheCommandOpensTheProjectDirAndRunsWithTheCorrelationID(t *testing.T) {
	w := newWorld(seed{Env: uicli.Env{Cwd: mainPath, ProjectDir: "/named", CorrelationID: "corr-1"}})

	got := runExtract(t, w, source, "--files", "apps/api/new.txt", "--to", split, "--from", source, "--keep")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.Equal(t, []string{"/named"}, w.seen.dirs)
	require.Len(t, w.seen.ops, 1)
	assert.Equal(t, "corr-1", w.seen.ops[0].CorrelationID)
}

func TestTheEnvelopeOfEachEndingIsByteForByte(t *testing.T) {
	for name, c := range map[string]struct {
		Seed  seed
		Words []string
		Code  int
	}{
		"success":  {Words: []string{source, "--files", "apps/api/new.txt", "--to", other}, Code: uicli.ExitOK},
		"dry_run":  {Words: []string{source, "--files", "old.txt", "--to", split, "--dry-run"}, Code: uicli.ExitOK},
		"conflict": {Seed: seed{Conflicts: conflicting}, Words: []string{source, "--files", "apps/api", "--to", other}, Code: exitConflict},
		"stale":    {Words: []string{source, "--files", "old.txt", "--to", other, "--expect-fingerprint", fingerprint}, Code: uicli.ExitPrecondition},
		"invalid":  {Words: []string{source, "--to", other}, Code: uicli.ExitUsage},
		"nothing":  {Seed: seed{Clean: true}, Words: []string{source, "--yes"}, Code: uicli.ExitOK},
		"resolved": {Seed: seed{Conflicts: conflicting}, Words: []string{source, "--files", "apps/api", "--to", other, "--on-conflict", "resolve"}, Code: uicli.ExitOK},
	} {
		t.Run(name, func(t *testing.T) {
			got := runExtract(t, newWorld(c.Seed), append(c.Words, "--output", "json")...)

			assert.Equal(t, c.Code, got.Code, got.Stdout)
			assert.Empty(t, got.Stderr)
			formtest.Golden(t, "envelope_"+name, got.Stdout)
		})
	}
}
