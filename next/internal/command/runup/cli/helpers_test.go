package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/LucasPcq/wtm/next/internal/ui/runview"

	"github.com/spf13/cobra"
)

func runUp(t *testing.T, w world, words ...string) formtest.Printed {
	t.Helper()
	return formtest.Run(t.Context(), func(streams *output.Streams) *cobra.Command {
		parent := &cobra.Command{Use: "run"}
		parent.AddCommand(New(streams, w.env(), w.open, formtest.CommandLine, runview.View{Daemon: w.daemon}))
		return parent
	}, append([]string{"run", "up"}, words...)...)
}

// runUpAt runs run up at a terminal on which the user types keys.
func runUpAt(t *testing.T, w world, keys string, words ...string) formtest.Printed {
	t.Helper()
	return runUpIn(t.Context(), w, keys, words...)
}

// runUpIn runs run up at a terminal, as runUpAt, under ctx.
func runUpIn(ctx context.Context, w world, keys string, words ...string) formtest.Printed {
	return formtest.Run(ctx, func(streams *output.Streams) *cobra.Command {
		streams.In = strings.NewReader(keys)
		streams.SetStdinTerminal(true)
		streams.Err.SetTerminal(true)
		parent := &cobra.Command{Use: "run"}
		parent.AddCommand(New(streams, w.env(), w.open, formtest.CommandLine, runview.View{Daemon: w.daemon}))
		return parent
	}, append([]string{"run", "up"}, words...)...)
}

// viewOpened reports whether a run drew the live view: it takes the alternate screen.
func viewOpened(printed formtest.Printed) bool {
	return strings.Contains(printed.Stderr, altScreen)
}
