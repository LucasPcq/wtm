package worktree

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAMetaDirHoldsTheBranchAsOneEscapedSegment(t *testing.T) {
	dir, ok := MetaDir(MetaDirParams{StateDir: "/state", Branch: "feat/x"})

	require.True(t, ok)
	assert.Equal(t, filepath.Join("/state", "worktrees", "feat%2Fx"), dir)
}

func TestAMetaDirIsRefusedRatherThanGuessed(t *testing.T) {
	cases := map[string]MetaDirParams{
		"no state dir":      {Branch: "feat/x"},
		"no branch":         {StateDir: "/state"},
		"a branch named ..": {StateDir: "/state", Branch: ".."},
		"a branch named .":  {StateDir: "/state", Branch: "."},
	}
	for name, params := range cases {
		t.Run(name, func(t *testing.T) {
			dir, ok := MetaDir(params)

			assert.False(t, ok)
			assert.Empty(t, dir)
		})
	}
}

func TestMetaJSONIsTheRecordWtmWrites(t *testing.T) {
	data, err := json.Marshal(Meta{SourceBranch: "main", CreatedAt: "2026-10-16T10:00:00Z", EnvStrategy: "example", Ordinal: 3, Isolation: "isolated"})

	require.NoError(t, err)
	assert.JSONEq(t, `{"source_branch":"main","created_at":"2026-10-16T10:00:00Z","env_strategy":"example","ordinal":3,"isolation":"isolated"}`, string(data))
}

func TestAnIdentityWithoutAnOrdinalSaysNull(t *testing.T) {
	data, err := json.Marshal(Identity{Branch: "feat/x", Path: "/r/.trees/feat-x", Parent: "main", Isolation: "isolated", CreatedAt: "2026-10-16T10:00:00Z"})

	require.NoError(t, err)
	assert.JSONEq(t, `{"branch":"feat/x","path":"/r/.trees/feat-x","parent":"main","ordinal":null,"isolation":"isolated","is_main":false,"created_at":"2026-10-16T10:00:00Z"}`, string(data))
}
