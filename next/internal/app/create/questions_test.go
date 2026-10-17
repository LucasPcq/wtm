package create_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/stretchr/testify/assert"
)

const remembering = projectToml + "\n[wizard.remembered]\nenv_strategy = \"example\"\nisolation = \"verbatim\"\nsource_update = \"ff\"\n"

func TestEachDefaultSaysWhereItComesFrom(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml, Behind: true})

	questions := w.questions(t, create.Request{Branches: []string{branchName}})

	assert.Equal(t, create.Answer[string]{Value: "main", Origin: create.OriginConfig}, questions.From.Answer)
	assert.Equal(t, create.Answer[project.EnvStrategy]{Value: project.EnvMain, Origin: create.OriginConfig}, questions.EnvFrom.Answer)
	assert.Equal(t, create.Answer[run.Isolation]{Value: run.Isolated, Origin: create.OriginDefault}, questions.Isolation.Answer)
	assert.Equal(t, create.Answer[bool]{Value: false, Origin: create.OriginDefault}, questions.FastForward.Answer)
	assert.Empty(t, questions.Errors)
}

func TestARememberedAnswerIsTheDefaultUnlessAskedAgain(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml, Behind: true, Config: remembering})

	remembered := w.questions(t, create.Request{Branches: []string{branchName}})
	asked := w.questions(t, create.Request{Branches: []string{branchName}, Ask: true})

	assert.Equal(t, create.Answer[project.EnvStrategy]{Value: project.EnvExample, Origin: create.OriginRemembered}, remembered.EnvFrom.Answer)
	assert.Equal(t, create.Answer[run.Isolation]{Value: run.Verbatim, Origin: create.OriginRemembered}, remembered.Isolation.Answer)
	assert.Equal(t, create.Answer[bool]{Value: true, Origin: create.OriginRemembered}, remembered.FastForward.Answer)
	assert.Equal(t, create.Answer[project.EnvStrategy]{Value: project.EnvMain, Origin: create.OriginConfig}, asked.EnvFrom.Answer)
	assert.Equal(t, create.Answer[bool]{Value: false, Origin: create.OriginDefault}, asked.FastForward.Answer)
}

func TestAnAnswerGivenWinsOverARememberedOne(t *testing.T) {
	w := newWorld(t, seed{Run: jobsToml, Behind: true, Config: remembering})

	questions := w.questions(t, create.Request{Branches: []string{branchName}, EnvFrom: project.EnvParent, Isolation: run.Isolated, FastForward: new(false)})

	assert.Equal(t, create.Answer[project.EnvStrategy]{Value: project.EnvParent, Origin: create.OriginRequest}, questions.EnvFrom.Answer)
	assert.Equal(t, create.Answer[run.Isolation]{Value: run.Isolated, Origin: create.OriginRequest}, questions.Isolation.Answer)
	assert.Equal(t, create.Answer[bool]{Value: false, Origin: create.OriginRequest}, questions.FastForward.Answer)
}

func TestTheRunIsolationIsTheDefaultBeforeIsolated(t *testing.T) {
	w := newWorld(t, seed{Run: "isolation = \"verbatim\"\n" + jobsToml})

	questions := w.questions(t, create.Request{Branches: []string{branchName}})

	assert.Equal(t, create.Answer[run.Isolation]{Value: run.Verbatim, Origin: create.OriginConfig}, questions.Isolation.Answer)
}

func TestIsolationIsAskedOnlyWhenTheRunHasSomethingToIsolate(t *testing.T) {
	bare := newWorld(t, seed{})
	isolating := newWorld(t, seed{Run: jobsToml})

	skipped := bare.questions(t, create.Request{Branches: []string{branchName}})
	asked := isolating.questions(t, create.Request{Branches: []string{branchName}})

	assert.Equal(t, "run.toml declares nothing a worktree isolates", skipped.Isolation.Skip)
	assert.Equal(t, create.Answer[run.Isolation]{}, skipped.Isolation.Answer)
	assert.Empty(t, asked.Isolation.Skip)
	assert.Equal(t, []run.Isolation{run.Isolated, run.Verbatim}, asked.Isolation.Options)
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

	assert.Equal(t, []create.Option{
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

	assert.Equal(t, create.Option{Value: "main", Badge: "1 behind"}, behind.questions(t, create.Request{Branches: []string{branchName}}).From.Options[1])
}

func TestTheQuestionsCarryTheProblemsOfTheRequest(t *testing.T) {
	w := newWorld(t, seed{})

	questions := w.questions(t, create.Request{Branches: []string{"bad..name"}})

	assert.Equal(t, map[string]string{"branches[0]": "create.branch_name_invalid"}, problemsOf(questions))
}
