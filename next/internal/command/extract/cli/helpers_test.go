package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	update      = flag.Bool("update", false, "rewrite the golden files")
	testdata, _ = filepath.Abs("testdata")
)

type printed struct {
	Stdout string
	Stderr string
	Code   int
}

func runExtract(t *testing.T, w world, words ...string) printed {
	t.Helper()
	streams, _, stdout, stderr := output.Test()
	root := uicli.NewRoot("1.2.3", streams, []*cobra.Command{New(streams, w.env(), w.open)})
	code := uicli.Execute(t.Context(), root, append([]string{"extract"}, words...))
	return printed{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}
}

type envelope struct {
	V       int    `json:"v"`
	Command string `json:"command"`
	OK      bool   `json:"ok"`
	Items   []struct {
		Subject string          `json:"subject"`
		Status  string          `json:"status"`
		Detail  json.RawMessage `json:"detail"`
	} `json:"items"`
	Plan *struct {
		Source string `json:"source"`
		Target struct {
			Branch string `json:"branch"`
			Action string `json:"action"`
		} `json:"target"`
		Keep bool `json:"keep"`
	} `json:"plan"`
	Fingerprint string `json:"fingerprint"`
	Error       *struct {
		Kind    string `json:"kind"`
		Code    string `json:"code"`
		Message string `json:"message"`
		Fields  []struct {
			Path string `json:"path"`
			Code string `json:"code"`
			Flag string `json:"flag"`
		} `json:"fields"`
	} `json:"error"`
}

func decoded(t *testing.T, stdout string) envelope {
	t.Helper()
	var doc envelope
	require.NoError(t, json.Unmarshal([]byte(stdout), &doc), stdout)
	return doc
}

func golden(t *testing.T, name string, got string) {
	t.Helper()
	path := filepath.Join(testdata, name+".golden")
	if *update {
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), got)
}

func paths(changes []worktree.Change) []string {
	found := make([]string, 0, len(changes))
	for _, change := range changes {
		found = append(found, change.Path)
	}
	return found
}

const settleWithin = 100 * time.Millisecond

// A driver plays the wizard's event loop by hand: it sends a message, runs the command it returns, and feeds back the message that command produces.
type driver struct {
	t *testing.T
	w *form.Wizard
	s *session
}

func open(t *testing.T, params sessionParams) *driver {
	t.Helper()
	s := newForm(nil).Session(t.Context(), params)
	return start(t, s, form.New(t.Context(), s.Params()))
}

func start(t *testing.T, s *session, w *form.Wizard) *driver {
	t.Helper()
	d := &driver{t: t, w: w, s: s}
	d.send(tea.WindowSizeMsg{Width: 100, Height: 40})
	d.run(w.Init())
	return d
}

func (d *driver) send(msg tea.Msg) {
	_, cmd := d.w.Update(msg)
	d.run(cmd)
}

// run runs cmd and sends back its message; a command still running after settleWithin, a tick or a cursor blink is dropped, so the loop settles.
func (d *driver) run(cmd tea.Cmd) {
	if cmd == nil {
		return
	}
	result := make(chan tea.Msg, 1)
	go func() { result <- cmd() }()
	var msg tea.Msg
	select {
	case msg = <-result:
	case <-time.After(settleWithin):
		return
	}
	switch m := msg.(type) {
	case nil, tea.QuitMsg, spinner.TickMsg:
	case tea.BatchMsg:
		for _, inner := range m {
			d.run(inner)
		}
	default:
		if !strings.Contains(fmt.Sprintf("%T", m), "blink") {
			d.send(m)
		}
	}
}

func (d *driver) keys(keys ...string) {
	for _, key := range keys {
		switch key {
		case "enter":
			d.send(tea.KeyMsg{Type: tea.KeyEnter})
		case "esc":
			d.send(tea.KeyMsg{Type: tea.KeyEsc})
		case "up":
			d.send(tea.KeyMsg{Type: tea.KeyUp})
		case "down":
			d.send(tea.KeyMsg{Type: tea.KeyDown})
		case "space":
			d.send(tea.KeyMsg{Type: tea.KeySpace, Runes: []rune{' '}})
		default:
			d.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
		}
	}
}

func (d *driver) screen() string {
	lines := strings.Split(ansi.Strip(d.w.View()), "\n")
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " ")
	}
	return strings.TrimRight(strings.Join(lines, "\n"), "\n")
}

// looksLike compares the screen with the frame testdata/frame_<name>.golden holds.
func (d *driver) looksLike(name string) {
	d.t.Helper()
	golden(d.t, "frame_"+name, d.screen()+"\n")
}
