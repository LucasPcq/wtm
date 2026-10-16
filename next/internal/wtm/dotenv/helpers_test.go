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
