package config

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAHookIsAStringOrATable(t *testing.T) {
	content := "[hooks]\non_create = [\n  \"pnpm install\",\n  { cmd = \"pnpm db:migrate\", cwd = \"apps/api\", continue_on_error = true },\n]\n"

	cfg, _, err := loadState(t, stateWithConfig(t, content))

	require.NoError(t, err)
	assert.Equal(t, []project.Hook{{Cmd: "pnpm install"}, {Cmd: "pnpm db:migrate", Cwd: "apps/api", ContinueOnError: true}}, cfg.OnCreate)
}

func TestTheCleanHooksAreReadButNotKept(t *testing.T) {
	cfg, _, err := loadState(t, stateWithConfig(t, "[hooks]\non_create = []\non_clean = [\"docker compose down\", { cmd = \"rm -rf tmp\", cwd = \".\" }]\n"))

	require.NoError(t, err)
	assert.Empty(t, cfg.OnCreate)
}

func TestNoHooksIsNoHook(t *testing.T) {
	cfg, _, err := loadState(t, stateWithConfig(t, "[hooks]\non_create = []\n"))

	require.NoError(t, err)
	assert.Empty(t, cfg.OnCreate)
}

func TestAHookThatIsNeitherAStringNorATableIsRefused(t *testing.T) {
	_, _, err := loadState(t, stateWithConfig(t, "[hooks]\non_create = [42]\n"))

	assert.ErrorContains(t, err, "on_create: hook[0]: hook must be a string or {cmd, cwd}, got int64")
}

func TestAHookTableWithoutACommandIsRefused(t *testing.T) {
	_, _, err := loadState(t, stateWithConfig(t, "[hooks]\non_create = [{ cwd = \"/tmp\" }]\n"))

	assert.ErrorContains(t, err, `on_create: hook[0]: hook table requires a string "cmd" field`)
}

func TestAnUnknownKeyInAHookTableIsTolerated(t *testing.T) {
	cfg, _, err := loadState(t, stateWithConfig(t, "[hooks]\non_create = [{ cmd = \"make\", shell = \"zsh\" }]\n"))

	require.NoError(t, err)
	assert.Equal(t, []project.Hook{{Cmd: "make"}}, cfg.OnCreate)
}
