package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARepositoryIsAddedOnceInTheFormatWtmShares(t *testing.T) {
	registry, repo := registryIn(t), initialized(t)

	first, err := registry.Add(t.Context(), repo)
	require.NoError(t, err)
	second, err := registry.Add(t.Context(), repo)
	require.NoError(t, err)

	assert.True(t, first)
	assert.False(t, second)
	assert.Equal(t, "[\n  {\n    \"common_dir\": \""+repo.CommonDir+"\",\n    \"root\": \""+repo.Root+"\",\n    \"added_at\": \"2026-10-16T12:00:00Z\"\n  }\n]\n", readFile(t, registry.Path))
}

func TestARepositoryWithNoWtmConfigIsNotAdded(t *testing.T) {
	registry, repo := registryIn(t), initialized(t)
	require.NoError(t, os.Remove(filepath.Join(repo.CommonDir, "wtm", "config.toml")))

	added, err := registry.Add(t.Context(), repo)

	require.NoError(t, err)
	assert.False(t, added)
	assert.NoFileExists(t, registry.Path)
}

func TestAnUnparsableRegistryIsRebuiltFromEmpty(t *testing.T) {
	registry, repo := registryIn(t), initialized(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(registry.Path), 0o755))
	require.NoError(t, os.WriteFile(registry.Path, []byte("{not json"), 0o644))

	added, err := registry.Add(t.Context(), repo)

	require.NoError(t, err)
	assert.True(t, added)
	kept, _, err := registry.Prune(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []domain.Repo{repo}, kept)
}

func TestPruneDropsTheRepositoriesNoLongerInitialized(t *testing.T) {
	registry, stays, leaves := registryIn(t), initialized(t), initialized(t)
	for _, repo := range []domain.Repo{stays, leaves} {
		_, err := registry.Add(t.Context(), repo)
		require.NoError(t, err)
	}
	require.NoError(t, os.Remove(filepath.Join(leaves.CommonDir, "wtm", "config.toml")))

	kept, dropped, err := registry.Prune(t.Context())

	require.NoError(t, err)
	assert.Equal(t, []domain.Repo{stays}, kept)
	assert.Equal(t, []domain.Repo{leaves}, dropped)
	again, droppedAgain, err := registry.Prune(t.Context())
	require.NoError(t, err)
	assert.Equal(t, []domain.Repo{stays}, again)
	assert.Empty(t, droppedAgain)
}

func TestPruningWhatIsNotThereKeepsNothingAndWritesNothing(t *testing.T) {
	registry := registryIn(t)

	kept, dropped, err := registry.Prune(t.Context())

	require.NoError(t, err)
	assert.Empty(t, kept)
	assert.Empty(t, dropped)
	assert.NoFileExists(t, registry.Path)
}

func TestTheRegistryLiesInWtmsConfigDirectory(t *testing.T) {
	assert.Equal(t, filepath.Join("wtm", "repos.json"), filepath.Join(filepath.Base(filepath.Dir(Path())), filepath.Base(Path())))
}
