package form_test

import (
	"cmp"
	"encoding/json"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

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
		"--expect-fingerprint":       {In: true, Err: true, Expect: "print"},
	}
	for name, c := range cases {
		streams, _, _, _ := output.Test()
		streams.SetStdinTerminal(c.In)
		streams.Err.SetTerminal(c.Err)
		streams.SetDumbTerminal(c.Dumb)
		command := fakeCommand()
		command.SetRun(&uicli.Run{Streams: streams, Output: cmp.Or(c.Output, uicli.OutputText)}, c.Yes, c.Expect)

		assert.Equal(t, c.Asks, command.Interactive(), name)
	}
}

func TestEachFieldIsAFlagOfItsKindAndTheSharedFlagsFollow(t *testing.T) {
	_, cmd := bound(t)
	types := map[string]string{}
	cmd.Flags().VisitAll(func(f *pflag.Flag) { types[f.Name] = f.Value.Type() })

	assert.Equal(t, map[string]string{
		"from": "string", "mode": "string", "ff": "bool", "keep": "bool", "files": "stringSlice", "name": "string",
		"remember": "stringArray", "ask": "bool", "yes": "bool", "output": "string", "dry-run": "bool", "expect-fingerprint": "string",
	}, types)
	assert.Equal(t, "The source", cmd.Flags().Lookup("from").Usage)
	assert.Equal(t, "The answers to always use in this repository: mode, ff (repeatable)", cmd.Flags().Lookup("remember").Usage)
	assert.Equal(t, "y", cmd.Flags().Lookup("yes").Shorthand)
}

func TestTheFlagsGivenAndTheArgumentsAnswerTheRequest(t *testing.T) {
	command, cmd := bound(t, "--from", "main", "--ff=false", "--files", "a.txt,b.txt", "--remember", "mode", "--ask")

	req := command.RequestOf(cmd, []string{"feat/a", " feat/b "})

	assert.Equal(t, request{"branches": {"feat/a", "feat/b"}, "from": {"main"}, "ff": {"false"}, "files": {"a.txt", "b.txt"}, domain.PathRemember: {"mode"}, domain.PathAsk: {"true"}}, req)
}

func TestNoFlagGivenIsAnEmptyRequest(t *testing.T) {
	command, cmd := bound(t)

	assert.Empty(t, command.RequestOf(cmd, nil))
}

func TestTheCommandLineSpellsThePositionalWordsThenTheFlagsSortedThenAsk(t *testing.T) {
	req := request{"branches": {"feat/a", "it's"}, "name": {"two words"}, "ff": {"false"}, "keep": {"true"}, "files": {"a.txt", "b.txt"}, "from": {"main"}, domain.PathRemember: {"mode"}, domain.PathAsk: {"true"}}

	argv := fakeCommand().Argv("wtm-next fake", req)

	assert.Equal(t, []string{"wtm-next", "fake", "feat/a", "it's", "--ff=false", "--files", "a.txt", "--files", "b.txt", "--from", "main", "--keep", "--name", "two words", "--remember", "mode", "--ask"}, argv)
	assert.Equal(t, `wtm-next fake feat/a 'it'\''s' --ff=false --files a.txt --files b.txt --from main --keep --name 'two words' --remember mode --ask`, output.Line(argv))
}

func TestTheWordsAfterTheDashAnswerTheTrailingFieldAsOneLine(t *testing.T) {
	command, cmd := boundLines(t, "feat/a", "--all", "--", "pnpm", "test", "--watch")

	req := command.RequestOf(cmd, cmd.Flags().Args())

	assert.Equal(t, request{"worktree": {"feat/a"}, "all": {"true"}, "command": {"pnpm test --watch"}}, req)
	assert.Nil(t, cmd.Flags().Lookup("command"), "the trailing field is no flag")
}

func TestTheTrailingFieldIsSpelledLastEveryFlagBeforeIt(t *testing.T) {
	followUp := domain.FollowUp{Command: "run", Answers: []domain.Answered{{Path: "worktree", Values: []string{"feat/a"}}, {Path: "command", Values: []string{"pnpm test"}}}, DryRun: true}

	argv := linesCommand().Spell("wtm-next run", followUp)

	assert.Equal(t, `wtm-next run feat/a --dry-run -- 'pnpm test'`, output.Line(argv))
}

func TestAListEntryWithACommaReadsBackAsOneEntry(t *testing.T) {
	req := request{"files": {"a,b.txt", `say "hi".txt`, "c.txt"}}
	argv := fakeCommand().Argv("wtm-next fake", req)

	command, cmd := bound(t, argv[2:]...)

	assert.Equal(t, req, command.RequestOf(cmd, nil))
}

func TestADryRunEndsOnTheLineThatAppliesExactlyItsPlan(t *testing.T) {
	req := request{"branches": {"feat/a"}}

	got, failure := concluded(t, uicli.OutputText, form.Ran[request, outcome]{Request: req, Outcome: outcome{Plan: req, Fingerprint: "print"}}, nil)

	require.NoError(t, failure)
	assert.Contains(t, got, "fake · dry run, nothing applied")
	assert.Contains(t, got, "plan of  feat/a")
	assert.Contains(t, got, "→ fake feat/a --expect-fingerprint print")
}

func TestAnApplyIsDrawnByTheCommandAndEndsOnItsFailure(t *testing.T) {
	req := request{"branches": {"feat/a"}}
	failed := &domain.Error{Kind: domain.KindInternal, Code: "fake.failed", Message: "failed"}
	applied := outcome{Plan: req, Result: &domain.Result[struct{}]{Items: []domain.Unit[struct{}]{{Subject: "feat/a", Status: domain.StatusFailed, Err: failed}}}}

	got, failure := concluded(t, uicli.OutputText, form.Ran[request, outcome]{Request: req, Outcome: applied}, nil)

	assert.Contains(t, got, "drew feat/a from /repo")
	var refused *uicli.Failed
	require.ErrorAs(t, failure, &refused)
	assert.Equal(t, failed, refused.Err)
}

func TestAStalePlanIsFollowedByTheDryRunThatPlansAgainQuietOrNot(t *testing.T) {
	stale := &domain.Error{Kind: domain.KindPrecondition, Code: domain.CodePlanStale, Message: "stale"}
	ran := form.Ran[request, outcome]{Request: request{"branches": {"it's"}}}
	replan := output.NextStep{Command: `fake 'it'\''s' --dry-run`, Note: domain.ReplanNote}

	printed, _ := refused(t, false, ran, stale)
	_, quiet := refused(t, true, ran, stale)
	_, held := refused(t, true, ran, &domain.Error{Kind: domain.KindConflict, Code: "lock.held", Message: "held"})

	assert.Contains(t, printed, "→ "+replan.Command+"   "+replan.Note)
	assert.Equal(t, []output.NextStep{replan}, quiet)
	assert.Empty(t, held)
}

func TestAFollowUpNamingTheCommandItselfIsSpelledFromItsSchemaQuietOrNot(t *testing.T) {
	conflict := &domain.Error{Kind: domain.KindConflict, Code: "fake.conflict", Message: "conflict", FollowUp: &domain.FollowUp{Command: "fake", Answers: []domain.Answered{{Path: "branches", Values: []string{"feat/a"}}, {Path: "mode", Values: []string{"resolve"}}}, Message: "resolve"}}
	resolve := output.NextStep{Command: "fake feat/a --mode resolve", Note: "resolve"}

	printed, _ := refused(t, false, form.Ran[request, outcome]{}, conflict)
	_, quiet := refused(t, true, form.Ran[request, outcome]{}, conflict)

	assert.Contains(t, printed, "→ "+resolve.Command+"   "+resolve.Note)
	assert.Equal(t, []output.NextStep{resolve}, quiet)
}

func TestAFollowUpItsCommandCannotFoldIsNoStepAndAForeignOneIsNeverFolded(t *testing.T) {
	unfoldable := &domain.Error{Kind: domain.KindConflict, Code: "fake.conflict", Message: "conflict", FollowUp: &domain.FollowUp{Command: "fake", Answers: []domain.Answered{{Path: "nope", Values: []string{"x"}}}, Message: "resolve"}}
	foreign := &domain.Error{Kind: domain.KindConflict, Code: "fake.held", Message: "held", FollowUp: &domain.FollowUp{Command: "go", Answers: []domain.Answered{{Path: "nope", Values: []string{"x"}}}}}

	unfolded, _ := refused(t, false, form.Ran[request, outcome]{}, unfoldable)
	spelled, _ := refused(t, false, form.Ran[request, outcome]{}, foreign)

	assert.NotContains(t, unfolded, "→")
	assert.Contains(t, spelled, "→ wtm go --nope x")
}

func TestAStalePlanInJSONFollowsUpWithItsReplan(t *testing.T) {
	stale := &domain.Error{Kind: domain.KindConflict, Code: domain.CodePlanStale, Message: "stale"}

	got, _ := concluded(t, uicli.OutputJSON, form.Ran[request, outcome]{Request: request{"branches": {"feat/a"}, domain.PathRemember: {"mode"}, domain.PathAsk: {"true"}}}, stale)

	var doc struct {
		FollowUps []domain.FollowUp `json:"follow_ups"`
	}
	require.NoError(t, json.Unmarshal([]byte(got), &doc))
	assert.Equal(t, []domain.FollowUp{{
		Command: "fake",
		Answers: []domain.Answered{{Path: "branches", Values: []string{"feat/a"}}, {Path: domain.PathRemember, Values: []string{"mode"}}, {Path: domain.PathAsk, Values: []string{"true"}}},
		DryRun:  true,
		Code:    domain.CodeReplan,
		Message: domain.ReplanNote,
	}}, doc.FollowUps)
}
