package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfDaemonStopSaysWhatStopsAndWhatRunsOn(t *testing.T) {
	got := runStop(t, newWorld(), "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "stop_help", got.Stdout)
}

func TestADryRunSaysWhatStopsWhatRunsOnAndWhatIsLetGo(t *testing.T) {
	got := runStop(t, newWorld(), "--dry-run")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "dryrun", got.Stdout)
}

func TestAStopSaysTheDaemonStopped(t *testing.T) {
	got := runStop(t, newWorld(), "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "stopped", got.Stdout)
}

// L1: legacy refused --output json without --yes and wrote nothing on stdout.
func TestAStopInJSONIsItsEnvelope(t *testing.T) {
	got := runStop(t, newWorld(), "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "envelope_stopped", got.Stdout)
}

func TestWithNoDaemonRunningAStopSaysSo(t *testing.T) {
	w := newWorld()
	w.daemon.status = run.Daemon{}

	got := runStop(t, w, "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "not_running", got.Stdout)
}

func TestADaemonThatDoesNotStopFails(t *testing.T) {
	w := newWorld()
	w.daemon.fail = errStuck

	got := runStop(t, w, "--yes")

	assert.Equal(t, uicli.ExitError, got.Code)
	formtest.Golden(t, "failed", got.Stdout)
}
