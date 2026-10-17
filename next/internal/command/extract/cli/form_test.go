package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

func TestTheRecapAppliesThePlanItPreviewedAndPreviewsAgainWhenItTurnedStale(t *testing.T) {
	stale := &domain.Error{Kind: domain.KindPrecondition, Code: domain.CodePlanStale}
	app := &previewing{useCase: newWorld(seed{}).usecase(), errs: []error{stale, nil}}
	streams, _, _, _ := output.Test()
	req := settled()
	req.Keep = new(false)

	_, err := newForm(nil).Ask(t.Context(), form.AskParams[useCase, extract.Request]{App: app, Request: req, In: enters{}, Out: streams.Err})

	require.NoError(t, err)
	assert.Equal(t, []string{"preview-1", "preview-2"}, app.held)
}

func TestADryRunWithNothingToAskDrawsNothing(t *testing.T) {
	streams, _, _, out := output.Test()
	req := settled()
	req.Keep = new(false)

	ran, err := newForm(nil).Ask(t.Context(), form.AskParams[useCase, extract.Request]{App: newWorld(seed{}).usecase(), Request: req, Options: domain.RunOptions{DryRun: true}, In: streams.In, Out: streams.Err})

	require.NoError(t, err)
	assert.Equal(t, req, ran.Request)
	assert.NotEmpty(t, ran.Outcome.Fingerprint)
	assert.Empty(t, out.String())
}

func TestTheRecapRunsThePreviewAndTheApplyWithTheCorrelationID(t *testing.T) {
	app := &previewing{useCase: newWorld(seed{}).usecase(), errs: []error{nil}}
	streams, _, _, _ := output.Test()
	req := settled()
	req.Keep = new(false)

	_, err := newForm(nil).Ask(t.Context(), form.AskParams[useCase, extract.Request]{App: app, Request: req, Options: domain.RunOptions{CorrelationID: "corr-1"}, In: enters{}, Out: streams.Err})

	require.NoError(t, err)
	assert.Equal(t, []string{"corr-1", "corr-1"}, app.ids)
}
