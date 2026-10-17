package dotenv

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func onlyPair(t *testing.T, raw string) Line {
	t.Helper()
	lines := Parse([]byte(raw))
	require.Len(t, lines, 1)
	require.Equal(t, Pair, lines[0].Kind)
	return lines[0]
}

func valueOf(t *testing.T, data []byte, key string) string {
	t.Helper()
	for _, line := range Parse(data) {
		if line.Kind == Pair && line.Key == key {
			return line.Value
		}
	}
	require.Failf(t, "key not found", "%s in %q", key, data)
	return ""
}
