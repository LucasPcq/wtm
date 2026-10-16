package create_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/LucasPcq/wtm/internal/command/create"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestThePlanSaysWhatBecomesOfEachBranch(t *testing.T) {
	w := newWorld(t, seed{Linked: []string{"linked"}})

	plan := w.preview(t, create.Request{Branches: []string{branch, existing, remote, "linked"}, From: "main", IfNotExists: true})

	assert.Equal(t, []create.Planned{
		{Branch: branch, Path: w.path(branch), Action: create.ActionCreate},
		{Branch: existing, Path: w.path(existing), Action: create.ActionCheckOut},
		{Branch: remote, Path: w.path(remote), Action: create.ActionResume, Upstream: "origin/" + remote},
		{Branch: "linked", Path: linkedPath(w.root, "linked"), Action: create.ActionUnchanged},
	}, plan.Detail.Branches)
	assert.Equal(t, "main", plan.Detail.From)
	assert.Equal(t, domain.EnvStrategyMain, plan.Detail.EnvFrom)
	assert.Equal(t, domain.IsolationIsolated, plan.Detail.Isolation)
}

func TestThePlanFastForwardsOnlyWhatIsBehindAndOnlyWhenAsked(t *testing.T) {
	w := newWorld(t, seed{Behind: true})

	kept := w.preview(t, create.Request{Branches: []string{branch}})
	asked := w.preview(t, create.Request{Branches: []string{branch}, FastForward: true})

	assert.Empty(t, kept.Detail.FastForward)
	assert.Equal(t, []string{"main"}, asked.Detail.FastForward)
}

func TestADivergedSourceIsAWarningOfThePlan(t *testing.T) {
	w := newWorld(t, seed{Diverged: true})

	plan := w.preview(t, create.Request{Branches: []string{branch}})

	assert.Equal(t, []kernel.Warning{{Code: create.CodeSourceDiverged, Params: kernel.Params{create.ParamBranch: "main", create.ParamAhead: "1", create.ParamBehind: "1"}}}, plan.Warnings)
}

func TestTheParentStrategyWithoutTheSourcesWorktreeFallsBackToMainAndSaysSo(t *testing.T) {
	w := newWorld(t, seed{})

	fromMain := w.preview(t, create.Request{Branches: []string{branch}, EnvFrom: domain.EnvStrategyParent})
	fromExisting := w.preview(t, create.Request{Branches: []string{branch}, From: existing, EnvFrom: domain.EnvStrategyParent})

	assert.Empty(t, fromMain.Warnings)
	assert.Equal(t, []kernel.Code{create.CodeEnvParentFallback}, codes(fromExisting.Warnings))
}
