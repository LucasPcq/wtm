package git_test

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestASnapshotIsTheWorktreesTheBranchesAndTheCountsOfThoseThatDifferFromOrigin(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			worktrees, err := impl.port.Worktrees(t.Context())
			require.NoError(t, err)
			branches, err := impl.port.Branches(t.Context())
			require.NoError(t, err)

			snapshot, err := impl.port.Snapshot(t.Context())

			require.NoError(t, err)
			assert.Equal(t, worktree.Snapshot{
				Worktrees:  worktrees,
				Branches:   branches,
				Divergence: map[string]worktree.AheadBehind{"diverged": {Ahead: 1}},
			}, snapshot)
		})
	}
}
