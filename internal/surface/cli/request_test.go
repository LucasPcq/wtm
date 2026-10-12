package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/kernel"
)

func TestEachKindOfFieldReachesTheRequestFromItsFlag(t *testing.T) {
	w := newWorld(t, worldParams{})
	cmd, b := w.binding(t, Positional("branches"))
	require.NoError(t, cmd.ParseFlags([]string{
		"--isolation", verbatim, "--fast-forward", "--env", "PORT=3000", "--env", "HOST=", "--target-base", "main",
	}))

	request, invalid := b.request(cmd, []string{"feat/a", "feat/b"})

	require.Nil(t, invalid)
	assert.JSONEq(t, `{
		"branches": ["feat/a", "feat/b"],
		"isolation": "verbatim",
		"fast_forward": true,
		"env": {"PORT": "3000", "HOST": ""},
		"target": {"base": "main"}
	}`, string(request))
}

func TestAFlagNotPassedStaysOutOfTheRequest(t *testing.T) {
	w := newWorld(t, worldParams{})
	cmd, b := w.binding(t)
	require.NoError(t, cmd.ParseFlags([]string{"--fast-forward=false"}))

	request, invalid := b.request(cmd, nil)

	require.Nil(t, invalid)
	assert.JSONEq(t, `{"fast_forward": false}`, string(request))
}

func TestADecisionWithNoKeyIsRefusedOnItsField(t *testing.T) {
	w := newWorld(t, worldParams{})
	cmd, b := w.binding(t)
	require.NoError(t, cmd.ParseFlags([]string{"--env", "3000"}))

	_, invalid := b.request(cmd, nil)

	require.NotNil(t, invalid)
	assert.Equal(t, []kernel.FieldError{{Path: "env", Code: kernel.CodeInvalid, Params: kernel.Params{kernel.ParamValue: "3000"}}}, fieldsOf(t, invalid))
}

func TestAOneValuePositionalTakesOneArgument(t *testing.T) {
	w := newWorld(t, worldParams{})
	cmd, b := w.binding(t, Positional("from"))

	one, oneInvalid := b.request(cmd, []string{"main"})
	_, twoInvalid := b.request(cmd, []string{"main", "dev"})

	require.Nil(t, oneInvalid)
	assert.JSONEq(t, `{"from": "main"}`, string(one))
	assert.Equal(t, []kernel.FieldError{{Path: "from", Code: kernel.CodeTooMany, Params: kernel.Params{kernel.ParamLimit: "1"}}}, fieldsOf(t, twoInvalid))
}

func TestAnArgumentWithNoPositionalToTakeItIsInvalid(t *testing.T) {
	w := newWorld(t, worldParams{})
	cmd, b := w.binding(t)

	_, invalid := b.request(cmd, []string{"feat/a"})

	require.NotNil(t, invalid)
	assert.EqualError(t, invalid.Base().Cause, `unexpected argument "feat/a": the command takes none`)
}
