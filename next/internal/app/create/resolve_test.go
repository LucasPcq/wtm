package create_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestANewBranchStartsFromTheBaseBranchAndTheConfiguredEnv(t *testing.T) {
	w := newWorld(t, seed{})

	resolved, err := resolve(t, w, create.Request{Branches: []string{branchName}})

	require.NoError(t, err)
	assert.Equal(t, create.Request{Branches: []string{branchName}, From: "main", EnvFrom: project.EnvMain, Isolation: project.Isolated, FastForward: new(false)}, resolved)
}

func TestABlankBranchIsRequiredAndANameIsTakenAsGiven(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := resolve(t, w, create.Request{Branches: []string{branchName, "  ", "bad..name", " feat/z"}})

	assert.Equal(t, []wtm.FieldError{
		{Path: "branches[1]", Code: "required", Template: "{path} is required"},
		{Path: "branches[2]", Code: "create.branch_name_invalid", Template: "{problem}", Params: wtm.Params{"problem": `"bad..name" is not a valid branch name: it cannot hold .. or end with .`}},
		{Path: "branches[3]", Code: "create.branch_name_invalid", Template: "{problem}", Params: wtm.Params{"problem": `" feat/z" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`}},
	}, fieldsOf(t, err))
}

func TestNoBranchIsRequired(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := resolve(t, w, create.Request{})

	assert.Equal(t, []wtm.FieldError{{Path: "branches", Code: "required", Template: "{path} is required"}}, fieldsOf(t, err))
}

func TestABranchThatAlreadyExistsNeedsItsParentNamed(t *testing.T) {
	w := newWorld(t, seed{})

	_, missing := resolve(t, w, create.Request{Branches: []string{existing}})
	named, err := resolve(t, w, create.Request{Branches: []string{existing}, From: "main"})

	assert.Equal(t, []wtm.FieldError{{
		Path:     "from",
		Code:     "required",
		Template: "{path} is required: a branch that already exists locally needs its parent named (it can't be inferred, and `wtm sync` needs it)",
		Params:   wtm.Params{"because": "create.parent_required"},
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
	assert.Equal(t, []wtm.FieldError{{Path: "from", Code: "not_found", Template: "{path}: {value} not found", Params: wtm.Params{"value": "nope"}}}, fieldsOf(t, unknown))
	assert.NoError(t, onOrigin)
}

func TestAValueNamingAPlaceholderIsWrittenAsGiven(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := resolve(t, w, create.Request{Branches: []string{branchName}, From: "x{path}"})

	assert.Equal(t, map[string]string{"from": "<from>: x{path} not found"}, messages(t, err))
}

func TestABranchIsNeverItsOwnSource(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := resolve(t, w, create.Request{Branches: []string{branchName}, From: branchName})

	assert.Contains(t, fieldsOf(t, err), wtm.FieldError{Path: "from", Code: "self_parent", Template: "{value} cannot be its own parent: pass another value to {path}", Params: wtm.Params{"value": branchName}, With: []string{"branches"}})
}

func TestEachEntryIsRefusedForWhatStandsInItsWay(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml, Linked: []string{"linked"}})
	writeFile(t, w.path("plain")+"/work", "x")

	_, err := resolve(t, w, create.Request{Branches: []string{"bad..name", "linked", "plain", "feat.x", "feat/x", "linked"}, From: "main"})

	assert.Equal(t, map[string]string{
		"branches[0]": `"bad..name" is not a valid branch name: it cannot hold .. or end with .`,
		"branches[1]": "branch linked is checked out at " + linkedPath(w.root, "linked") + " — run `wtm go linked` to jump in",
		"branches[2]": "worktree path already exists: " + w.path("plain"),
		"branches[4]": "branch feat/x would share its name with feat.x (feat-x) — pick another name",
		"branches[5]": "<branches[5]>: linked is given twice",
	}, messages(t, err))
	fields := fieldsOf(t, err)
	assert.Equal(t, wtm.Params{"branch": "linked", "path": linkedPath(w.root, "linked")}, fields[1].Params)
	assert.Equal(t, wtm.Params{"path": w.path("plain")}, fields[2].Params)
	assert.Equal(t, wtm.Params{"branch": branchName, "other": "feat.x", "name": "feat-x"}, fields[3].Params)
	assert.Equal(t, wtm.FieldError{Path: "branches[5]", Code: "distinct", Template: "{path}: {value} is given twice", Params: wtm.Params{"value": "linked"}}, fields[4])
}

func TestTwoBranchesOfOneRequestNeverShareAFolder(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := resolve(t, w, create.Request{Branches: []string{"a/b", "a-b"}})

	assert.Equal(t, map[string]string{"branches[1]": "branch a-b would share its name with a/b (a-b) — pick another name"}, messages(t, err))
	assert.Equal(t, wtm.Params{"branch": "a-b", "other": "a/b", "name": "a-b"}, fieldsOf(t, err)[0].Params)
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

	assert.Equal(t, map[string]string{"branches[0]": "branch feat/x would share its name with feat-x-other (feat-x) — pick another name"}, messages(t, err))
}

func TestABranchAlreadyOpenIsUnchangedUnderIfNotExists(t *testing.T) {
	w := newWorld(t, seed{Linked: []string{"linked"}})

	_, err := resolve(t, w, create.Request{Branches: []string{"linked"}, IfNotExists: true})

	assert.NoError(t, err)
}

func TestAnAnswerOutsideItsChoicesIsRefused(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml})

	_, err := resolve(t, w, create.Request{Branches: []string{branchName}, EnvFrom: "copy", Isolation: "keep", Remember: []string{"env_from", "from"}})

	template := "{path} must be one of {accepted}, not {value}"
	assert.Equal(t, []wtm.FieldError{
		{Path: "env_from", Code: "one_of", Template: template, Params: wtm.Params{"value": "copy"}, Accepted: []string{"example", "main", "parent"}},
		{Path: "isolation", Code: "one_of", Template: template, Params: wtm.Params{"value": "keep"}, Accepted: []string{"isolated", "verbatim"}},
		{Path: "remember[1]", Code: "one_of", Template: template, Params: wtm.Params{"value": "from"}, Accepted: []string{"env_from", "isolation", "fast_forward"}},
	}, fieldsOf(t, err))
	assert.Equal(t, "<remember[1]> must be one of env_from, isolation, fast_forward, not from", messages(t, err)["remember[1]"])
}

func TestAnIsolationGivenIsCheckedEvenWhenNothingIsIsolated(t *testing.T) {
	w := newWorld(t, seed{})

	_, refused := resolve(t, w, create.Request{Branches: []string{branchName}, Isolation: "keep"})
	kept, err := resolve(t, w, create.Request{Branches: []string{branchName}, Isolation: project.Verbatim})

	assert.Equal(t, map[string]string{"isolation": "one_of"}, problems(refused))
	require.NoError(t, err)
	assert.Equal(t, project.Verbatim, kept.Isolation)
}

func TestAnAnswerToASkippedQuestionIsNeverRemembered(t *testing.T) {
	w := newWorld(t, seed{})

	resolved, err := resolve(t, w, create.Request{Branches: []string{branchName}, Remember: []string{"env_from", "isolation", "fast_forward"}})

	require.NoError(t, err)
	assert.Equal(t, []string{"env_from"}, resolved.Remember)
}

func TestTheBranchesAreCheckedFirstThenTheOtherFieldsThenTheRulesAcrossThem(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := resolve(t, w, create.Request{Branches: []string{existing, existing}, From: existing, EnvFrom: "copy"})

	paths := make([]string, 0)
	for _, field := range fieldsOf(t, err) {
		paths = append(paths, field.Path)
	}
	assert.Equal(t, []string{"branches[1]", "env_from", "from"}, paths)
}

func TestDecidingABranchWhoseFolderWasNeverLocatedIsAProgrammingError(t *testing.T) {
	w := newWorld(t, seed{})
	facts := w.facts(t)

	assert.Panics(t, func() { create.Form(facts, create.Request{Branches: []string{branchName}}) })
}
