package cli

import (
	"strings"
	"testing"

	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTheHelpOfConfigShow(t *testing.T) {
	got := runShow(t, world{}, "--help")

	require.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "show_help", got.Stdout)
}

func TestConfigShowDrawsTheDocument(t *testing.T) {
	cases := map[string]world{
		"document": {Config: acme},
		"invalid":  {Config: strings.Replace(acme, `strategy = "example"`, `strategy = "copy"`, 1)},
		"newer":    {Config: strings.Replace(acme, "schema = 1", "schema = 9", 1)},
	}
	for name, w := range cases {
		t.Run(name, func(t *testing.T) {
			got := runShow(t, w)

			require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
			formtest.Golden(t, "show_"+name, got.Stdout)
		})
	}
}

func TestConfigShowInJSONIsTheReadEnvelope(t *testing.T) {
	got := runShow(t, world{Config: acme}, "--output", "json")

	require.Equal(t, uicli.ExitOK, got.Code, got.Stderr)
	formtest.Golden(t, "envelope_document", got.Stdout)
}

func TestConfigShowRefusesNoConfigAndOneThatDoesNotParse(t *testing.T) {
	cases := map[string]struct {
		world world
		code  int
	}{
		"no_config": {world{}, uicli.ExitConfigNotFound},
		"no_parse":  {world{Config: strings.Replace(acme, `base_branch = "main"`, `base_branch =`, 1)}, uicli.ExitError},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := runShow(t, c.world)

			assert.Equal(t, c.code, got.Code)
			formtest.Golden(t, "show_"+name, got.Stderr)
		})
	}
}
