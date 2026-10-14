package store_test

import (
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	domainstatefile "github.com/LucasPcq/wtm/internal/domain/statefile"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/store"
	"github.com/LucasPcq/wtm/internal/testutil/globaldir"
)

func TestRememberWritesNoDefaultThatWasNotInTheFile(t *testing.T) {
	globaldir.Isolate(t)
	stateDir := t.TempDir()
	writeFile(t, filepath.Join(stateDir, domain.ConfigFileName), sparseConfig)

	require.NoError(t, errOf(store.ConfigDisk{StateDir: stateDir}.Remember(t.Context(), rules.RememberedChange{Remember: map[string]string{domain.RememberIsolation: "verbatim"}})))

	project, err := config.LoadProjectRaw(stateDir)
	require.NoError(t, err)
	assert.Equal(t, domain.WorktreesConfig{BasePath: "../.trees"}, project.Worktrees, "the default base branch is not written in")
	assert.Equal(t, domain.EnvConfig{Files: []domain.EnvFile{{Target: ".env"}}}, project.Env, "the default strategy is not written in")
	assert.Empty(t, project.Hooks.OnCreate)
	assert.Equal(t, map[string]string{domain.RememberIsolation: "verbatim"}, project.Wizard.Remembered)
}

func TestRememberKeepsAndForgetsOnTopOfWhatIsOnDisk(t *testing.T) {
	stateDir := t.TempDir()
	writeFile(t, filepath.Join(stateDir, domain.ConfigFileName), sparseConfig+"\n[wizard.remembered]\nisolation = \"verbatim\"\nenv_strategy = \"parent\"\n")

	require.NoError(t, errOf(store.ConfigDisk{StateDir: stateDir}.Remember(t.Context(), rules.RememberedChange{
		Remember: map[string]string{domain.RememberSourceUpdate: "ff"},
		Forget:   []string{domain.RememberIsolation},
	})))

	project, err := config.LoadProjectRaw(stateDir)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{domain.RememberEnvStrategy: "parent", domain.RememberSourceUpdate: "ff"}, project.Wizard.Remembered)
}

func TestRemembersFromSeveralGoroutinesAllLand(t *testing.T) {
	stateDir := t.TempDir()
	writeFile(t, filepath.Join(stateDir, domain.ConfigFileName), sparseConfig)
	answers := map[string]string{domain.RememberIsolation: "verbatim", domain.RememberEnvStrategy: "parent", domain.RememberSourceUpdate: "ff"}

	var wg sync.WaitGroup
	for key, value := range answers {
		wg.Go(func() {
			assert.NoError(t, errOf(store.ConfigDisk{StateDir: stateDir}.Remember(t.Context(), rules.RememberedChange{Remember: map[string]string{key: value}})))
		})
	}
	wg.Wait()

	project, err := config.LoadProjectRaw(stateDir)
	require.NoError(t, err)
	assert.Equal(t, answers, project.Wizard.Remembered)
}

func TestAnEmptyChangeWritesNothing(t *testing.T) {
	stateDir := t.TempDir()

	require.NoError(t, errOf(store.ConfigDisk{StateDir: stateDir}.Remember(t.Context(), rules.RememberedChange{})))

	assert.NoFileExists(t, filepath.Join(stateDir, domain.ConfigFileName))
}

func TestRememberNeedsAConfig(t *testing.T) {
	_, err := store.ConfigDisk{StateDir: t.TempDir()}.Remember(t.Context(), rules.RememberedChange{Remember: map[string]string{domain.RememberIsolation: "verbatim"}})

	require.ErrorIs(t, err, domain.ErrConfigNotFound)
}

func TestAConfigAtANewerSchemaIsNeverRewritten(t *testing.T) {
	stateDir := t.TempDir()
	content := "schema = 99\n" + sparseConfig
	path := filepath.Join(stateDir, domain.ConfigFileName)
	writeFile(t, path, content)

	_, err := store.ConfigDisk{StateDir: stateDir}.Remember(t.Context(), rules.RememberedChange{Remember: map[string]string{domain.RememberIsolation: "verbatim"}})

	require.ErrorIs(t, err, domainstatefile.ErrSchemaNewer)
	assert.Equal(t, content, readFile(t, path))
}

func TestUpdateWritesTheSchemaBesideTheConfig(t *testing.T) {
	stateDir := t.TempDir()
	writeFile(t, filepath.Join(stateDir, domain.ConfigFileName), sparseConfig)

	require.NoError(t, errOf(store.ConfigDisk{StateDir: stateDir}.Update(t.Context(), func(project *domain.ProjectConfig) {
		project.Worktrees.BasePath = "../elsewhere"
	})))

	project, err := config.LoadProjectRaw(stateDir)
	require.NoError(t, err)
	assert.Equal(t, "../elsewhere", project.Worktrees.BasePath)
	assert.FileExists(t, filepath.Join(stateDir, domain.SchemasDirName, "project.schema.json"))
}
