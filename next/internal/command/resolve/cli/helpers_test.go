package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/resolve"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// runResolve runs wtm resolve with words over w, off a terminal.
func runResolve(t *testing.T, w world, words ...string) formtest.Printed {
	t.Helper()
	return runOn(t, func(*output.Streams) {}, w, words...)
}

// runOn runs wtm resolve with words over w on streams set up by setup.
func runOn(t *testing.T, setup func(*output.Streams), w world, words ...string) formtest.Printed {
	t.Helper()
	return formtest.Run(t.Context(), func(streams *output.Streams) *cobra.Command {
		setup(streams)
		return New(streams, w.env(), w.open, formtest.CommandLine)
	}, append([]string{resolve.Name}, words...)...)
}

// captured is a terminal whose stdout a shell captures, `$(wtm resolve)`, on which the user keeps pressing enter.
func captured(streams *output.Streams) {
	streams.In = formtest.Enters{}
	streams.SetStdinTerminal(true)
	streams.Err.SetTerminal(true)
}
