package create_test

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func resolve(t *testing.T, w world, req create.Request) (create.Request, error) {
	t.Helper()
	return create.Resolve(w.facts(t, req.Branches...), req)
}

func fieldsOf(t *testing.T, err error) []wtm.FieldError {
	t.Helper()
	invalid, ok := errors.AsType[*wtm.Error](err)
	require.True(t, ok, err)
	assert.Equal(t, wtm.KindInvalid, invalid.Kind)
	assert.Equal(t, "request.invalid", invalid.Code)
	return invalid.Fields
}

func TestANewBranchStartsFromTheBaseBranchAndTheConfiguredEnv(t *testing.T) {
	w := newWorld(t, seed{})

	resolved, err := resolve(t, w, create.Request{Branches: []string{branchName}})

	require.NoError(t, err)
	assert.Equal(t, create.Request{Branches: []string{branchName}, From: "main", EnvFrom: project.EnvMain, FastForward: new(false)}, resolved)
}

func TestEachBranchIsTrimmedAndABlankOneIsRequired(t *testing.T) {
	w := newWorld(t, seed{})

	trimmed, err := resolve(t, w, create.Request{Branches: []string{"  " + branchName + " "}})
	_, blank := resolve(t, w, create.Request{Branches: []string{branchName, "  ", "bad..name"}})

	require.NoError(t, err)
	assert.Equal(t, []string{branchName}, trimmed.Branches)
	assert.Equal(t, []wtm.FieldError{
		{Path: "branches[1]", Code: "required", Message: "{path} is required"},
		{Path: "branches[2]", Code: "create.branch_name_invalid", Message: `"bad..name" is not a valid branch name: it cannot hold .. or end with .`, Params: wtm.Params{"problem": `"bad..name" is not a valid branch name: it cannot hold .. or end with .`}},
	}, fieldsOf(t, blank))
}

func TestNoBranchIsRequired(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := resolve(t, w, create.Request{})

	assert.Equal(t, []wtm.FieldError{{Path: "branches", Code: "required", Message: "{path} is required"}}, fieldsOf(t, err))
}

func TestABranchThatAlreadyExistsNeedsItsParentNamed(t *testing.T) {
	w := newWorld(t, seed{})

	_, missing := resolve(t, w, create.Request{Branches: []string{existing}})
	named, err := resolve(t, w, create.Request{Branches: []string{existing}, From: "main"})

	assert.Equal(t, []wtm.FieldError{{
		Path:    "from",
		Code:    "required",
		Message: "{path} is required: a branch that already exists locally needs its parent named (it can't be inferred, and `wtm sync` needs it)",
		Params:  wtm.Params{"because": "create.parent_required"},
	}}, fieldsOf(t, missing))
	require.NoError(t, err)
	assert.Equal(t, "main", named.From)
}

func TestASourceThatIsNoBranchIsNotFound(t *testing.T) {
	ghost := newWorld(t, seed{Config: "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"ghost\"\n\n[env]\nstrategy = \"main\"\n"})
	w := newWorld(t, seed{})

	_, defaulted := resolve(t, ghost, create.Request{Branches: []string{branchName}})
	_, unknown := resolve(t, w, create.Request{Branches: []string{branchName}, From: "nope"})
	_, onOrigin := resolve(t, w, create.Request{Branches: []string{branchName}, From: "origin/" + remote})

	assert.Equal(t, map[string]string{"from": "not_found"}, problems(defaulted))
	assert.Equal(t, []wtm.FieldError{{Path: "from", Code: "not_found", Message: "{path}: nope not found", Params: wtm.Params{"value": "nope"}}}, fieldsOf(t, unknown))
	assert.NoError(t, onOrigin)
}

func TestABranchIsNeverItsOwnSource(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := resolve(t, w, create.Request{Branches: []string{branchName}, From: branchName})

	assert.Contains(t, fieldsOf(t, err), wtm.FieldError{Path: "from", Code: "self_parent", Message: "feat/x cannot be its own parent: pass another value to {path}", Params: wtm.Params{"value": branchName}})
}

func TestEachEntryIsRefusedForWhatStandsInItsWay(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml, Linked: []string{"linked"}})
	writeFile(t, w.path("plain")+"/work", "x")

	_, err := resolve(t, w, create.Request{Branches: []string{"bad..name", "linked", "plain", "feat.x", "feat/x", "linked"}, From: "main"})

	assert.Equal(t, map[string]string{
		"branches[0]": "create.branch_name_invalid",
		"branches[1]": "create.checked_out_elsewhere",
		"branches[2]": "create.path_exists",
		"branches[4]": "create.name_clash",
		"branches[5]": "distinct",
	}, problems(err))
	fields := fieldsOf(t, err)
	assert.Equal(t, wtm.FieldError{Path: "branches[1]", Code: "create.checked_out_elsewhere", Message: "branch linked is checked out at " + linkedPath(w.root, "linked") + " — run `wtm go linked` to jump in", Params: wtm.Params{"branch": "linked", "path": linkedPath(w.root, "linked")}}, fields[1])
	assert.Equal(t, wtm.FieldError{Path: "branches[2]", Code: "create.path_exists", Message: "worktree path already exists: " + w.path("plain"), Params: wtm.Params{"path": w.path("plain")}}, fields[2])
	assert.Equal(t, wtm.FieldError{Path: "branches[4]", Code: "create.name_clash", Message: "branch feat/x would share its name with feat.x (feat-x) — pick another name", Params: wtm.Params{"branch": branchName, "other": "feat.x", "name": "feat-x"}}, fields[3])
	assert.Equal(t, wtm.FieldError{Path: "branches[5]", Code: "distinct", Message: "{path}: linked is given twice", Params: wtm.Params{"value": "linked"}}, fields[4])
}

func TestANameClashWithALiveWorktreeOnlyMattersWhenTheRunNamesWorktrees(t *testing.T) {
	plain := newWorld(t, seed{Linked: []string{"feat.x"}})
	named := newWorld(t, seed{Run: jobsToml, Linked: []string{"feat.x"}})

	_, ignored := resolve(t, plain, create.Request{Branches: []string{branchName}})
	_, refused := resolve(t, named, create.Request{Branches: []string{branchName}})

	assert.NoError(t, ignored)
	assert.Equal(t, map[string]string{"branches[0]": "create.name_clash"}, problems(refused))
}

func TestAFolderHoldingAnotherBranchIsANameClash(t *testing.T) {
	occupied := newWorld(t, seed{})
	gitAddWorktree(t, occupied, "feat-x-other", occupied.path(branchName))

	_, err := resolve(t, occupied, create.Request{Branches: []string{branchName}})

	assert.Equal(t, []wtm.FieldError{{Path: "branches[0]", Code: "create.name_clash", Message: "branch feat/x would share its name with feat-x-other (feat-x) — pick another name", Params: wtm.Params{"branch": branchName, "other": "feat-x-other", "name": "feat-x"}}}, fieldsOf(t, err))
}

func TestABranchAlreadyOpenIsUnchangedUnderIfNotExists(t *testing.T) {
	w := newWorld(t, seed{Linked: []string{"linked"}})

	_, err := resolve(t, w, create.Request{Branches: []string{"linked"}, IfNotExists: true})

	assert.NoError(t, err)
}

func TestAnAnswerOutsideItsChoicesIsRefused(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml})

	_, err := resolve(t, w, create.Request{Branches: []string{branchName}, EnvFrom: "copy", Isolation: "keep", Remember: []string{"env_from", "from"}})

	assert.Equal(t, []wtm.FieldError{
		{Path: "env_from", Code: "one_of", Message: "{path} must be one of example, main, parent, not copy", Params: wtm.Params{"value": "copy"}, Accepted: []string{"example", "main", "parent"}},
		{Path: "isolation", Code: "one_of", Message: "{path} must be one of isolated, verbatim, not keep", Params: wtm.Params{"value": "keep"}, Accepted: []string{"isolated", "verbatim"}},
		{Path: "remember[1]", Code: "one_of", Message: "{path} must be one of env_from, isolation, fast_forward, not from", Params: wtm.Params{"value": "from"}, Accepted: []string{"env_from", "isolation", "fast_forward"}},
	}, fieldsOf(t, err))
}

func TestAnIsolationGivenIsCheckedEvenWhenNothingIsIsolated(t *testing.T) {
	w := newWorld(t, seed{})

	_, refused := resolve(t, w, create.Request{Branches: []string{branchName}, Isolation: "keep"})
	kept, err := resolve(t, w, create.Request{Branches: []string{branchName}, Isolation: run.Verbatim})

	assert.Equal(t, map[string]string{"isolation": "one_of"}, problems(refused))
	require.NoError(t, err)
	assert.Equal(t, run.Verbatim, kept.Isolation)
}

func TestTheFieldsAreCheckedInTheirOrderThenTheRulesAcrossThem(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := resolve(t, w, create.Request{Branches: []string{existing, existing}, From: existing, EnvFrom: "copy"})

	paths := make([]string, 0)
	for _, field := range fieldsOf(t, err) {
		paths = append(paths, field.Path)
	}
	assert.Equal(t, []string{"env_from", "branches[1]", "from"}, paths)
}
