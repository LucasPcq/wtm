package cli

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

// runStatus runs wtm status with words over w, off a terminal.
func runStatus(t *testing.T, w world, words ...string) formtest.Printed {
	t.Helper()
	return runOn(t, func(*output.Streams) {}, w, words...)
}

// runOn runs wtm status with words over w on streams set up by setup, its clock stopped at clock.
func runOn(t *testing.T, setup func(*output.Streams), w world, words ...string) formtest.Printed {
	t.Helper()
	was := now
	now = func() time.Time { return clock }
	t.Cleanup(func() { now = was })

	return formtest.Run(t.Context(), func(streams *output.Streams) *cobra.Command {
		setup(streams)
		return New(streams, w.env(), w.open, formtest.CommandLine)
	}, append([]string{status.Name}, words...)...)
}

// terminal makes every stream a terminal on which the user keeps pressing enter.
func terminal(streams *output.Streams) {
	streams.In = formtest.Enters{}
	streams.SetStdinTerminal(true)
	streams.Err.SetTerminal(true)
	streams.Out.SetTerminal(true)
}
