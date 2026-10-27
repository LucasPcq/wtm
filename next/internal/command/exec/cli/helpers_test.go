package cli

import (
	"context"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func runExec(t *testing.T, w world, words ...string) formtest.Printed {
	t.Helper()
	return formtest.Run(t.Context(), func(streams *output.Streams) *cobra.Command {
		return New(streams, w.env(), w.open, formtest.CommandLine)
	}, append([]string{"exec"}, words...)...)
}

// runCancelled runs exec in w, on a terminal when live, cancelled as its command starts, which holds on until the cancel was announced.
func runCancelled(t *testing.T, w world, live bool, words ...string) formtest.Printed {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	w.shell = func(run context.Context, chain env.HookChain) error {
		cancel()
		<-run.Done()
		time.Sleep(200 * time.Millisecond)
		return &env.HookFailure{Cmd: chain.Hooks[0].Cmd, Err: run.Err()}
	}
	return formtest.Run(ctx, func(streams *output.Streams) *cobra.Command {
		streams.Err.SetTerminal(live)
		return New(streams, w.env(), w.open, formtest.CommandLine)
	}, append([]string{"exec"}, words...)...)
}

// A driver plays the wizard of a session.
type driver struct {
	*formtest.Driver
	t *testing.T
	s *session
}

func open(t *testing.T, params sessionParams) *driver {
	t.Helper()
	bound := newForm(nil)
	bound.Bind(&cobra.Command{}, &uicli.Run{CommandLine: formtest.CommandLine})
	s := bound.Session(t.Context(), params.App, params.Request, domain.RunOptions{})
	return &driver{Driver: formtest.Start(form.New(t.Context(), s.Script()), tea.WindowSizeMsg{Width: 100, Height: 40}), t: t, s: s}
}

// looksLike compares the screen with the frame testdata/frame_<name>.golden holds.
func (d *driver) looksLike(name string) {
	d.t.Helper()
	formtest.Golden(d.t, "frame_"+name, d.Screen()+"\n")
}
