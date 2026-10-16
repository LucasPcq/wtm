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

func existsIn(present map[string]bool) func(string) (bool, error) {
	return func(path string) (bool, error) { return present[path], nil }
}
