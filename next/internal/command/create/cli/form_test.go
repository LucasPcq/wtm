package cli

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/service/provision"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

func TestADryRunWithNothingToAskDrawsNothing(t *testing.T) {
	streams, _, _, out := output.Test()
	req := create.Request{Branches: []string{"feat/a"}, From: "main", Given: provision.Given{EnvFrom: project.EnvExample}}

	ran, err := newForm(nil).Ask(t.Context(), form.AskParams[create.Request, create.Questions, create.Outcome]{App: newApp(seed{}), Request: req, Options: domain.RunOptions{DryRun: true}, In: streams.In, Out: streams.Err})

	require.NoError(t, err)
	assert.Equal(t, req, ran.Request)
	assert.NotEmpty(t, ran.Outcome.Fingerprint)
	assert.Empty(t, out.String())
}

func TestTheFormAppliesThePlanItPreviewedAndPreviewsAgainWhenItTurnedStale(t *testing.T) {
	stale := &domain.Error{Kind: domain.KindPrecondition, Code: domain.CodePlanStale}
	app := &previewing{UseCase: newApp(seed{}), Errs: []error{stale, nil}}
	streams, _, _, _ := output.Test()
	req := create.Request{Branches: []string{"feat/a"}, From: "main", Given: provision.Given{EnvFrom: project.EnvExample}}

	_, err := newForm(nil).Ask(t.Context(), form.AskParams[create.Request, create.Questions, create.Outcome]{App: app, Request: req, In: formtest.Enters{}, Out: streams.Err})

	require.NoError(t, err)
	assert.Equal(t, []string{"preview-1", "preview-2"}, app.Held)
}

func TestAFailedApplyEndsTheFormWithoutAskingAgain(t *testing.T) {
	failed := errors.New("hook failed")
	app := &previewing{UseCase: newApp(seed{}), Errs: []error{failed}}
	streams, _, _, _ := output.Test()
	req := create.Request{Branches: []string{"feat/a"}, From: "main", Given: provision.Given{EnvFrom: project.EnvExample}}

	_, err := newForm(nil).Ask(t.Context(), form.AskParams[create.Request, create.Questions, create.Outcome]{App: app, Request: req, In: formtest.Enters{}, Out: streams.Err})

	assert.ErrorIs(t, err, failed)
	assert.Equal(t, []string{"preview-1"}, app.Held)
}

func TestTheFormRunsThePreviewAndTheApplyWithTheCorrelationID(t *testing.T) {
	app := &previewing{UseCase: newApp(seed{}), Errs: []error{nil}}
	streams, _, _, _ := output.Test()
	req := create.Request{Branches: []string{"feat/a"}, From: "main", Given: provision.Given{EnvFrom: project.EnvExample}}

	_, err := newForm(nil).Ask(t.Context(), form.AskParams[create.Request, create.Questions, create.Outcome]{App: app, Request: req, Options: domain.RunOptions{CorrelationID: "corr-1"}, In: formtest.Enters{}, Out: streams.Err})

	require.NoError(t, err)
	assert.Equal(t, []string{"corr-1", "corr-1"}, app.IDs)
}

func TestTheRecapNamesTheWorktreesThePlanOpens(t *testing.T) {
	opening := func(count int) create.Outcome { return create.Outcome{Plan: create.Plan{Creates: count}} }

	assert.Equal(t, "Yes, create worktree", applyLabel(opening(1)))
	assert.Equal(t, "Yes, create 2 worktrees", applyLabel(opening(2)))
	assert.Equal(t, "Yes, apply", applyLabel(opening(0)))
}
