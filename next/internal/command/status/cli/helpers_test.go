package cli

import (
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/status"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

type printed struct {
	Stdout string
	Stderr string
	Code   int
}

// runStatus runs wtm status with words over w, off a terminal.
func runStatus(t *testing.T, w world, words ...string) printed {
	t.Helper()
	return runOn(t, func(*output.Streams) {}, w, words...)
}

// runOn runs wtm status with words over w on streams set up by setup, its clock stopped at clock.
func runOn(t *testing.T, setup func(*output.Streams), w world, words ...string) printed {
	t.Helper()
	was := now
	now = func() time.Time { return clock }
	t.Cleanup(func() { now = was })

	streams, _, stdout, stderr := output.Test()
	setup(streams)
	root := uicli.NewRoot("1.2.3", streams, []*cobra.Command{New(streams, w.env(), w.open, formtest.CommandLine)})
	code := uicli.Execute(t.Context(), root, append([]string{status.Name}, words...))
	return printed{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}
}

// terminal makes every stream a terminal on which the user keeps pressing enter.
func terminal(streams *output.Streams) {
	streams.In = enters{}
	streams.SetStdinTerminal(true)
	streams.Err.SetTerminal(true)
	streams.Out.SetTerminal(true)
}
