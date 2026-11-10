package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/initialize"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func runInit(t *testing.T, w world, words ...string) formtest.Printed {
	t.Helper()
	return formtest.Run(t.Context(), func(streams *output.Streams) *cobra.Command {
		return New(streams, w.env(), w.open, formtest.CommandLine)
	}, append([]string{"init"}, words...)...)
}

// A driver plays the wizard of a session.
type driver struct {
	*formtest.Driver
	t *testing.T
	s *session
}

func open(t *testing.T, w world) *driver {
	t.Helper()
	bound := newForm(nil)
	bound.Bind(&cobra.Command{}, &uicli.Run{CommandLine: formtest.CommandLine})
	s := bound.Session(t.Context(), w.usecase(), initialize.Request{}, domain.RunOptions{})
	return &driver{Driver: formtest.Start(form.New(t.Context(), s.Script()), tea.WindowSizeMsg{Width: 100, Height: 40}), t: t, s: s}
}

// looksLike compares the screen with the frame testdata/frame_<name>.golden holds.
func (d *driver) looksLike(name string) {
	d.t.Helper()
	formtest.Golden(d.t, "frame_"+name, d.Screen()+"\n")
}
