package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runlist"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfRunList(t *testing.T) {
	got := runList(t, world{}, "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "list_help", got.Stdout)
}

func TestRunListDrawsTheProfilesThenTheJobsAsTheWorktreeRunsThem(t *testing.T) {
	got := runList(t, world{})

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "list_text", got.Stdout)
}

func TestAWorktreeNamedIsTheOneListedItsPortsItsOwn(t *testing.T) {
	got := runList(t, world{}, "main")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "list_main", got.Stdout)
}

func TestARepositoryWithNoRunTomlIsFollowedUpByRunInit(t *testing.T) {
	got := runList(t, world{Bare: true})

	assert.Equal(t, uicli.ExitPrecondition, got.Code)
	formtest.Golden(t, "list_not_initialized", got.Stderr)
}

func TestTheJSONOfRunListIsTheV2EnvelopeOfARead(t *testing.T) {
	got := runList(t, world{}, "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "envelope_list", got.Stdout)
	doc := formtest.Decoded[struct{}](t, got.Stdout)
	assert.Equal(t, runlist.Name, doc.Command)
}
