package extract_test

import (
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheSourcePickerListsTheWorktreesWithChangesAndTheirCount(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	w.change(t, source, "b.txt", "b\n")
	w.change(t, other, "c.txt", "c\n")

	questions := w.questions(t, extract.Request{})

	assert.Equal(t, []domain.Option{{Value: source, Changes: 2}, {Value: other, Changes: 1}}, questions.Source.Options)
	assert.Equal(t, map[string]string{"source": "required", "to": "required"}, fieldCodes(questions.Errors))
}

func TestTheFilesAreTheSourcesChangesAndTheTargetsEveryOtherWorktree(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")

	questions := w.questions(t, extract.Request{Source: source})

	assert.Equal(t, []worktree.Change{{Path: "a.txt", Status: worktree.ChangeUntracked}}, questions.Files.Options)
	assert.Equal(t, []string{"main", other}, valuesOf(questions.To.Options))
	assert.Equal(t, map[string]string{"files": "required", "to": "required"}, fieldCodes(questions.Errors))
}

func TestASourceNoWorktreeHoldsIsNotFound(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := extract.Resolve(w.facts(t, extract.Request{Source: existing}), extract.Request{Source: existing, Files: []string{"a.txt"}, To: split})

	assert.Equal(t, map[string]string{"source": "not_found"}, problems(err))
}

func TestASourceWithNothingToExtractNeedsNothingElse(t *testing.T) {
	w := newWorld(t, seed{})
	req := extract.Request{Source: source}

	resolved, err := extract.Resolve(w.facts(t, req), req)

	require.NoError(t, err)
	assert.Equal(t, source, resolved.Source)
}

func TestAFileThatIsNoChangeIsRefused(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	req := moving("a.txt", "missing.txt")

	_, err := extract.Resolve(w.facts(t, req), req)

	assert.Equal(t, map[string]string{"files": "extract.not_a_change"}, problems(err))
	assert.Equal(t, "files: missing.txt is not an uncommitted change", failureOf(t, err).Fields[0].Message("files"))
}

func TestADirectorySelectsEveryChangeBelowIt(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "apps/api/a.txt", "a\n")
	w.change(t, source, "apps/api/new/b.txt", "b\n")
	w.change(t, source, "apps/web/c.txt", "c\n")

	planned, _ := w.plan(t, moving("apps/api/"))

	assert.Equal(t, []worktree.Change{
		{Path: "apps/api/a.txt", Status: worktree.ChangeUntracked},
		{Path: "apps/api/new/b.txt", Status: worktree.ChangeUntracked},
	}, planned.Files)
}

func TestTheSourceIsNeverItsOwnTarget(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	req := extract.Request{Source: source, Files: []string{"a.txt"}, To: source}

	_, err := extract.Resolve(w.facts(t, req), req)

	assert.Equal(t, map[string]string{"to": "extract.same_worktree"}, problems(err))
}

func TestANewTargetIsRefusedForWhatStandsInItsWay(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	writeFile(t, filepath.Join(w.path(split), "work"), "a folder already there")

	for to, code := range map[string]string{"bad..name": "extract.branch_name_invalid", split: "extract.path_exists"} {
		req := extract.Request{Source: source, Files: []string{"a.txt"}, To: to}
		_, err := extract.Resolve(w.facts(t, req), req)
		assert.Equal(t, map[string]string{"to": code}, problems(err), to)
	}
}

func TestTheParentIsCheckedAsCreateChecksIt(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")

	for from, code := range map[string]string{split: "self_parent", "typo": "not_found"} {
		req := extract.Request{Source: source, Files: []string{"a.txt"}, To: split, From: from}
		_, err := extract.Resolve(w.facts(t, req), req)
		assert.Equal(t, map[string]string{"from": code}, problems(err), from)
	}
}

func TestABranchThatAlreadyExistsLocallyNeedsItsParentNamed(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	req := extract.Request{Source: source, Files: []string{"a.txt"}, To: existing}

	_, err := extract.Resolve(w.facts(t, req), req)

	assert.Equal(t, map[string]string{"from": "required"}, problems(err))
	assert.Equal(t, "extract.parent_required", failureOf(t, err).Fields[0].Params["because"])
}

func TestANewTargetStartsFromTheSourcesRecordedParentElseTheBaseBranch(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	req := extract.Request{Source: source, Files: []string{"a.txt"}, To: split}

	before := w.questions(t, req).From
	w.record(t, source, worktree.Meta{SourceBranch: existing})
	after := w.questions(t, req).From

	assert.Equal(t, domain.Answer[string]{Value: "main", Origin: domain.OriginConfig}, before.Answer)
	assert.Equal(t, domain.Answer[string]{Value: existing, Origin: domain.OriginDefault}, after.Answer)
	assert.Equal(t, existing, after.Pinned)
}

func TestARememberedIsolationIsTheDefaultUnlessAskedAgain(t *testing.T) {
	w := newWorld(t, seed{Config: remembering, Run: jobsToml})
	w.change(t, source, "a.txt", "a\n")
	req := extract.Request{Source: source, Files: []string{"a.txt"}, To: split}

	recalled := w.questions(t, req).Isolation
	req.Ask = true
	asked := w.questions(t, req).Isolation

	assert.Equal(t, domain.Answer[project.Isolation]{Value: project.Verbatim, Origin: domain.OriginRemembered}, recalled.Answer)
	assert.Equal(t, domain.Answer[project.Isolation]{Value: project.Isolated, Origin: domain.OriginDefault}, asked.Answer)
	assert.Equal(t, project.Verbatim, asked.Remembered)
}

func TestTheCreationQuestionsAreSkippedForAnExistingTarget(t *testing.T) {
	w := newWorld(t, seed{Behind: true, Run: jobsToml})
	w.change(t, source, "a.txt", "a\n")

	questions := w.questions(t, moving("a.txt"))

	assert.NotEmpty(t, questions.From.Skip)
	assert.NotEmpty(t, questions.Isolation.Skip)
	assert.NotEmpty(t, questions.FastForward.Skip)
	assert.Empty(t, questions.Isolation.Value)
}

func TestTheFastForwardIsAskedOnlyWhenTheBranchItStartsFromIsBehind(t *testing.T) {
	w := newWorld(t, seed{})
	behind := newWorld(t, seed{Behind: true})
	for _, world := range []world{w, behind} {
		world.change(t, source, "a.txt", "a\n")
	}
	req := extract.Request{Source: source, Files: []string{"a.txt"}, To: split}

	assert.NotEmpty(t, w.questions(t, req).FastForward.Skip)
	assert.Empty(t, behind.questions(t, req).FastForward.Skip)
}

func TestTheConflictModeIsAskedOnlyWhenSomethingConflicts(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "source\n")
	clean, err := w.usecase.Questions(t.Context(), moving("a.txt"))
	require.NoError(t, err)
	w.change(t, other, "a.txt", "target\n")

	conflicting, err := w.usecase.Questions(t.Context(), moving("a.txt"))

	require.NoError(t, err)
	assert.NotEmpty(t, clean.OnConflict.Skip)
	assert.Empty(t, conflicting.OnConflict.Skip)
	assert.Equal(t, []worktree.Conflict{{Path: "a.txt", Kind: worktree.ConflictExists}}, conflicting.OnConflict.Conflicts)
	assert.Equal(t, domain.Answer[extract.OnConflict]{Value: extract.OnConflictAbort, Origin: domain.OriginDefault}, conflicting.OnConflict.Answer)
}

func TestAnAnswerOutsideItsChoicesIsRefused(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	req := moving("a.txt")
	req.OnConflict, req.Isolation, req.Remember = "merge", "loose", []string{"keep"}

	_, err := extract.Resolve(w.facts(t, req), req)

	assert.Equal(t, map[string]string{"on_conflict": "one_of", "isolation": "one_of", "remember[0]": "one_of"}, problems(err))
}

func TestARecordedParentThatNoLongerExistsFallsBackToTheBaseBranch(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")
	w.record(t, source, worktree.Meta{SourceBranch: "feat/base"})
	req := extract.Request{Source: source, Files: []string{"a.txt"}, To: split}

	resolved, err := extract.Resolve(w.facts(t, req), req)

	require.NoError(t, err)
	assert.Equal(t, "main", resolved.From)
}

func TestEachFieldOfTheFormAnswersTheRequestBack(t *testing.T) {
	cases := map[string]extract.Request{
		"to_existing_worktree": {Source: source, Files: []string{"a.txt"}, To: other, Keep: new(true), OnConflict: extract.OnConflictResolve},
		"to_new_branch": {
			Source:     source,
			Files:      []string{"a.txt"},
			To:         split,
			From:       "main",
			Given:      provision.Given{EnvFrom: project.EnvMain, Isolation: project.Verbatim, FastForward: new(true), Remember: []string{"fast_forward", "env_from", "isolation", "fast_forward"}},
			Keep:       new(false),
			OnConflict: extract.OnConflictAbort,
		},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			w := newWorld(t, seed{Behind: true, Run: jobsToml})
			w.change(t, source, "a.txt", "a\n")

			questions := w.questions(t, req)

			require.Empty(t, questions.Errors)
			assert.Equal(t, questions.Request(), answered(t, questions.Form()))
		})
	}
}

func TestTheFormExistsBeforeAnyRead(t *testing.T) {
	form := extract.Questions{}.Form()

	assert.Equal(t, []string{"isolated", "verbatim"}, isolationOptions(form))
	assert.Equal(t, []string{"env_from", "isolation", "fast_forward"}, form.Rememberable())
	assert.Len(t, form.Fields, 10, "a step per field: the target is asked as a worktree, then as a branch")
}

func TestABranchAWorktreeHoldsIsNoNewTarget(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")

	questions := w.questions(t, extract.Request{Source: source})

	require.Len(t, questions.NewTarget(other), 1)
	assert.Equal(t, other+" already has a worktree — pick it as the target worktree", questions.NewTarget(other)[0].Message("Branch name"))
	assert.Empty(t, questions.NewTarget(split))
}

func TestOnlyABranchNoWorktreeButTheSourcesHoldsCreatesATarget(t *testing.T) {
	w := newWorld(t, seed{})
	w.change(t, source, "a.txt", "a\n")

	questions := w.questions(t, extract.Request{Source: source})

	assert.True(t, questions.CreatesTarget(split))
	assert.False(t, questions.CreatesTarget(other))
	assert.False(t, questions.CreatesTarget(source))
	assert.False(t, questions.CreatesTarget(""))
}

func TestAnAnswerTheRequestCannotTakeIsRefusedAndChangesNothing(t *testing.T) {
	cases := map[string]struct {
		path   string
		values []string
		code   string
	}{
		"unknown_field":            {"branches", []string{"a"}, domain.CodeUnknown},
		"unknown_field_two_values": {"typo", []string{"a", "b"}, domain.CodeUnknown},
		"two_values_for_one":       {"to", []string{other, split}, domain.CodeOneValue},
		"confirmation_misspelt":    {"keep", []string{"copy"}, domain.CodeOneOf},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			req := extract.Request{To: other, Keep: new(true)}

			got, err := req.Answer(c.path, c.values)

			assert.Equal(t, map[string]string{c.path: c.code}, problems(err))
			assert.Equal(t, req, got)
		})
	}
}

func TestAConfirmationTakesTrueAndFalse(t *testing.T) {
	form := extract.Questions{}.Form()
	for _, field := range form.Fields {
		confirm, isConfirm := field.(domain.ConfirmField)
		if !isConfirm {
			continue
		}

		for want, value := range map[bool]string{true: "true", false: "false"} {
			req, err := extract.Request{}.Answer(confirm.Path, []string{value})
			require.NoError(t, err, confirm.Path)
			assert.Equal(t, want, answeredTrue(t, req, confirm.Path), confirm.Path)
		}
	}
}

func TestEachAnswerReadsBackAsAnswerTakesIt(t *testing.T) {
	req := extract.Request{Source: "feat/login", Files: []string{"a.txt"}, To: "feat/split", From: "main", Keep: new(false), OnConflict: extract.OnConflictResolve, Given: provision.Given{EnvFrom: project.EnvParent, FastForward: new(true), Isolation: project.Verbatim, Remember: []string{"isolation"}, Ask: true}}
	paths := []string{domain.PathRemember, domain.PathAsk}
	for _, field := range (extract.Questions{}).Form().Fields {
		paths = append(paths, field.Spec().Path)
	}

	var answers []domain.Answered
	for _, path := range paths {
		answers = append(answers, domain.Answered{Path: path, Values: req.Values(path)})
	}
	got, err := domain.Fold(extract.Request{}, answers)

	require.NoError(t, err)
	assert.Equal(t, req, got)
}

func TestARememberedSourceUpdateIsShownEvenWhenAskedAgain(t *testing.T) {
	w := newWorld(t, seed{Config: remembering, Behind: true})
	w.change(t, source, "a.txt", "a\n")

	asked := w.questions(t, extract.Request{Source: source, Files: []string{"a.txt"}, To: split, Given: provision.Given{Ask: true}}).FastForward

	assert.Equal(t, new(true), asked.Remembered)
	assert.Equal(t, domain.OriginDefault, asked.Origin)
}
