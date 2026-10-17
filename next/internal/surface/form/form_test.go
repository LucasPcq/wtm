package form

import (
	"bytes"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestADryRunWithNothingToAskDrawsNothing(t *testing.T) {
	var out bytes.Buffer
	req := create.Request{Branches: []string{"feat/a"}, From: "main", EnvFrom: project.EnvExample}

	ran, err := Run(t.Context(), Params{App: newApp(seed{}), Request: req, DryRun: true, In: &bytes.Buffer{}, Out: &out})

	require.NoError(t, err)
	assert.Equal(t, req, ran.Request)
	assert.NotEmpty(t, ran.Outcome.Fingerprint)
	assert.Empty(t, out.String())
}
