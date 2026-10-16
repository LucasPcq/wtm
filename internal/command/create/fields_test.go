package create_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/command/create"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/kerneltest"
)

func TestEachFieldDeclaresWhatItReads(t *testing.T) {
	w := newWorld(t, seed{Run: jobs, Behind: true})

	kerneltest.CheckDependsOn(t, kerneltest.DependsOnParams[create.Request, create.Facts]{
		Fields:  create.Command(w.ports).Fields,
		Request: create.Request{Branches: []string{branch, existing}, From: "main", EnvFrom: domain.EnvStrategyMain, Isolation: domain.IsolationIsolated, FastForward: true, IfNotExists: true},
		Facts:   w.facts(t),
	})
}

func TestANewBranchStartsFromTheBaseBranchAndTheConfiguredEnv(t *testing.T) {
	w := newWorld(t, seed{})

	form := w.form(t, create.Request{Branches: []string{branch}})

	require.True(t, form.Complete, form.Errors)
	assert.Equal(t, kernel.FieldState{Path: create.PathFrom, Status: kernel.FieldDefaulted, Origin: kernel.OriginConfig, Value: kernel.Text("main")}, stateOf(form, create.PathFrom))
	assert.Equal(t, kernel.FieldState{Path: create.PathEnvFrom, Status: kernel.FieldDefaulted, Origin: kernel.OriginConfig, Value: kernel.Text(domain.EnvStrategyMain)}, stateOf(form, create.PathEnvFrom))
}

func TestABranchThatAlreadyExistsNeedsItsParentNamed(t *testing.T) {
	w := newWorld(t, seed{})

	missing := w.form(t, create.Request{Branches: []string{existing}})
	named := w.form(t, create.Request{Branches: []string{existing}, From: "main"})

	assert.Equal(t, map[string]kernel.Code{create.PathFrom: kernel.CodeRequired}, problems(missing))
	assert.Equal(t, string(create.CodeParentRequired), missing.Errors[0].Params[kernel.ParamBecause])
	assert.True(t, named.Complete, named.Errors)
}

func TestABaseBranchThatDoesNotExistIsNotFound(t *testing.T) {
	w := newWorld(t, seed{Config: "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"ghost\"\n\n[env]\nstrategy = \"main\"\n"})

	form := w.form(t, create.Request{Branches: []string{branch}})

	assert.Equal(t, map[string]kernel.Code{create.PathFrom: kernel.CodeNotFound}, problems(form))
}

func TestASourceThatIsNoBranchIsNotFound(t *testing.T) {
	w := newWorld(t, seed{})

	unknown := w.form(t, create.Request{Branches: []string{branch}, From: "nope"})
	onOrigin := w.form(t, create.Request{Branches: []string{branch}, From: "origin/" + remote})

	assert.Equal(t, map[string]kernel.Code{create.PathFrom: kernel.CodeNotFound}, problems(unknown))
	assert.True(t, onOrigin.Complete, onOrigin.Errors)
}

func TestABranchIsNeverItsOwnSource(t *testing.T) {
	w := newWorld(t, seed{})

	form := w.form(t, create.Request{Branches: []string{branch}, From: branch})

	assert.Contains(t, problems(form), create.PathFrom)
	assert.Equal(t, kernel.CodeSelfParent, problems(form)[create.PathFrom])
}

func TestEachEntryIsRefusedForWhatStandsInItsWay(t *testing.T) {
	w := newWorld(t, seed{Run: jobs, Linked: []string{"linked"}})
	writeFile(t, filepath.Join(w.trees, "plain", "work"), "x")

	form := w.form(t, create.Request{Branches: []string{"bad..name", "linked", "plain", "feat.x", "feat/x", "linked"}})

	assert.Equal(t, map[string]kernel.Code{
		"branches[0]": create.CodeBranchNameInvalid,
		"branches[1]": create.CodeCheckedOutElsewhere,
		"branches[2]": create.CodePathExists,
		"branches[4]": create.CodeNameClash,
		"branches[5]": kernel.CodeDistinct,
	}, problems(form))
}

func TestABranchAlreadyOpenIsUnchangedUnderIfNotExists(t *testing.T) {
	w := newWorld(t, seed{Linked: []string{"linked"}})

	form := w.form(t, create.Request{Branches: []string{"linked"}, IfNotExists: true})

	assert.True(t, form.Complete, form.Errors)
}

func TestIsolationIsAskedOnlyWhenTheRunHasSomethingToIsolate(t *testing.T) {
	bare := newWorld(t, seed{})
	run := newWorld(t, seed{Run: jobs})

	assert.Equal(t, kernel.FieldSkipped, stateOf(bare.form(t, create.Request{Branches: []string{branch}}), create.PathIsolation).Status)
	assert.Equal(t, kernel.FieldState{Path: create.PathIsolation, Status: kernel.FieldDefaulted, Origin: kernel.OriginDefault, Value: kernel.Text(domain.IsolationIsolated)},
		stateOf(run.form(t, create.Request{Branches: []string{branch}}), create.PathIsolation))
}

func TestARememberedAnswerIsTheDefault(t *testing.T) {
	w := newWorld(t, seed{Run: jobs, Behind: true, Config: project + "\n[wizard.remembered]\nenv_strategy = \"example\"\nisolation = \"verbatim\"\nsource_update = \"ff\"\n"})

	form := w.form(t, create.Request{Branches: []string{branch}})

	assert.Equal(t, kernel.FieldState{Path: create.PathEnvFrom, Status: kernel.FieldDefaulted, Origin: kernel.OriginRemembered, Value: kernel.Text(domain.EnvStrategyExample)}, stateOf(form, create.PathEnvFrom))
	assert.Equal(t, kernel.FieldState{Path: create.PathIsolation, Status: kernel.FieldDefaulted, Origin: kernel.OriginRemembered, Value: kernel.Text(domain.IsolationVerbatim)}, stateOf(form, create.PathIsolation))
	assert.Equal(t, kernel.FieldState{Path: create.PathFastForward, Status: kernel.FieldDefaulted, Origin: kernel.OriginRemembered, Value: kernel.Bool(true)}, stateOf(form, create.PathFastForward))
}

func TestTheFastForwardIsAskedOnlyWhenTheBranchItStartsFromIsBehind(t *testing.T) {
	upToDate := newWorld(t, seed{})
	behind := newWorld(t, seed{Behind: true})

	assert.Equal(t, kernel.FieldSkipped, stateOf(upToDate.form(t, create.Request{Branches: []string{branch}}), create.PathFastForward).Status)
	assert.Equal(t, kernel.FieldState{Path: create.PathFastForward, Status: kernel.FieldDefaulted, Origin: kernel.OriginDefault, Value: kernel.Bool(false)},
		stateOf(behind.form(t, create.Request{Branches: []string{branch}}), create.PathFastForward))
}

func TestIfNotExistsIsSetByItsFlagOnly(t *testing.T) {
	w := newWorld(t, seed{})

	state := stateOf(w.form(t, create.Request{Branches: []string{branch}, IfNotExists: true}), create.PathIfNotExists)

	assert.Equal(t, kernel.FieldSkipped, state.Status)
	assert.Equal(t, kernel.Bool(true), state.Value)
}

func TestTheSourcePickerListsEveryBranchButThoseBeingCreated(t *testing.T) {
	w := newWorld(t, seed{Behind: true})
	field := create.Command(w.ports).Fields[1]

	choices, err := field.Choices(t.Context(), create.Request{Branches: []string{existing}}, w.facts(t))

	require.NoError(t, err)
	assert.Equal(t, "main", choices.Pinned)
	values := make([]string, 0, len(choices.Options))
	for _, option := range choices.Options {
		values = append(values, option.Value)
	}
	assert.Equal(t, []string{"main", "origin/" + remote}, values)
	assert.Equal(t, []kernel.Badge{{Code: create.CodeBadgeBehind, Params: kernel.Params{create.ParamAhead: "0", create.ParamBehind: "1"}}}, choices.Options[0].Badges)
}
