package create_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEachDefaultSaysWhereItComesFrom(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml, Behind: true})

	questions := w.questions(t, create.Request{Branches: []string{branchName}})

	assert.Equal(t, domain.Answer[string]{Value: "main", Origin: domain.OriginConfig}, questions.From.Answer)
	assert.Equal(t, domain.Answer[project.EnvStrategy]{Value: project.EnvMain, Origin: domain.OriginConfig}, questions.EnvFrom.Answer)
	assert.Equal(t, domain.Answer[project.Isolation]{Value: project.Isolated, Origin: domain.OriginDefault}, questions.Isolation.Answer)
	assert.Equal(t, domain.Answer[bool]{Value: false, Origin: domain.OriginDefault}, questions.FastForward.Answer)
	assert.Empty(t, questions.Errors)
}

func TestARememberedAnswerIsTheDefaultUnlessAskedAgain(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml, Behind: true, Config: remembering})

	remembered := w.questions(t, create.Request{Branches: []string{branchName}})
	asked := w.questions(t, create.Request{Branches: []string{branchName}, Ask: true})

	assert.Equal(t, domain.Answer[project.EnvStrategy]{Value: project.EnvExample, Origin: domain.OriginRemembered}, remembered.EnvFrom.Answer)
	assert.Equal(t, domain.Answer[project.Isolation]{Value: project.Verbatim, Origin: domain.OriginRemembered}, remembered.Isolation.Answer)
	assert.Equal(t, domain.Answer[bool]{Value: true, Origin: domain.OriginRemembered}, remembered.FastForward.Answer)
	assert.Equal(t, domain.Answer[project.EnvStrategy]{Value: project.EnvMain, Origin: domain.OriginConfig}, asked.EnvFrom.Answer)
	assert.Equal(t, domain.Answer[bool]{Value: false, Origin: domain.OriginDefault}, asked.FastForward.Answer)
	assert.Equal(t, project.EnvExample, asked.EnvFrom.Remembered)
	assert.Equal(t, project.Verbatim, asked.Isolation.Remembered)
	assert.Equal(t, project.EnvExample, remembered.EnvFrom.Remembered)
}

func TestARememberedConfigDefaultFollowsTheConfiguredEnvStrategy(t *testing.T) {
	w := newWorld(t, seed{Config: projectToml + "\n[wizard.remembered]\nenv_strategy = \"config\"\n"})

	questions := w.questions(t, create.Request{Branches: []string{branchName}})

	assert.Equal(t, domain.Answer[project.EnvStrategy]{Value: project.EnvMain, Origin: domain.OriginRemembered}, questions.EnvFrom.Answer)
	assert.Equal(t, project.EnvMain, questions.EnvFrom.Remembered)
	assert.Empty(t, questions.Errors)
}

func TestIfNotExistsIsSetByItsFlagOnly(t *testing.T) {
	w := newWorld(t, seed{})

	questions := w.questions(t, create.Request{Branches: []string{branchName}, IfNotExists: true})

	assert.Equal(t, domain.Confirm{Answer: domain.Answer[bool]{Value: true, Origin: domain.OriginRequest}, Skip: "set by its flag only"}, questions.IfNotExists)
}

func TestAnAnswerGivenWinsOverARememberedOne(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml, Behind: true, Config: remembering})

	questions := w.questions(t, create.Request{Branches: []string{branchName}, EnvFrom: project.EnvParent, Isolation: project.Isolated, FastForward: new(false)})

	assert.Equal(t, domain.Answer[project.EnvStrategy]{Value: project.EnvParent, Origin: domain.OriginRequest}, questions.EnvFrom.Answer)
	assert.Equal(t, domain.Answer[project.Isolation]{Value: project.Isolated, Origin: domain.OriginRequest}, questions.Isolation.Answer)
	assert.Equal(t, domain.Answer[bool]{Value: false, Origin: domain.OriginRequest}, questions.FastForward.Answer)
}

func TestTheRunIsolationIsTheDefaultBeforeIsolated(t *testing.T) {
	w := newWorld(t, seed{Run: "isolation = \"verbatim\"\n" + jobsToml})

	questions := w.questions(t, create.Request{Branches: []string{branchName}})

	assert.Equal(t, domain.Answer[project.Isolation]{Value: project.Verbatim, Origin: domain.OriginConfig}, questions.Isolation.Answer)
}

func TestIsolationIsAskedOnlyWhenTheRunHasSomethingToIsolate(t *testing.T) {
	bare := newWorld(t, seed{})
	isolating := newWorld(t, seed{Run: jobsToml})

	skipped := bare.questions(t, create.Request{Branches: []string{branchName}})
	asked := isolating.questions(t, create.Request{Branches: []string{branchName}})

	assert.Equal(t, "run.toml declares nothing a worktree isolates", skipped.Isolation.Skip)
	assert.Equal(t, domain.Answer[project.Isolation]{Value: project.Isolated, Origin: domain.OriginDefault}, skipped.Isolation.Answer)
	assert.Empty(t, asked.Isolation.Skip)
	assert.Equal(t, []project.Isolation{project.Isolated, project.Verbatim}, asked.Isolation.Options)
}

func TestTheFastForwardIsAskedOnlyWhenTheBranchItStartsFromIsBehind(t *testing.T) {
	upToDate := newWorld(t, seed{})
	behind := newWorld(t, seed{Behind: true})

	assert.Equal(t, "nothing to fast-forward: no branch it starts from is behind origin", upToDate.questions(t, create.Request{Branches: []string{branchName}}).FastForward.Skip)
	assert.Empty(t, behind.questions(t, create.Request{Branches: []string{branchName}}).FastForward.Skip)
}

func TestTheEnvStrategiesAreTheChoices(t *testing.T) {
	w := newWorld(t, seed{})

	assert.Equal(t, []project.EnvStrategy{project.EnvExample, project.EnvMain, project.EnvParent}, w.questions(t, create.Request{}).EnvFrom.Options)
}

func TestTheBranchOptionsSayWhichAlreadyExist(t *testing.T) {
	w := newWorld(t, seed{Linked: []string{"linked"}})

	questions := w.questions(t, create.Request{})

	assert.Equal(t, []domain.Option{
		{Value: existing, Badge: "existing"},
		{Value: "linked", Badge: "worktree exists"},
		{Value: "main", Badge: "worktree exists"},
		{Value: remote, Badge: "on origin"},
	}, questions.Branches.Options)
}

func TestTheSourcePickerListsEveryBranchButThoseBeingCreated(t *testing.T) {
	w := newWorld(t, seed{Diverged: true})

	questions := w.questions(t, create.Request{Branches: []string{existing}})

	assert.Equal(t, "main", questions.From.Pinned)
	assert.Equal(t, []string{"main", "origin/" + remote}, values(questions.From.Options))
	assert.Equal(t, "1 ahead, 1 behind", questions.From.Options[0].Badge)
}

func TestTheSourceBadgesSayWhereABranchStandsAgainstOrigin(t *testing.T) {
	behind := newWorld(t, seed{Behind: true})

	assert.Equal(t, domain.Option{Value: "main", Badge: "1 behind"}, behind.questions(t, create.Request{Branches: []string{branchName}}).From.Options[1])
}

func TestTheQuestionsCarryTheProblemsOfTheRequest(t *testing.T) {
	w := newWorld(t, seed{})

	questions := w.questions(t, create.Request{Branches: []string{"bad..name"}})

	assert.Equal(t, map[string]string{"branches[0]": "create.branch_name_invalid"}, problemsOf(questions))
}

func TestANewBranchStartsFromTheBaseBranchAndTheConfiguredEnv(t *testing.T) {
	w := newWorld(t, seed{})

	resolved, err := resolve(t, w, create.Request{Branches: []string{branchName}})

	require.NoError(t, err)
	assert.Equal(t, create.Request{Branches: []string{branchName}, From: "main", EnvFrom: project.EnvMain, Isolation: project.Isolated, FastForward: new(false)}, resolved)
}

func TestABlankBranchIsRequiredAndANameIsTakenAsGiven(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := resolve(t, w, create.Request{Branches: []string{branchName, "  ", "bad..name", " feat/z"}})

	assert.Equal(t, []domain.FieldError{
		{Path: "branches[1]", Code: "required", Template: "{path} is required"},
		{Path: "branches[2]", Code: "create.branch_name_invalid", Template: "{problem}", Params: domain.Params{"problem": `"bad..name" is not a valid branch name: it cannot hold .. or end with .`}},
		{Path: "branches[3]", Code: "create.branch_name_invalid", Template: "{problem}", Params: domain.Params{"problem": `" feat/z" is not a valid branch name: it cannot hold a space, a control character or any of ~ ^ : ? * [ \`}},
	}, fieldsOf(t, err))
}

func TestNoBranchIsRequired(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := resolve(t, w, create.Request{})

	assert.Equal(t, []domain.FieldError{{Path: "branches", Code: "required", Template: "{path} is required"}}, fieldsOf(t, err))
}

func TestABranchThatAlreadyExistsNeedsItsParentNamed(t *testing.T) {
	w := newWorld(t, seed{})

	_, missing := resolve(t, w, create.Request{Branches: []string{existing}})
	named, err := resolve(t, w, create.Request{Branches: []string{existing}, From: "main"})

	assert.Equal(t, []domain.FieldError{{
		Path:     "from",
		Code:     "required",
		Template: "{path} is required: a branch that already exists locally needs its parent named (it can't be inferred, and `wtm sync` needs it)",
		Params:   domain.Params{"because": "create.parent_required"},
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
	assert.Equal(t, []domain.FieldError{{Path: "from", Code: "not_found", Template: "{path}: {value} not found", Params: domain.Params{"value": "nope"}}}, fieldsOf(t, unknown))
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

	assert.Contains(t, fieldsOf(t, err), domain.FieldError{Path: "from", Code: "self_parent", Template: "{value} cannot be its own parent: pass another value to {path}", Params: domain.Params{"value": branchName}, With: []string{"branches"}})
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
	assert.Equal(t, domain.Params{"branch": "linked", "path": linkedPath(w.root, "linked")}, fields[1].Params)
	assert.Equal(t, domain.Params{"path": w.path("plain")}, fields[2].Params)
	assert.Equal(t, domain.Params{"branch": branchName, "other": "feat.x", "name": "feat-x"}, fields[3].Params)
	assert.Equal(t, domain.FieldError{Path: "branches[5]", Code: "distinct", Template: "{path}: {value} is given twice", Params: domain.Params{"value": "linked"}}, fields[4])
}

func TestTwoBranchesOfOneRequestNeverShareAFolder(t *testing.T) {
	w := newWorld(t, seed{})

	_, err := resolve(t, w, create.Request{Branches: []string{"a/b", "a-b"}})

	assert.Equal(t, map[string]string{"branches[1]": "branch a-b would share its name with a/b (a-b) — pick another name"}, messages(t, err))
	assert.Equal(t, domain.Params{"branch": "a-b", "other": "a/b", "name": "a-b"}, fieldsOf(t, err)[0].Params)
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
	assert.Equal(t, []domain.FieldError{
		{Path: "env_from", Code: "one_of", Template: template, Params: domain.Params{"value": "copy"}, Accepted: []string{"example", "main", "parent"}},
		{Path: "isolation", Code: "one_of", Template: template, Params: domain.Params{"value": "keep"}, Accepted: []string{"isolated", "verbatim"}},
		{Path: "remember[1]", Code: "one_of", Template: template, Params: domain.Params{"value": "from"}, Accepted: []string{"env_from", "isolation", "fast_forward"}},
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

	assert.Panics(t, func() { create.NewQuestions(facts, create.Request{Branches: []string{branchName}}) })
}

func TestEachFieldOfTheFormAnswersTheRequestBack(t *testing.T) {
	cases := map[string]create.Request{
		"defaults": {Branches: []string{branchName}},
		"every field given": {
			Branches:    []string{branchName, "feat/two"},
			From:        "main",
			EnvFrom:     project.EnvMain,
			Isolation:   project.Verbatim,
			FastForward: new(true),
			IfNotExists: true,
			Remember:    []string{"isolation", "env_from", "isolation", "fast_forward"},
		},
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			questions := newWorld(t, seed{Run: jobsToml, Behind: true}).questions(t, req)

			assert.Equal(t, questions.Request(), answered(t, questions.Form()))
		})
	}
}

func TestTheFormExistsBeforeAnyRead(t *testing.T) {
	form := create.Questions{}.Form()

	flags, usages := form.Flags()
	assert.Equal(t, map[string]string{"branches": "<branches>", "from": "--from", "env_from": "--env-from", "isolation": "--isolation", "fast_forward": "--ff", "if_not_exists": "--if-not-exists", "remember": "--remember"}, flags)
	assert.Equal(t, "The answers to always use in this repository: env_from, isolation, fast_forward (repeatable)", usages["remember"])
	assert.Equal(t, []string{"env_from", "isolation", "fast_forward"}, form.Rememberable())
}

func TestTheFormCarriesTheRequestsProblems(t *testing.T) {
	questions := newWorld(t, seed{}).questions(t, create.Request{Branches: []string{branchName, branchName}, EnvFrom: "copy", Remember: []string{"from"}})

	form := questions.Form()

	assert.Equal(t, questions.Errors, form.Problems)
	assert.Equal(t, []string{"branches[1]"}, paths(domain.ProblemsOf(form.Problems, "branches")))
	assert.Equal(t, []string{"remember[0]"}, paths(domain.ProblemsOf(form.Problems, domain.PathRemember)))
}

func TestAnAnswerTheRequestCannotTakeIsRefusedAndChangesNothing(t *testing.T) {
	cases := map[string]struct {
		path   string
		values []string
		code   string
	}{
		"an unknown field":             {"files", []string{"a"}, domain.CodeUnknown},
		"an unknown field, two values": {"typo", []string{"a", "b"}, domain.CodeUnknown},
		"two values for one":           {"from", []string{"main", "dev"}, domain.CodeOneValue},
		"a confirmation unspelt":       {"fast_forward", []string{"yes"}, domain.CodeOneOf},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			req := create.Request{From: "feature", FastForward: new(true)}

			got, err := req.Answer(c.path, c.values)

			require.Len(t, fieldsOf(t, err), 1)
			assert.Equal(t, c.code, fieldsOf(t, err)[0].Code)
			assert.Equal(t, req, got)
		})
	}
}
