package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfEnvSaysWhatItReconcilesAndWhatItWithholds(t *testing.T) {
	got := runEnv(t, newWorld(), "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "env_help", got.Stdout)
}

func TestADryRunShowsEachChangeAndWhatIsLeft(t *testing.T) {
	got := runEnv(t, newWorld(), "--all", "--dry-run")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "dryrun", got.Stdout)
}

func TestAnUnattendedRunReconcilesTheWorktreesOutOfStep(t *testing.T) {
	w := newWorld()

	got := runEnv(t, w, "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "applied", got.Stdout)
	content, _ := w.store.File(pathOf(billing) + "/" + webEnv)
	assert.Equal(t, "PORT=3020\n", content)
}

func TestAJobUpOnTheOldValuesIsToRestart(t *testing.T) {
	w := newWorld()
	w.jobs = []run.Job{{Name: "web", WorkDir: pathOf(billing), Up: true}}

	got := runEnv(t, w, billing, "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "restart", got.Stdout)
}

func TestTheJSONEnvelopeWithholdsTheValuesUnlessShown(t *testing.T) {
	hidden := runEnv(t, newWorld(), billing, "--yes", "--output", "json")
	shown := runEnv(t, newWorld(), billing, "--yes", "--output", "json", "--show-values")

	require.Equal(t, uicli.ExitOK, hidden.Code, hidden.Stderr)
	formtest.Golden(t, "json", hidden.Stdout)
	assert.NotContains(t, hidden.Stdout, "s3cret")
	assert.Contains(t, shown.Stdout, "s3cret")
}

func TestARequestTheWorktreesCannotTakeIsRefusedWithItsExit(t *testing.T) {
	got := runEnv(t, newWorld(), "main", "--isolation", "verbatim", "--yes")

	assert.Equal(t, uicli.ExitUsage, got.Code)
	formtest.Golden(t, "main_isolated", got.Stderr)
}
