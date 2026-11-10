package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSavedIsTheConfigAsWrittenWithNoDefault(t *testing.T) {
	stateDir := stateWithConfig(t, golden(t, "sparse.before.toml"))

	saved, err := Project{StateDir: stateDir}.Saved()

	require.NoError(t, err)
	assert.Empty(t, saved.Config.Worktrees.BaseBranch)
	assert.Empty(t, saved.Config.Env.Strategy)
	assert.Equal(t, digest([]byte(golden(t, "sparse.before.toml"))), saved.Digest)
}

func TestSavedWithoutAConfigIsNotFound(t *testing.T) {
	_, err := Project{StateDir: t.TempDir()}.Saved()

	require.ErrorIs(t, err, project.ErrConfigNotFound)
}

func TestSavedRefusesAConfigThatDoesNotValidate(t *testing.T) {
	_, err := Project{StateDir: stateWithConfig(t, "[env]\nstrategy = \"copy\"\n")}.Saved()

	_, invalid := errors.AsType[*project.InvalidConfigError](err)
	assert.True(t, invalid, "Saved = %v, want a *project.InvalidConfigError", err)
}

func TestSaveCreatesTheConfigAndItsSchema(t *testing.T) {
	stateDir := t.TempDir()

	written, err := Project{StateDir: stateDir}.Save(t.Context(), project.Rewrite{Config: *seeded()})

	require.NoError(t, err)
	assert.True(t, written.Changed)
	assert.Equal(t, golden(t, "saved.toml"), readFile(t, filepath.Join(stateDir, "config.toml")))
	assert.FileExists(t, filepath.Join(stateDir, "schemas", "project.schema.json"))
}

func TestWhatSaveWritesIsSavedBack(t *testing.T) {
	config := project.Config{
		Worktrees: project.Worktrees{BasePath: "../acme.trees", BaseBranch: "main"},
		OnCreate:  []project.Hook{{Cmd: "pnpm install"}, {Cmd: "pnpm db:generate", Cwd: "packages/db", ContinueOnError: true}},
		OnClean:   []project.Hook{},
	}
	stateDir := t.TempDir()
	_, err := Project{StateDir: stateDir}.Save(t.Context(), project.Rewrite{Config: config})
	require.NoError(t, err)

	saved, err := Project{StateDir: stateDir}.Saved()

	require.NoError(t, err)
	assert.Equal(t, config.Worktrees, saved.Config.Worktrees)
	assert.Equal(t, project.Env{Files: []project.EnvFile{}}, saved.Config.Env, "no .env copied stays none: no strategy nor file")
	assert.Equal(t, config.OnCreate, saved.Config.OnCreate)
	assert.Empty(t, saved.Config.OnClean)
}

func TestSaveKeepsTheRememberedAnswersItIsGiven(t *testing.T) {
	stateDir := stateWithConfig(t, golden(t, "full.after.toml"))
	saved, err := Project{StateDir: stateDir}.Saved()
	require.NoError(t, err)

	_, err = Project{StateDir: stateDir}.Save(t.Context(), project.Rewrite{Config: saved.Config, Replaces: saved.Digest})

	require.NoError(t, err)
	again, err := Project{StateDir: stateDir}.Saved()
	require.NoError(t, err)
	assert.Equal(t, saved.Config.Remembered, again.Config.Remembered)
}

func TestSaveRefusesAConfigThatChangedOnDisk(t *testing.T) {
	cases := map[string]struct {
		before string
		digest string
	}{
		"edited":  {before: minimalConfig, digest: digest([]byte("[worktrees]\n"))},
		"created": {before: minimalConfig, digest: ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			stateDir := stateWithConfig(t, c.before)

			_, err := Project{StateDir: stateDir}.Save(t.Context(), project.Rewrite{Config: *seeded(), Replaces: c.digest})

			require.ErrorIs(t, err, project.ErrConfigChanged)
			assert.Equal(t, c.before, readFile(t, filepath.Join(stateDir, "config.toml")))
		})
	}
}

func TestSaveRefusesAConfigRemovedSinceItWasRead(t *testing.T) {
	stateDir := stateWithConfig(t, minimalConfig)
	saved, err := Project{StateDir: stateDir}.Saved()
	require.NoError(t, err)
	require.NoError(t, os.Remove(filepath.Join(stateDir, "config.toml")))

	_, err = Project{StateDir: stateDir}.Save(t.Context(), project.Rewrite{Config: *seeded(), Replaces: saved.Digest})

	require.ErrorIs(t, err, project.ErrConfigChanged)
}

func TestSaveNeverRewritesAConfigAtANewerSchema(t *testing.T) {
	stateDir := stateWithConfig(t, newerSchema)

	_, err := Project{StateDir: stateDir}.Save(t.Context(), project.Rewrite{Config: *seeded(), Replaces: digest([]byte(newerSchema))})

	_, newer := errors.AsType[*domain.SchemaNewer](err)
	assert.True(t, newer, "Save = %v, want a *domain.SchemaNewer", err)
}
