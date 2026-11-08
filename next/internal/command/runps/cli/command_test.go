package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runps"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfRunPs(t *testing.T) {
	got := runPs(t, world{}, "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "ps_help", got.Stdout)
}

func TestRunPsDrawsAGroupPerWorktreeCrashesFirstThenTheSharedServicesWithTheirHolders(t *testing.T) {
	got := runPs(t, world{})

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "ps_text", got.Stdout)
}

func TestOverSeveralRepositoriesEachTitleNamesItsRepository(t *testing.T) {
	got := runPs(t, world{Other: true})

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "ps_repositories", got.Stdout)
}

func TestNothingRunningIsSaid(t *testing.T) {
	got := runPs(t, world{Nothing: true})

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "ps_nothing", got.Stdout)
}

func TestADaemonThatCannotTellIsAWarning(t *testing.T) {
	got := runPs(t, world{JobsErr: run.ErrUnknown})

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "ps_jobs_unknown", got.Stdout)
}

func TestARepositoryNamedOutsideAnyIsRefused(t *testing.T) {
	got := runPs(t, world{}, "--repo", "/nowhere")

	assert.Equal(t, uicli.ExitNotGitRepo, got.Code)
	formtest.Golden(t, "ps_not_a_repository", got.Stderr)
}

func TestTheJSONOfRunPsIsTheV2EnvelopeOfARead(t *testing.T) {
	got := runPs(t, world{}, "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "envelope_ps", got.Stdout)
	doc := formtest.Decoded[struct{}](t, got.Stdout)
	assert.Equal(t, runps.Name, doc.Command)
	assert.Empty(t, doc.Op)
}
