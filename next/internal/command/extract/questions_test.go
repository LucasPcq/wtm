package extract_test

import (
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

	assert.Equal(t, []domain.Option{{Value: source, Badge: "2 changes"}, {Value: other, Badge: "1 change"}}, questions.Source.Options)
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
