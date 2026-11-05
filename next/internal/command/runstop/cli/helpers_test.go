package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runstop"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func runStop(t *testing.T, w world, words ...string) formtest.Printed {
	t.Helper()
	return formtest.Run(t.Context(), func(streams *output.Streams) *cobra.Command {
		parent := &cobra.Command{Use: "run"}
		parent.AddCommand(New(streams, w.env(), w.open, formtest.CommandLine))
		return parent
	}, append([]string{"run", "stop"}, words...)...)
}

// wizard opens the wizard of req over w, its first step drawn.
func wizard(t *testing.T, w world, req runstop.Request) *formtest.Driver {
	t.Helper()
	app, err := w.open(t.Context(), mainPath)
	require.NoError(t, err)
	bound := newForm(nil)
	bound.Bind(&cobra.Command{}, &uicli.Run{CommandLine: formtest.CommandLine})
	s := bound.Session(t.Context(), app, req, domain.RunOptions{})
	return formtest.Start(form.New(t.Context(), s.Script()), tea.WindowSizeMsg{Width: 100, Height: 40})
}
