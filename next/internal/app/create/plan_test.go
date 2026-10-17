package create_test

import (
	"errors"
	"os"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestThePlanSaysWhatBecomesOfEachBranch(t *testing.T) {
	w := newWorld(t, seed{Linked: []string{"linked"}})

	planned, warnings := w.plan(t, create.Request{Branches: []string{branchName, existing, remote, "linked"}, From: "main", IfNotExists: true})

	assert.Equal(t, create.Planned{
		Branches: []create.BranchPlan{
			{Branch: branchName, Path: w.path(branchName), Action: create.ActionCreate},
			{Branch: existing, Path: w.path(existing), Action: create.ActionCheckOut},
			{Branch: remote, Path: w.path(remote), Action: create.ActionResume, Upstream: "origin/" + remote},
			{Branch: "linked", Path: linkedPath(w.root, "linked"), Action: create.ActionUnchanged},
		},
		From:    "main",
		EnvFrom: project.EnvMain,
	}, planned)
	assert.Empty(t, warnings)
}

func TestThePlanNamesTheIsolationOnlyWhenTheRunIsolatesSomething(t *testing.T) {
	isolating := newWorld(t, seed{Run: jobsToml})
	bare := newWorld(t, seed{})

	given, _ := isolating.plan(t, create.Request{Branches: []string{branchName}, Isolation: project.Verbatim})
	defaulted, _ := isolating.plan(t, create.Request{Branches: []string{branchName}})
	ignored, _ := bare.plan(t, create.Request{Branches: []string{branchName}, Isolation: project.Verbatim})

	assert.Equal(t, project.Verbatim, given.Isolation)
	assert.Equal(t, project.Isolated, defaulted.Isolation)
	assert.Empty(t, ignored.Isolation)
}

func TestThePlanFastForwardsOnlyWhatIsBehindAndOnlyWhenAsked(t *testing.T) {
	w := newWorld(t, seed{Behind: true})

	kept, _ := w.plan(t, create.Request{Branches: []string{branchName}})
	asked, _ := w.plan(t, create.Request{Branches: []string{branchName}, FastForward: new(true)})
	fromRemote, _ := w.plan(t, create.Request{Branches: []string{branchName}, From: "origin/main", FastForward: new(true)})

	assert.Empty(t, kept.FastForward)
	assert.Equal(t, []string{"main"}, asked.FastForward)
	assert.Empty(t, fromRemote.FastForward)
}

func TestAnExistingBranchIsItsOwnFastForwardSubject(t *testing.T) {
	w := newWorld(t, seed{})
	behindOrigin(t, w, existing)

	planned, _ := w.plan(t, create.Request{Branches: []string{existing}, From: "main", FastForward: new(true)})

	assert.Equal(t, []string{existing}, planned.FastForward)
}

func TestADivergedSourceIsAWarningOfThePlan(t *testing.T) {
	w := newWorld(t, seed{Diverged: true})

	_, warnings := w.plan(t, create.Request{Branches: []string{branchName}})

	assert.Equal(t, []wtm.Warning{{
		Code:    "create.source_diverged",
		Message: "branch main has diverged from origin (1 ahead, 1 behind) and can't be fast-forwarded: the worktree starts from your local branch, missing commits that are on origin — you may have to rebase or resolve conflicts later",
		Params:  wtm.Params{"branch": "main", "ahead": "1", "behind": "1"},
	}}, warnings)
}

func TestTheParentStrategyWithoutTheSourcesWorktreeFallsBackToMainAndSaysSo(t *testing.T) {
	w := newWorld(t, seed{Linked: []string{"linked"}})

	_, fromMain := w.plan(t, create.Request{Branches: []string{branchName}, EnvFrom: project.EnvParent})
	_, fromExisting := w.plan(t, create.Request{Branches: []string{branchName}, From: existing, EnvFrom: project.EnvParent})
	_, unchanged := w.plan(t, create.Request{Branches: []string{"linked"}, From: existing, EnvFrom: project.EnvParent, IfNotExists: true})

	assert.Empty(t, fromMain)
	assert.Equal(t, []wtm.Warning{{Code: "create.env_parent_fallback", Message: "The \"parent\" env strategy needs the source branch checked out to copy its .env; without a worktree it comes from main."}}, fromExisting)
	assert.Empty(t, unchanged)
}

func TestAPlanOfARequestNeverResolvedRefusesItAsInvalid(t *testing.T) {
	w := newWorld(t, seed{})

	_, _, err := create.Plan(w.facts(t, "  "), create.Request{Branches: []string{"  "}, From: "main", EnvFrom: project.EnvMain})

	assert.Equal(t, []wtm.FieldError{{Path: "branches[0]", Code: "required", Template: "{path} is required"}}, fieldsOf(t, err))
}

func TestAFolderTakenSinceTheRequestWasResolvedIsAConflict(t *testing.T) {
	w := newWorld(t, seed{})
	facts := w.facts(t, branchName)
	resolved, err := create.Resolve(facts, create.Request{Branches: []string{branchName}})
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(w.path(branchName), 0o755))

	_, _, planErr := create.Plan(w.facts(t, branchName), resolved)

	failure, ok := errors.AsType[*wtm.Error](planErr)
	require.True(t, ok, planErr)
	assert.Equal(t, wtm.KindConflict, failure.Kind)
	assert.Equal(t, "create.path_exists", failure.Code)
	assert.Equal(t, "worktree path already exists: "+w.path(branchName), failure.Message)
}
