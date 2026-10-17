package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
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
