package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func runIn(t *testing.T, w world, args ...string) printed {
	t.Helper()
	root := uicli.NewRoot("1.2.3", []*cobra.Command{New(w.open)})
	var stdout, stderr bytes.Buffer
	root.SetIn(&bytes.Buffer{})
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(args)
	code := uicli.Execute(t.Context(), root)
	return printed{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}
}

func runCreate(t *testing.T, w world, words ...string) printed {
	t.Helper()
	return runIn(t, w, append([]string{"create"}, words...)...)
}

type envelope struct {
	OK    bool   `json:"ok"`
	Op    string `json:"op"`
	Items []struct {
		Subject string `json:"subject"`
		Status  string `json:"status"`
		Reason  string `json:"reason"`
	} `json:"items"`
	Plan *struct {
		Branches []struct {
			Branch string `json:"branch"`
			Action string `json:"action"`
		} `json:"branches"`
		From        string   `json:"from"`
		EnvFrom     string   `json:"env_from"`
		FastForward []string `json:"fast_forward"`
		Remember    []string `json:"remember"`
	} `json:"plan"`
	Error *struct {
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

const settleWithin = 100 * time.Millisecond

type driver struct {
	t *testing.T
	w *form.Wizard
	c *wizard
}

func open(t *testing.T, params wizardParams) *driver {
	t.Helper()
	c := newWizard(t.Context(), params)
	return start(t, c, form.New(t.Context(), c.form()))
}

func start(t *testing.T, c *wizard, w *form.Wizard) *driver {
	t.Helper()
	d := &driver{t: t, w: w, c: c}
	d.send(tea.WindowSizeMsg{Width: 100, Height: 40})
	d.run(w.Init())
	return d
}

func (d *driver) send(msg tea.Msg) {
	_, cmd := d.w.Update(msg)
	d.run(cmd)
}

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
		case "tab":
			d.send(tea.KeyMsg{Type: tea.KeyTab})
		case "esc":
			d.send(tea.KeyMsg{Type: tea.KeyEsc})
		case "up":
			d.send(tea.KeyMsg{Type: tea.KeyUp})
		case "down":
			d.send(tea.KeyMsg{Type: tea.KeyDown})
		case "backspace":
			d.send(tea.KeyMsg{Type: tea.KeyBackspace})
		case "ctrl+c":
			d.send(tea.KeyMsg{Type: tea.KeyCtrlC})
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

func engineScreen(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "engine_"+name+".txt"))
	require.NoError(t, err)
	return strings.TrimRight(engineGlyphsFontCovered.Replace(string(data)), "\n")
}

var engineGlyphsFontCovered = strings.NewReplacer("▸", "›", "❯", "›", "⊘", "·")

func (d *driver) looksLike(name string) {
	d.t.Helper()
	assert.Equal(d.t, engineScreen(d.t, name), d.screen())
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

func enveloped(t *testing.T, params envelopeParams) string {
	t.Helper()
	var out bytes.Buffer
	require.NoError(t, output.Envelope(&out, envelopeOf(params)))
	return out.String()
}
