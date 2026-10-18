package cli

import (
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/domain"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/formtest"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/require"
)

type printed struct {
	Stdout string
	Stderr string
	Code   int
}

func runList(t *testing.T, w world, words ...string) printed {
	t.Helper()
	streams, _, stdout, stderr := output.Test()
	root := uicli.NewRoot("1.2.3", streams, []*cobra.Command{New(streams, w.env(), w.open)})
	code := uicli.Execute(t.Context(), root, append([]string{list.Name}, words...))
	return printed{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}
}

// A driver plays the picker over what req reads of w.
type driver struct {
	*formtest.Driver
	t *testing.T
	p *picker
}

func picking(t *testing.T, w world, req list.Request) *driver {
	t.Helper()
	app, err := w.open(t.Context(), mainPath)
	require.NoError(t, err)
	read, err := app.Run(t.Context(), req, domain.RunOptions{})
	require.NoError(t, err)
	p := &picker{app: app, req: req, read: read, cwd: w.env().Cwd, withPRs: req.WithPRs}
	return &driver{Driver: formtest.Start(form.New(t.Context(), p.script()), tea.WindowSizeMsg{Width: 100, Height: 30}), t: t, p: p}
}

func (d *driver) looksLike(name string) {
	d.t.Helper()
	formtest.Golden(d.t, "pick_"+name, d.Screen())
}
