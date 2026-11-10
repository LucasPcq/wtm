package cli

import (
	"testing"

	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfEnvShowSaysWhatItReadsAndWithholds(t *testing.T) {
	got := runShow(t, "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "env_show_help", got.Stdout)
}

func TestTheWorktreeYouStandInIsReadKeysToCheckFirst(t *testing.T) {
	got := runShow(t)

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "show", got.Stdout)
	assert.NotContains(t, got.Stdout, "s3cret")
}

func TestShowValuesShowsEveryValueAndTheJSONTheTree(t *testing.T) {
	shown := runShow(t, "--show-values")
	json := runShow(t, "main", "--output", "json")

	assert.Contains(t, shown.Stdout, "s3cret")
	require.Equal(t, uicli.ExitOK, json.Code, json.Stderr)
	formtest.Golden(t, "json", json.Stdout)
}
