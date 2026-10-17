package worktree

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAnIdentityWithoutAnOrdinalSaysNull(t *testing.T) {
	data, err := json.Marshal(Identity{Branch: "feat/x", Path: "/r/.trees/feat-x", Parent: "main", Isolation: "isolated", CreatedAt: "2026-10-16T10:00:00Z"})

	require.NoError(t, err)
	assert.JSONEq(t, `{"branch":"feat/x","path":"/r/.trees/feat-x","parent":"main","ordinal":null,"isolation":"isolated","is_main":false,"created_at":"2026-10-16T10:00:00Z"}`, string(data))
}
