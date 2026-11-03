package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/spf13/cobra"
)

func runLogs(t *testing.T, w world, words ...string) formtest.Printed {
	t.Helper()
	return runLogsIn(t.Context(), w, words...)
}

func runLogsIn(ctx context.Context, w world, words ...string) formtest.Printed {
	return formtest.Run(ctx, func(streams *output.Streams) *cobra.Command {
		return parent(New(streams, w.env(), w.open, formtest.CommandLine, View{Daemon: w.daemon}))
	}, append([]string{"run", "logs"}, words...)...)
}

// runLogsAt runs run logs at a terminal on which the user types keys.
func runLogsAt(t *testing.T, w world, keys string, words ...string) formtest.Printed {
	t.Helper()
	return formtest.Run(t.Context(), func(streams *output.Streams) *cobra.Command {
		streams.In = strings.NewReader(keys)
		streams.SetStdinTerminal(true)
		streams.Out.SetTerminal(true)
		streams.Err.SetTerminal(true)
		return parent(New(streams, w.env(), w.open, formtest.CommandLine, View{Daemon: w.daemon}))
	}, append([]string{"run", "logs"}, words...)...)
}

func parent(logs *cobra.Command) *cobra.Command {
	cmd := &cobra.Command{Use: "run"}
	cmd.AddCommand(logs)
	return cmd
}

// viewOpened reports whether a run drew the live view: it takes the alternate screen.
func viewOpened(printed formtest.Printed) bool {
	return strings.Contains(printed.Stderr, altScreen)
}
