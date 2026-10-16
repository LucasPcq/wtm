package envcopy

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func sources(t *testing.T, params SourcesParams) ([]Source, []Warning) {
	t.Helper()
	found, warnings, err := Sources(params)
	require.NoError(t, err)
	return found, warnings
}
