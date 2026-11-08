package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfDaemonRestartSaysWhatComesBack(t *testing.T) {
	got := runRestart(t, newWorld(), "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "restart_help", got.Stdout)
}

func TestADryRunSaysWhatComesBackWhatIsTakenOverAndWhatIsNot(t *testing.T) {
	got := runRestart(t, newWorld(), "--dry-run")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "dryrun", got.Stdout)
}

// N2: next's restart printed one unframed line.
func TestARestartSaysHowManyJobsCameBack(t *testing.T) {
	got := runRestart(t, newWorld(), "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "restarted", got.Stdout)
}

// N2: next's restart had no --output json.
func TestARestartInJSONIsItsEnvelope(t *testing.T) {
	got := runRestart(t, newWorld(), "--yes", "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "envelope_restarted", got.Stdout)
}

func TestAJobThatDidNotComeBackIsNamed(t *testing.T) {
	w := newWorld()
	w.daemon.refuse = map[string]error{"web": errPort}

	got := runRestart(t, w, "--yes")

	assert.Equal(t, uicli.ExitError, got.Code)
	formtest.Golden(t, "not_back", got.Stdout)
}

func TestWithNoDaemonRunningARestartStartsOne(t *testing.T) {
	w := newWorld()
	w.daemon.status, w.daemon.records = run.Daemon{}, nil

	got := runRestart(t, w)

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "started", got.Stdout)
}

func TestARestartFollowUpIsSpelledAsItsCommandLine(t *testing.T) {
	assert.Equal(t, []string{"wtm-next", "daemon", "restart"}, Line(domain.FollowUp{Command: domain.CommandDaemonRestart}))
}
