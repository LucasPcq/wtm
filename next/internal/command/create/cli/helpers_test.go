package cli

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/create"
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

func runIn(t *testing.T, w world, args ...string) printed {
	t.Helper()
	return runOn(t, w, func(*output.Streams) {}, args...)
}

func runOn(t *testing.T, w world, setUp func(*output.Streams), args ...string) printed {
	t.Helper()
	streams, _, stdout, stderr := output.Test()
	setUp(streams)
	root := uicli.NewRoot("1.2.3", streams, []*cobra.Command{New(streams, w.env(), w.open)})
	code := uicli.Execute(t.Context(), root, args)
	return printed{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}
}

func runCreate(t *testing.T, w world, words ...string) printed {
	t.Helper()
	return runIn(t, w, append([]string{"create"}, words...)...)
}

// plan is the plan of create's envelope, as the tests read it.
type plan struct {
	Branches []struct {
		Branch string `json:"branch"`
		Action string `json:"action"`
	} `json:"branches"`
	From        string   `json:"from"`
	EnvFrom     string   `json:"env_from"`
	FastForward []string `json:"fast_forward"`
	Remember    []string `json:"remember"`
}

func decoded(t *testing.T, stdout string) formtest.Envelope[plan] {
	t.Helper()
	return formtest.Decoded[plan](t, stdout)
}

// A driver plays the wizard of a session.
type driver struct {
	*formtest.Driver
	t *testing.T
	s *session
}

func open(t *testing.T, params sessionParams) *driver {
	t.Helper()
	s := newForm(nil).Session(t.Context(), params)
	return start(t, s, form.New(t.Context(), s.Params()))
}

func start(t *testing.T, s *session, w *form.Wizard) *driver {
	t.Helper()
	return &driver{Driver: formtest.Start(w, tea.WindowSizeMsg{Width: 100, Height: 40}), t: t, s: s}
}

func (d *driver) looksLike(name string) {
	d.t.Helper()
	formtest.Golden(d.t, "engine_"+name, d.Screen())
}

func rendered(draw func(w *bytes.Buffer)) string {
	var out bytes.Buffer
	draw(&out)
	return out.String()
}

func jsonOf(t *testing.T, value any) string {
	t.Helper()
	data, err := json.Marshal(value)
	require.NoError(t, err)
	return string(data)
}

func enveloped(t *testing.T, outcome create.Outcome, err *domain.Error) string {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, output.Envelope(&out, outcome.Envelope(create.Name, err), createFlags.Spell))
	return out.String()
}
