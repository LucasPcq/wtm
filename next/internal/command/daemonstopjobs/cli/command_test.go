package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstopjobs"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfStopJobsSaysTheDaemonStaysUp(t *testing.T) {
	got := runStopJobs(t, newWorld(), "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "stop_jobs_help", got.Stdout)
}

func TestADryRunSaysWhatStopsInEachWorktreeOfEachRepository(t *testing.T) {
	got := runStopJobs(t, newWorld(), "--dry-run")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "dryrun", got.Stdout)
}

func TestStopJobsCountsTheWorktreesStopped(t *testing.T) {
	got := runStopJobs(t, newWorld(), "--yes")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "stopped", got.Stdout)
}

func TestAStackThatDidNotStopIsNamedAndTheStopOfferedAgain(t *testing.T) {
	w := newWorld()
	w.daemon.refuse = map[string]error{"mail": errStack}

	got := runStopJobs(t, w, "--repo", "/src/acme", "--yes")

	assert.Equal(t, uicli.ExitError, got.Code)
	formtest.Golden(t, "failed", got.Stdout)
}

func TestStopJobsInJSONIsItsEnvelope(t *testing.T) {
	got := runStopJobs(t, newWorld(), "--yes", "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "envelope_stopped", got.Stdout)
}

func TestTheStopAgainIsSpelledWithItsRepository(t *testing.T) {
	again := domain.FollowUp{Command: daemonstopjobs.Name, Answers: []domain.Answered{{Path: domain.PathRepo, Values: []string{"/src/acme"}}}}

	assert.Equal(t, []string{"wtm-next", "daemon", "stop-jobs", "--repo", "/src/acme"}, Line(again))
}
