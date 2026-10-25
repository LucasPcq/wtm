package create_test

import (
	"errors"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARepositoryWithoutConfigIsAPrecondition(t *testing.T) {
	w := newWorld(t, seed{})
	require.NoError(t, os.Remove(filepath.Join(w.stateDir, "config.toml")))

	_, err := w.usecase.Observe(t.Context(), []string{branchName})

	failure, ok := errors.AsType[*domain.Error](err)
	require.True(t, ok, err)
	assert.Equal(t, domain.KindPrecondition, failure.Kind)
	assert.Equal(t, create.CodeConfigNotFound, failure.Code)
	assert.Equal(t, "no wtm config found", failure.Message)
}

func TestTheFactsCountOnlyTheBranchesThatDifferFromOrigin(t *testing.T) {
	w := newWorld(t, seed{Behind: true})

	assert.Equal(t, map[string]worktree.AheadBehind{"main": {Behind: 1}}, w.facts(t).Divergence)
}

func TestTheFactsLocateTheFolderOfEachBranchAsked(t *testing.T) {
	w := newWorld(t, seed{})
	require.NoError(t, os.MkdirAll(w.path("plain"), 0o755))

	facts := w.facts(t, "plain", branchName, "  ")

	assert.Equal(t, map[string]worktree.Location{
		w.path("plain"):    {Path: w.path("plain"), Exists: true},
		w.path(branchName): {Path: w.path(branchName)},
	}, facts.Folders)
}

func TestARunConfigThatDoesNotLoadIsAFactNotAFailure(t *testing.T) {
	w := newWorld(t, seed{Run: "[[job]]\nname = 3\n"})

	facts := w.facts(t)

	assert.Error(t, facts.RunErr)
	assert.Equal(t, project.Run{}, facts.Run)
}

func TestThePlanSaysWhatBecomesOfEachBranch(t *testing.T) {
	added := []domain.Phase{domain.PhaseWorktree, domain.PhaseEnv, domain.PhaseMeta}
	w := newWorld(t, seed{Linked: []string{"linked"}})

	planned, warnings := w.plan(t, create.Request{Branches: []string{branchName, existing, remote, "linked"}, From: "main", IfNotExists: true})

	assert.Equal(t, create.Plan{
		Branches: []create.BranchPlan{
			{Branch: branchName, Path: w.path(branchName), Action: create.ActionCreate, Phases: added},
			{Branch: existing, Path: w.path(existing), Action: create.ActionCheckOut, Phases: added},
			{Branch: remote, Path: w.path(remote), Action: create.ActionResume, Upstream: "origin/" + remote, Phases: added},
			{Branch: "linked", Path: linkedPath(w.root, "linked"), Action: create.ActionUnchanged, Phases: []domain.Phase{}},
		},
		From:    "main",
		EnvFrom: project.EnvMain,
	}, shown(planned))
	assert.Empty(t, warnings)
}

func TestThePlanNamesTheIsolationOnlyWhenTheRunIsolatesSomething(t *testing.T) {
	isolating := newWorld(t, seed{Run: jobsToml})
	bare := newWorld(t, seed{})

	given, _ := isolating.plan(t, create.Request{Branches: []string{branchName}, Given: provision.Given{Isolation: project.Verbatim}})
	defaulted, _ := isolating.plan(t, create.Request{Branches: []string{branchName}})
	ignored, _ := bare.plan(t, create.Request{Branches: []string{branchName}, Given: provision.Given{Isolation: project.Verbatim}})

	assert.Equal(t, project.Verbatim, given.Isolation)
	assert.Equal(t, project.Isolated, defaulted.Isolation)
	assert.Empty(t, ignored.Isolation)
}

func TestThePlanFastForwardsOnlyWhatIsBehindAndOnlyWhenAsked(t *testing.T) {
	w := newWorld(t, seed{Behind: true})

	kept, _ := w.plan(t, create.Request{Branches: []string{branchName}})
	asked, _ := w.plan(t, create.Request{Branches: []string{branchName}, Given: provision.Given{FastForward: new(true)}})
	fromRemote, _ := w.plan(t, create.Request{Branches: []string{branchName}, From: "origin/main", Given: provision.Given{FastForward: new(true)}})

	assert.Empty(t, kept.FastForward)
	assert.Equal(t, []string{"main"}, asked.FastForward)
	assert.Empty(t, fromRemote.FastForward)
}

func TestAnExistingBranchIsItsOwnFastForwardSubject(t *testing.T) {
	w := newWorld(t, seed{})
	behindOrigin(t, w, existing)

	planned, _ := w.plan(t, create.Request{Branches: []string{existing}, From: "main", Given: provision.Given{FastForward: new(true)}})

	assert.Equal(t, []string{existing}, planned.FastForward)
}

func TestADivergedSourceIsAWarningOfThePlan(t *testing.T) {
	w := newWorld(t, seed{Diverged: true})

	_, warnings := w.plan(t, create.Request{Branches: []string{branchName}})

	assert.Equal(t, []domain.Warning{{
		Code:    "create.source_diverged",
		Message: "branch main has diverged from origin (1 ahead, 1 behind) and can't be fast-forwarded: the worktree starts from your local branch, missing commits that are on origin — you may have to rebase or resolve conflicts later",
		Params:  domain.Params{"branch": "main", "ahead": "1", "behind": "1"},
	}}, warnings)
}

func TestTheParentStrategyWithoutTheSourcesWorktreeFallsBackToMainAndSaysSo(t *testing.T) {
	w := newWorld(t, seed{Linked: []string{"linked"}})

	_, fromMain := w.plan(t, create.Request{Branches: []string{branchName}, Given: provision.Given{EnvFrom: project.EnvParent}})
	_, fromExisting := w.plan(t, create.Request{Branches: []string{branchName}, From: existing, Given: provision.Given{EnvFrom: project.EnvParent}})
	_, unchanged := w.plan(t, create.Request{Branches: []string{"linked"}, From: existing, Given: provision.Given{EnvFrom: project.EnvParent}, IfNotExists: true})

	assert.Empty(t, fromMain)
	assert.Equal(t, []domain.Warning{{Code: "create.env_parent_fallback", Message: "The \"parent\" env strategy needs the source branch checked out to copy its .env; without a worktree it comes from main."}}, fromExisting)
	assert.Empty(t, unchanged)
}
