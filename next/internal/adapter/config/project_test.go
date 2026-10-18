package config

import (
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAFullConfigLoadsEverySection(t *testing.T) {
	cfg, _, err := loadState(t, stateWithConfig(t, fullConfig))

	require.NoError(t, err)
	assert.Equal(t, project.Worktrees{BasePath: ".trees", BaseBranch: "develop"}, cfg.Worktrees)
	assert.Equal(t, project.Env{Strategy: project.EnvMain, Files: []project.EnvFile{{Target: ".env"}, {Target: "apps/api/.env"}}}, cfg.Env)
	assert.Equal(t, []project.Hook{{Cmd: "pnpm install"}, {Cmd: "pnpm install", Cwd: "apps/api"}}, cfg.OnCreate)
}

func TestAnEnvFileKeepsItsPinnedTemplateAndItsLocalMark(t *testing.T) {
	content := "[env]\nstrategy = \"example\"\n\n[[env.file]]\ntarget = \".env\"\ntemplate = \".env.dist\"\n\n[[env.file]]\ntarget = \".env.local\"\nlocal = true\n"

	cfg, _, err := loadState(t, stateWithConfig(t, content))

	require.NoError(t, err)
	assert.Equal(t, []project.EnvFile{{Target: ".env", Template: ".env.dist"}, {Target: ".env.local", Local: true}}, cfg.Env.Files)
}

func TestAMinimalConfigTakesTheDefaults(t *testing.T) {
	cfg, _, err := loadState(t, stateWithConfig(t, minimalConfig))

	require.NoError(t, err)
	assert.Equal(t, project.DefaultBasePath, cfg.Worktrees.BasePath)
	assert.Equal(t, project.DefaultBaseBranch, cfg.Worktrees.BaseBranch)
	assert.Equal(t, project.DefaultEnvStrategy, cfg.Env.Strategy)
	assert.Empty(t, cfg.OnCreate)
}

func TestAMissingConfigIsNotFound(t *testing.T) {
	_, _, err := loadState(t, t.TempDir())

	assert.ErrorIs(t, err, project.ErrConfigNotFound)
}

func TestAConfigThatIsNotTOMLIsRefused(t *testing.T) {
	_, _, err := loadState(t, stateWithConfig(t, "[worktrees\nthis is not valid toml !!!\n"))

	var invalid *project.InvalidConfigError
	assert.ErrorAs(t, err, &invalid)
}

func TestAnUnknownKeyInConfigTomlIsRefused(t *testing.T) {
	_, _, err := loadState(t, stateWithConfig(t, "[worktrees]\nbase_paht = \"../.trees\"\n"))

	var invalid *project.InvalidConfigError
	assert.ErrorAs(t, err, &invalid)
	assert.ErrorContains(t, err, "unknown keys in")
	assert.ErrorContains(t, err, "worktrees.base_paht")
}

func TestTheRememberedAnswersAreRead(t *testing.T) {
	content := minimalConfig + "\n[wizard.remembered]\nenv_strategy = \"parent\"\nisolation = \"verbatim\"\nsource_update = \"ff\"\n"

	cfg, _, err := loadState(t, stateWithConfig(t, content))

	require.NoError(t, err)
	assert.Equal(t, map[string]string{project.RememberEnvStrategy: "parent", project.RememberIsolation: "verbatim", project.RememberSourceUpdate: "ff"}, cfg.Remembered)
}

func TestAConfigAtANewerSchemaIsStillRead(t *testing.T) {
	cfg, _, err := loadState(t, stateWithConfig(t, newerSchema+fullConfig))

	require.NoError(t, err)
	assert.Equal(t, "develop", cfg.Worktrees.BaseBranch)
}

func TestAnInvalidConfigIsRefusedSayingWhy(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    string
	}{
		{"unknown_env_strategy", "[env]\nstrategy = \"copy\"\n", "invalid env strategy: must be example, main, or parent"},
		{"env_file_without_target", "[[env.file]]\ntemplate = \".env.example\"\n", "env file entry must have a target"},
		{"duplicate_env_target", "[[env.file]]\ntarget = \".env\"\n\n[[env.file]]\ntarget = \".env\"\n", "duplicate env file target: .env"},
		{"foreign_template", "[[env.file]]\ntarget = \".env\"\ntemplate = \"other.example\"\n", "env file template must be a known template of its target: other.example is not a template of .env"},
		{"unknown_remembered_answer", "[wizard.remembered]\ndelete = \"force\"\n", `unknown remembered answer "delete" in [wizard.remembered] (expected one of env_strategy, isolation, source_update)`},
		{"invalid_remembered_value", "[wizard.remembered]\nisolation = \"keep\"\n", `invalid remembered isolation "keep" in [wizard.remembered] (expected one of isolated, verbatim)`},
		{"absoluteBasePath", "[worktrees]\nbase_path = \"/tmp/trees\"\n", "base_path must be relative to the repository: /tmp/trees"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := loadState(t, stateWithConfig(t, tc.content))

			var invalid *project.InvalidConfigError
			assert.ErrorAs(t, err, &invalid)
			assert.ErrorContains(t, err, tc.want)
		})
	}
}

func TestEveryConventionalTemplateOfATargetIsAccepted(t *testing.T) {
	cases := []struct {
		name   string
		suffix string
	}{
		{"example", ".example"},
		{"dist", ".dist"},
		{"sample", ".sample"},
		{"template", ".template"},
		{"tmpl", ".tmpl"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := loadState(t, stateWithConfig(t, "[[env.file]]\ntarget = \".env\"\ntemplate = \".env"+tc.suffix+"\"\n"))

			assert.NoError(t, err)
		})
	}
}

func TestTheGlobalConfigPathIsUnderTheUserConfigDirectory(t *testing.T) {
	path := GlobalPath()

	if path == "" {
		t.Skip("no user config directory on this machine")
	}
	assert.Equal(t, "config.toml", filepath.Base(path))
	assert.Contains(t, path, "wtm")
}
