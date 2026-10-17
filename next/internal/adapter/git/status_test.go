package git_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheFakeAndGitTellADirtyWorktreeFromACleanOne(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			dirty, dirtyErr := impl.port.Dirty(t.Context(), impl.at("dirty"))
			clean, cleanErr := impl.port.Dirty(t.Context(), impl.at("linked"))
			_, notErr := impl.port.Dirty(t.Context(), filepath.Join(impl.root, "taken", "nowhere"))
			_, goneErr := impl.port.Dirty(t.Context(), impl.at("gone"))

			require.NoError(t, dirtyErr)
			assert.True(t, dirty)
			require.NoError(t, cleanErr)
			assert.False(t, clean)
			require.Error(t, notErr)
			require.Error(t, goneErr)
		})
	}
}
