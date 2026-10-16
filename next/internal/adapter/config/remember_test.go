package config

import (
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/BurntSushi/toml"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRememberingReRendersTheConfigAsWtmWritesIt(t *testing.T) {
	stateDir := stateWithConfig(t, golden(t, "full.before.toml"))

	written, err := Remember(t.Context(), RememberParams{StateDir: stateDir, Answers: map[string]string{project.RememberIsolation: "verbatim"}})

	require.NoError(t, err)
	assert.True(t, written.Changed)
	assert.Equal(t, golden(t, "full.after.toml"), readFile(t, filepath.Join(stateDir, "config.toml")))
}

func TestRememberingWritesNoDefaultThatWasNotInTheFile(t *testing.T) {
	stateDir := stateWithConfig(t, golden(t, "sparse.before.toml"))

	_, err := Remember(t.Context(), RememberParams{StateDir: stateDir, Answers: map[string]string{project.RememberIsolation: "verbatim"}})

	require.NoError(t, err)
	assert.Equal(t, golden(t, "sparse.after.toml"), readFile(t, filepath.Join(stateDir, "config.toml")))
	cfg, _, err := loadState(t, stateDir)
	require.NoError(t, err)
	assert.Equal(t, project.DefaultBaseBranch, cfg.Worktrees.BaseBranch, "the default is applied when loading, never written in")
}

func TestRememberingAddsToWhatIsOnDiskAndNeverForgets(t *testing.T) {
	stateDir := stateWithConfig(t, minimalConfig+"\n[wizard.remembered]\nisolation = \"verbatim\"\nenv_strategy = \"parent\"\n")

	_, err := Remember(t.Context(), RememberParams{StateDir: stateDir, Answers: map[string]string{project.RememberSourceUpdate: "ff", project.RememberEnvStrategy: "main"}})

	require.NoError(t, err)
	cfg, _, err := loadState(t, stateDir)
	require.NoError(t, err)
	assert.Equal(t, map[string]string{project.RememberIsolation: "verbatim", project.RememberEnvStrategy: "main", project.RememberSourceUpdate: "ff"}, cfg.Remembered)
}

func TestRememberingWhatIsAlreadyRememberedChangesNothing(t *testing.T) {
	stateDir := stateWithConfig(t, golden(t, "full.after.toml"))

	written, err := Remember(t.Context(), RememberParams{StateDir: stateDir, Answers: map[string]string{project.RememberIsolation: "verbatim"}})

	require.NoError(t, err)
	assert.False(t, written.Changed)
	assert.Equal(t, golden(t, "full.after.toml"), readFile(t, filepath.Join(stateDir, "config.toml")))
}

func TestRemembersFromSeveralGoroutinesAllLand(t *testing.T) {
	stateDir := stateWithConfig(t, golden(t, "sparse.before.toml"))
	answers := map[string]string{project.RememberIsolation: "verbatim", project.RememberEnvStrategy: "parent", project.RememberSourceUpdate: "ff"}

	var wg sync.WaitGroup
	for key, value := range answers {
		wg.Go(func() {
			_, err := Remember(t.Context(), RememberParams{StateDir: stateDir, Answers: map[string]string{key: value}})
			assert.NoError(t, err)
		})
	}
	wg.Wait()

	cfg, _, err := loadState(t, stateDir)
	require.NoError(t, err)
	assert.Equal(t, answers, cfg.Remembered)
}

func TestNoAnswerWritesNothing(t *testing.T) {
	stateDir := t.TempDir()

	written, err := Remember(t.Context(), RememberParams{StateDir: stateDir})

	require.NoError(t, err)
	assert.Equal(t, wtm.Written{}, written)
	assert.NoFileExists(t, filepath.Join(stateDir, "config.toml"))
	assert.NoDirExists(t, filepath.Join(stateDir, "schemas"))
}

func TestRememberingNeedsAConfig(t *testing.T) {
	stateDir := t.TempDir()

	_, err := Remember(t.Context(), RememberParams{StateDir: stateDir, Answers: map[string]string{project.RememberIsolation: "verbatim"}})

	require.ErrorIs(t, err, project.ErrConfigNotFound)
	assert.NoFileExists(t, filepath.Join(stateDir, "config.toml"))
}

func TestAConfigAtANewerSchemaIsNeverRewritten(t *testing.T) {
	content := newerSchema + golden(t, "sparse.before.toml")
	stateDir := stateWithConfig(t, content)

	_, err := Remember(t.Context(), RememberParams{StateDir: stateDir, Answers: map[string]string{project.RememberIsolation: "verbatim"}})

	require.ErrorIs(t, err, wtm.ErrSchemaNewer)
	var newer *wtm.SchemaNewer
	require.ErrorAs(t, err, &newer)
	assert.Equal(t, 99, newer.Found)
	assert.Equal(t, 1, newer.Supported)
	assert.Equal(t, content, readFile(t, filepath.Join(stateDir, "config.toml")))
}

func TestASchemaOneConfigIsWrittenWithoutASchemaKey(t *testing.T) {
	stateDir := stateWithConfig(t, "schema = 1\n"+golden(t, "sparse.before.toml"))

	_, err := Remember(t.Context(), RememberParams{StateDir: stateDir, Answers: map[string]string{project.RememberIsolation: "verbatim"}})

	require.NoError(t, err)
	assert.Equal(t, golden(t, "sparse.after.toml"), readFile(t, filepath.Join(stateDir, "config.toml")))
}

func TestRememberingWritesTheSchemaBesideTheConfig(t *testing.T) {
	stateDir := stateWithConfig(t, golden(t, "sparse.before.toml"))

	_, err := Remember(t.Context(), RememberParams{StateDir: stateDir, Answers: map[string]string{project.RememberIsolation: "verbatim"}})

	require.NoError(t, err)
	want, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "internal", "schemas", "project.schema.json"))
	require.NoError(t, err)
	assert.Equal(t, string(want), readFile(t, filepath.Join(stateDir, "schemas", "project.schema.json")))
}

func TestAnEmptyEnvSectionStaysCommented(t *testing.T) {
	stateDir := stateWithConfig(t, "[worktrees]\nbase_path = \"../.trees\"\nbase_branch = \"main\"\n")

	_, err := Remember(t.Context(), RememberParams{StateDir: stateDir, Answers: map[string]string{project.RememberIsolation: "verbatim"}})

	require.NoError(t, err)
	content := readFile(t, filepath.Join(stateDir, "config.toml"))
	assert.NotContains(t, content, "\nstrategy = ")
	assert.Contains(t, content, "# Skipped during init. Uncomment to enable .env provisioning.")
	var decoded map[string]any
	_, err = toml.Decode(content, &decoded)
	assert.NoError(t, err)
}

func TestEnvFilesWithoutAStrategyStayLiveAndTakeTheDefaultOnLoad(t *testing.T) {
	stateDir := stateWithConfig(t, golden(t, "sparse.before.toml"))

	_, err := Remember(t.Context(), RememberParams{StateDir: stateDir, Answers: map[string]string{project.RememberIsolation: "verbatim"}})

	require.NoError(t, err)
	assert.NotContains(t, readFile(t, filepath.Join(stateDir, "config.toml")), "\nstrategy = ")
	cfg, _, err := loadState(t, stateDir)
	require.NoError(t, err)
	assert.Equal(t, project.Env{Strategy: project.DefaultEnvStrategy, Files: []project.EnvFile{{Target: ".env"}}}, cfg.Env)
}

func TestEveryStringIsEncodedSoItReadsBackAsWritten(t *testing.T) {
	stateDir := stateWithConfig(t, awkwardConfig)

	_, err := Remember(t.Context(), RememberParams{StateDir: stateDir, Answers: map[string]string{project.RememberIsolation: awkward, "a key = \"x\"": awkward}})

	require.NoError(t, err)
	var decoded struct {
		Worktrees struct {
			BasePath   string `toml:"base_path"`
			BaseBranch string `toml:"base_branch"`
		} `toml:"worktrees"`
		Env struct {
			File []struct {
				Target   string `toml:"target"`
				Template string `toml:"template"`
			} `toml:"file"`
		} `toml:"env"`
		Hooks struct {
			OnCreate []any `toml:"on_create"`
			OnClean  []any `toml:"on_clean"`
		} `toml:"hooks"`
		Wizard struct {
			Remembered map[string]string `toml:"remembered"`
		} `toml:"wizard"`
	}
	_, err = toml.Decode(readFile(t, filepath.Join(stateDir, "config.toml")), &decoded)
	require.NoError(t, err)
	assert.Equal(t, "../"+awkward, decoded.Worktrees.BasePath)
	assert.Equal(t, "feat/"+awkward, decoded.Worktrees.BaseBranch)
	assert.Equal(t, awkward+"/.env", decoded.Env.File[0].Target)
	assert.Equal(t, awkward+"/.env.example", decoded.Env.File[0].Template)
	assert.Equal(t, []any{awkward, map[string]any{"cmd": awkward, "cwd": awkward, "continue_on_error": true}}, decoded.Hooks.OnCreate)
	assert.Equal(t, []any{awkward}, decoded.Hooks.OnClean)
	assert.Equal(t, map[string]string{project.RememberIsolation: awkward, "a key = \"x\"": awkward}, decoded.Wizard.Remembered)
}

func TestRememberingKeepsTheCleanHooks(t *testing.T) {
	stateDir := stateWithConfig(t, golden(t, "full.before.toml"))

	_, err := Remember(t.Context(), RememberParams{StateDir: stateDir, Answers: map[string]string{project.RememberIsolation: "verbatim"}})

	require.NoError(t, err)
	assert.Contains(t, readFile(t, filepath.Join(stateDir, "config.toml")), "on_clean = [\n  \"docker compose down\",\n]")
}
