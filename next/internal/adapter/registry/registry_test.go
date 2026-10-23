package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestARepositoryIsAddedOnce(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			first, err := impl.port.Add(t.Context(), stays)
			require.NoError(t, err)
			second, err := impl.port.Add(t.Context(), domain.Repo{Root: "/elsewhere", CommonDir: stays.CommonDir})
			require.NoError(t, err)

			assert.True(t, first)
			assert.False(t, second)
			kept, _, err := impl.port.Prune(t.Context(), keepAll)
			require.NoError(t, err)
			assert.Equal(t, []domain.Repo{stays}, kept)
		})
	}
}

func TestPruneDropsTheRepositoriesKeepRefuses(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			for _, repo := range []domain.Repo{stays, leaves} {
				_, err := impl.port.Add(t.Context(), repo)
				require.NoError(t, err)
			}

			kept, dropped, err := impl.port.Prune(t.Context(), func(repo domain.Repo) bool { return repo != leaves })

			require.NoError(t, err)
			assert.Equal(t, []domain.Repo{stays}, kept)
			assert.Equal(t, []domain.Repo{leaves}, dropped)
			again, droppedAgain, err := impl.port.Prune(t.Context(), keepAll)
			require.NoError(t, err)
			assert.Equal(t, []domain.Repo{stays}, again)
			assert.Empty(t, droppedAgain)
		})
	}
}

func TestARepositoryIsWrittenInTheFormatWtmShares(t *testing.T) {
	registry := registryIn(t)

	_, err := registry.Add(t.Context(), stays)

	require.NoError(t, err)
	assert.Equal(t, "[\n  {\n    \"common_dir\": \"/code/stays/.git\",\n    \"root\": \"/code/stays\",\n    \"added_at\": \"2026-10-16T12:00:00Z\"\n  }\n]\n", readFile(t, registry.Path))
}

func TestAnUnparsableRegistryIsRebuiltFromEmpty(t *testing.T) {
	registry := registryIn(t)
	require.NoError(t, os.MkdirAll(filepath.Dir(registry.Path), 0o755))
	require.NoError(t, os.WriteFile(registry.Path, []byte("{not json"), 0o644))

	added, err := registry.Add(t.Context(), stays)

	require.NoError(t, err)
	assert.True(t, added)
	kept, _, err := registry.Prune(t.Context(), keepAll)
	require.NoError(t, err)
	assert.Equal(t, []domain.Repo{stays}, kept)
}

func TestPruningAnEmptyRegistryKeepsAndDropsNil(t *testing.T) {
	for _, impl := range implementations(t) {
		t.Run(impl.name, func(t *testing.T) {
			kept, dropped, err := impl.port.Prune(t.Context(), keepAll)

			require.NoError(t, err)
			assert.Nil(t, kept)
			assert.Nil(t, dropped)
		})
	}
}

func TestPruningWhatIsNotThereWritesNothing(t *testing.T) {
	registry := registryIn(t)

	_, _, err := registry.Prune(t.Context(), keepAll)

	require.NoError(t, err)
	assert.NoFileExists(t, registry.Path)
}

func TestTheRegistryLiesInWtmsConfigDirectory(t *testing.T) {
	assert.Equal(t, filepath.Join("wtm", "repos.json"), filepath.Join(filepath.Base(filepath.Dir(Path())), filepath.Base(Path())))
}
