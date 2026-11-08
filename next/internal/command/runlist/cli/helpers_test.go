package cli

import (
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/runlist"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

func runList(t *testing.T, w world, words ...string) formtest.Printed {
	t.Helper()
	return formtest.Run(t.Context(), func(streams *output.Streams) *cobra.Command {
		parent := &cobra.Command{Use: "run"}
		parent.AddCommand(New(streams, w.env(), w.open, formtest.CommandLine))
		return parent
	}, append([]string{"run", "list"}, words...)...)
}

// runListAt runs run list at a terminal on which the user types nothing.
func runListAt(t *testing.T, w world, words ...string) formtest.Printed {
	t.Helper()
	return formtest.Run(t.Context(), func(streams *output.Streams) *cobra.Command {
		streams.In = strings.NewReader("")
		streams.SetStdinTerminal(true)
		streams.Out.SetTerminal(true)
		streams.Err.SetTerminal(true)
		parent := &cobra.Command{Use: "run"}
		parent.AddCommand(New(streams, w.env(), w.open, formtest.CommandLine))
		return parent
	}, append([]string{"run", "list"}, words...)...)
}

// A driver plays the picker over what w reads of feat/a.
type driver struct {
	*formtest.Driver
	t *testing.T
	p *picker
}

func picking(t *testing.T, w world) *driver {
	t.Helper()
	app, err := w.open(t.Context(), mainPath)
	require.NoError(t, err)
	read, err := app.Run(t.Context(), runlist.Request{}.From(w.env().Caller()), domain.RunOptions{})
	require.NoError(t, err)
	p := &picker{subject: read.Items[0].Subject, detail: *read.Items[0].Detail}
	return &driver{Driver: formtest.Start(form.New(t.Context(), p.script()), tea.WindowSizeMsg{Width: 100, Height: 30}), t: t, p: p}
}

func (d *driver) looksLike(name string) {
	d.t.Helper()
	formtest.Golden(d.t, "pick_"+name, d.Screen())
}
