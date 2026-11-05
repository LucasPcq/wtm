package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runstart"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/LucasPcq/wtm/next/internal/ui/runview"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func runStart(t *testing.T, w world, words ...string) formtest.Printed {
	t.Helper()
	return formtest.Run(t.Context(), func(streams *output.Streams) *cobra.Command {
		parent := &cobra.Command{Use: "run"}
		parent.AddCommand(New(streams, w.env(), w.open, formtest.CommandLine, runview.View{Daemon: w.daemon}))
		return parent
	}, append([]string{"run", "start"}, words...)...)
}

// runStartAt runs run start at a terminal on which the user types keys.
func runStartAt(t *testing.T, w world, keys string, words ...string) formtest.Printed {
	t.Helper()
	return runStartIn(t.Context(), w, keys, words...)
}

// runStartIn runs run start at a terminal, as runStartAt, under ctx.
func runStartIn(ctx context.Context, w world, keys string, words ...string) formtest.Printed {
	return formtest.Run(ctx, func(streams *output.Streams) *cobra.Command {
		streams.In = strings.NewReader(keys)
		streams.SetStdinTerminal(true)
		streams.Err.SetTerminal(true)
		parent := &cobra.Command{Use: "run"}
		parent.AddCommand(New(streams, w.env(), w.open, formtest.CommandLine, runview.View{Daemon: w.daemon}))
		return parent
	}, append([]string{"run", "start"}, words...)...)
}

// viewOpened reports whether a run drew the live view: it takes the alternate screen.
func viewOpened(printed formtest.Printed) bool {
	return strings.Contains(printed.Stderr, altScreen)
}

// wizard opens the wizard of req over w, its first step drawn.
func wizard(t *testing.T, w world, req runstart.Request) *formtest.Driver {
	t.Helper()
	app, err := w.open(t.Context(), mainPath)
	require.NoError(t, err)
	bound := newForm(nil)
	bound.Bind(&cobra.Command{}, &uicli.Run{CommandLine: formtest.CommandLine})
	s := bound.Session(t.Context(), app, req, domain.RunOptions{})
	return formtest.Start(form.New(t.Context(), s.Script()), tea.WindowSizeMsg{Width: 100, Height: 40})
}
