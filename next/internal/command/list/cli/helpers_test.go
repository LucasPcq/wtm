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
	p := newPicker(t, w, req)
	return &driver{Driver: formtest.Start(form.New(t.Context(), p.script()), tea.WindowSizeMsg{Width: 100, Height: 30}), t: t, p: p}
}

// newPicker is the picker of what req reads of w, before it is walked.
func newPicker(t *testing.T, w world, req list.Request) *picker {
	t.Helper()
	app, err := w.open(t.Context(), mainPath)
	require.NoError(t, err)
	read, err := app.Run(t.Context(), req, domain.RunOptions{})
	require.NoError(t, err)
	return &picker{app: app, read: read, cwd: w.env().Cwd, withPRs: req.WithPRs}
}

// shownDetail is the detail of subject in the read p shows.
func shownDetail(t *testing.T, p *picker, subject string) list.Detail {
	t.Helper()
	for _, item := range p.read.Items {
		if item.Subject == subject && item.Detail != nil {
			return *item.Detail
		}
	}
	t.Fatalf("no detail of %s shown", subject)
	return list.Detail{}
}

func (d *driver) looksLike(name string) {
	d.t.Helper()
	formtest.Golden(d.t, "pick_"+name, d.Screen())
}
