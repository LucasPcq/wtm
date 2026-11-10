package cli

import (
	"path/filepath"
	"testing"

	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfRunInitNamesEachPartsFlag(t *testing.T) {
	got := runInit(t, newWorld(t), "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "init_help", got.Stdout)
}

func TestADryRunShowsThePlanItsProblemsToCheckAndTheLineThatAppliesIt(t *testing.T) {
	got := runInit(t, newWorld(t), "--yes", "--dry-run")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "dry_run", got.Stdout)
}

func TestAFirstRunInitUnattendedWritesItsFilesAndNamesRunUp(t *testing.T) {
	w := newWorld(t)

	got := runInit(t, w, "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "first_init", got.Stdout)
	assert.FileExists(t, filepath.Join(w.stateDir, "run.toml"))
}

// Legacy asked nothing without a terminal and wrote from detection; next does too, as every command.
func TestWithoutATerminalRunInitRunsUnattended(t *testing.T) {
	w := newWorld(t)

	got := runInit(t, w)

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	assert.FileExists(t, filepath.Join(w.stateDir, "run.toml"))
}

func TestAReInitThatChangesNothingSaysSo(t *testing.T) {
	w := newWorld(t)
	require.Equal(t, uicli.ExitOK, runInit(t, w, "--yes").Code)

	got := runInit(t, w, "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "unchanged", got.Stdout)
}

func TestTheJSONOfRunInitIsTheV2Envelope(t *testing.T) {
	got := runInit(t, newWorld(t), "--yes", "--output", "json", "--patch-compose=false")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "envelope_applied", got.Stdout)
}

func TestAJobRunInitCannotWriteIsRefusedByItsFlag(t *testing.T) {
	got := runInit(t, newWorld(t), "--yes", "--job", "nope")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	formtest.Golden(t, "unknown_job", got.Stderr)
}
