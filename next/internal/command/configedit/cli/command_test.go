package cli

import (
	"strings"
	"testing"

	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"

	"github.com/stretchr/testify/assert"
)

func TestTheHelpOfConfigEdit(t *testing.T) {
	got := runEdit(t, world{Config: acme}, "--help")

	assert.Equal(t, uicli.ExitOK, got.Code)
	formtest.Golden(t, "edit_help", got.Stdout)
}

func TestConfigEditSaysWhatTheEditorLeft(t *testing.T) {
	cases := map[string]struct {
		edited string
		code   int
		output func(formtest.Printed) string
	}{
		"valid":    {strings.Replace(acme, "../.trees", "../wt", 1), uicli.ExitOK, func(p formtest.Printed) string { return p.Stdout }},
		"invalid":  {strings.Replace(acme, `strategy = "example"`, `strategy = "copy"`, 1), uicli.ExitError, func(p formtest.Printed) string { return p.Stdout + p.Stderr }},
		"no_parse": {acme + "x = [\n", uicli.ExitError, func(p formtest.Printed) string { return p.Stderr }},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			got := runEdit(t, world{Config: acme, Edited: c.edited})

			assert.Equal(t, c.code, got.Code, got.Stderr)
			formtest.Golden(t, "edit_"+name, c.output(got))
		})
	}
}

func TestConfigEditInJSONIsTheReadEnvelope(t *testing.T) {
	got := runEdit(t, world{Config: acme, Edited: strings.Replace(acme, `strategy = "example"`, `strategy = "copy"`, 1)}, "--output", "json")

	formtest.Golden(t, "envelope_invalid", got.Stdout)
}
