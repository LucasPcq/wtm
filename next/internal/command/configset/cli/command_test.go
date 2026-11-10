package cli

import (
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

	assert.NotContains(t, d.Screen(), "Yes, write config.toml")
	assert.NotContains(t, d.Screen(), "kept")
}
