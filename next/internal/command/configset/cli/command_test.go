package cli

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/configset"
	"github.com/LucasPcq/wtm/next/internal/service/settings"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfConfigSet(t *testing.T) {
	got := runSet(t, world{}, "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "set_help", got.Stdout)
}

func TestConfigSetPreviewsThenWrites(t *testing.T) {
	preview := runSet(t, world{Config: acme}, "--base-path", "../wt", "--on-clean=", "--dry-run")
	require.Equal(t, uicli.ExitOK, preview.Code, preview.Stderr)
	formtest.Golden(t, "set_dry_run", preview.Stdout)

	applied := runSet(t, world{Config: acme}, "--base-path", "../wt", "--on-clean=", "--yes")

	require.Equal(t, uicli.ExitOK, applied.Code, applied.Stderr)
	formtest.Golden(t, "set_applied", applied.Stdout)
	assert.Contains(t, written(t), `base_path = "../wt"`)
	assert.NotContains(t, written(t), "docker compose down")
}

func TestConfigSetRefusesAValueOnItsFlag(t *testing.T) {
	got := runSet(t, world{Config: acme}, "--base-path", "/tmp/wt", "--yes")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	formtest.Golden(t, "set_invalid", got.Stderr)
	assert.Equal(t, acme, written(t))
}

func TestConfigSetInJSONIsTheEnvelopeWithItsUndo(t *testing.T) {
	got := runSet(t, world{Config: acme}, "--env-strategy", "main", "--yes", "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "envelope_applied", got.Stdout)
}

func TestConfigSetRefusesNoConfig(t *testing.T) {
	got := runSet(t, world{}, "--base-path", "../wt", "--yes")

	assert.Equal(t, uicli.ExitConfigNotFound, got.Code)
	formtest.Golden(t, "set_no_config", got.Stderr)
}

func TestAFlagGivenInATerminalChangesItAloneWithoutAskingTheOthers(t *testing.T) {
	d := wizard(t, world{Config: acme}, configset.Request{Given: settings.Given{BasePath: "../wt"}})

	screen := d.Screen()
	assert.Contains(t, screen, "Yes, write config.toml")
	assert.Contains(t, screen, "../wt · --base-path")
	assert.Contains(t, screen, "main · kept")
}

func TestNoFlagInATerminalAsksEverySetting(t *testing.T) {
	d := wizard(t, world{Config: acme}, configset.Request{})

	assert.Contains(t, d.Screen(), "Step 1/9 • Worktrees folder")
	assert.NotContains(t, d.Screen(), "kept")
}

// a strategy given over a config.toml that copies no .env file was written with none of the repository's
func TestAProblemTheFlagCreatesOnAnotherSettingNamesItsFlag(t *testing.T) {
	got := runSet(t, noEnv, "--env-strategy", "main", "--yes")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	assert.Contains(t, got.Stderr, "--env-file")
	assert.Equal(t, noEnv.Config, written(t))
}

func TestAKeptStepTheFlagMakesInvalidIsAsked(t *testing.T) {
	d := wizard(t, noEnv, configset.Request{Given: settings.Given{EnvStrategy: "main"}})

	assert.Contains(t, d.Screen(), "Step 4/9")
	assert.NotContains(t, d.Screen(), "Yes, write config.toml")
}

func TestAProblemOnDiskOfAKeptStepIsNotAsked(t *testing.T) {
	d := wizard(t, world{Config: strings.Replace(acme, `strategy = "example"`, `strategy = "copy"`, 1)}, configset.Request{Given: settings.Given{BasePath: "../wt"}})

	assert.Contains(t, d.Screen(), "Yes, write config.toml")
}

func TestTheFileAChangeIsMadeOnAloneKeepsNoSetting(t *testing.T) {
	d := wizard(t, world{Config: acme}, configset.Request{Replaces: "d1"})

	assert.Contains(t, d.Screen(), "Step 1/9 • Worktrees folder")
}
