package cli

import (
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestADryRunWithNothingToAskDrawsNothing(t *testing.T) {
	streams, _, _, out := output.Test()
	req := create.Request{Branches: []string{"feat/a"}, From: "main", EnvFrom: project.EnvExample}

	ran, err := runForm(t.Context(), formParams{App: newApp(seed{}), Request: req, DryRun: true, In: streams.In, Out: streams.Err})

	require.NoError(t, err)
	assert.Equal(t, req, ran.Request)
	assert.NotEmpty(t, ran.Outcome.Fingerprint)
	assert.Empty(t, out.String())
}

func TestTheFormAppliesThePlanItPreviewedAndPreviewsAgainWhenItTurnedStale(t *testing.T) {
	stale := &domain.Error{Kind: domain.KindPrecondition, Code: operation.CodePlanStale}
	app := &previewing{useCase: newApp(seed{}), errs: []error{stale, nil}}
	streams, _, _, _ := output.Test()
	req := create.Request{Branches: []string{"feat/a"}, From: "main", EnvFrom: project.EnvExample}

	_, err := runForm(t.Context(), formParams{App: app, Request: req, In: enters{}, Out: streams.Err})

	require.NoError(t, err)
	assert.Equal(t, []string{"preview-1", "preview-2"}, app.held)
}

func TestAFailedApplyEndsTheFormWithoutAskingAgain(t *testing.T) {
	failed := errors.New("hook failed")
	app := &previewing{useCase: newApp(seed{}), errs: []error{failed}}
	streams, _, _, _ := output.Test()
	req := create.Request{Branches: []string{"feat/a"}, From: "main", EnvFrom: project.EnvExample}

	_, err := runForm(t.Context(), formParams{App: app, Request: req, In: enters{}, Out: streams.Err})

	assert.ErrorIs(t, err, failed)
	assert.Equal(t, []string{"preview-1"}, app.held)
}
