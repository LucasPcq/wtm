package form

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/kernel/text"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
	"github.com/LucasPcq/wtm/internal/testutil/dispatchtest"
)

const settleWithin = 50 * time.Millisecond

type worldParams struct {
	Request string
	Options dispatch.Options
}

type world struct {
	Repo     *repo
	Progress *dispatchtest.Progress
	session  *session
}

func newWorld(t *testing.T, r *repo, params worldParams) *world {
	t.Helper()
	b := dispatchtest.New(t, dispatchtest.BenchParams{})
	entry, ok := dispatch.Register(dispatch.RegisterParams[request, facts, plan, string]{Dispatcher: b.Dispatcher, Command: makeCommand(r)}).Fillable()
	require.True(t, ok)
	progress := &dispatchtest.Progress{}
	s := newSession(t.Context(), Params{
		Entry:   entry,
		Request: json.RawMessage(params.Request),
		Options: params.Options,
		Names:   func(path string) string { return "--" + path },
		Plan: func(detail json.RawMessage) []styles.PlanRow {
			return []styles.PlanRow{{Label: "Plan", Values: []string{string(detail)}}}
		},
		Progress: progress,
		Catalog:  text.NewCatalog(),
	})
	s.loading = func(params components.LoadingParams) error { return params.Work() }
	return &world{Repo: r, Progress: progress, session: s}
}

func (w *world) open(t *testing.T) *driver {
	t.Helper()
	require.NoError(t, w.session.observe())
	if w.session.flagged == nil {
		require.NoError(t, w.session.flag())
	}
	steps, _ := w.session.steps()
	return start(t, components.RunWizardParams{Steps: steps, OnMsg: w.session.handle, InitCmd: w.session.initCmd()})
}

func (w *world) script(t *testing.T, turns ...func(*driver)) (dispatch.Reply, error) {
	t.Helper()
	opened := 0
	w.session.program = func(params components.RunWizardParams) (components.WizardModel, error) {
		require.Less(t, opened, len(turns), "the form opened one wizard more than scripted")
		d := start(t, params)
		turns[opened](d)
		opened++
		if d.model.Aborted() {
			return components.WizardModel{}, domain.ErrUserAborted
		}
		return d.model, nil
	}
	return w.session.run()
}

type driver struct {
	t     *testing.T
	model components.WizardModel
}

func start(t *testing.T, params components.RunWizardParams) *driver {
	t.Helper()
	d := &driver{t: t, model: components.NewWizardWithParams(components.WizardParams{
		Steps: params.Steps, OnMsg: params.OnMsg, InitCmd: params.InitCmd, Loading: params.Loading, LoadingText: params.LoadingText,
	})}
	d.send(tea.WindowSizeMsg{Width: 100, Height: 60})
	d.run(d.model.Init())
	return d
}

func (d *driver) send(msg tea.Msg) {
	d.t.Helper()
	d.run(d.update(msg))
}

func (d *driver) update(msg tea.Msg) tea.Cmd {
	d.t.Helper()
	next, cmd := d.model.Update(msg)
	model, ok := next.(components.WizardModel)
	require.True(d.t, ok)
	d.model = model
	// Left unrun: a test looks at the wizard while a load is in flight.
	return cmd
}

func (d *driver) run(cmd tea.Cmd) {
	d.t.Helper()
	for _, msg := range answers(cmd) {
		d.send(msg)
	}
}

func (d *driver) key(keys ...string) {
	d.t.Helper()
	for _, key := range keys {
		d.send(keyMsg(key))
	}
}

func (d *driver) typed(text string) {
	d.t.Helper()
	d.send(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(text)})
}

func (d *driver) view() string { return d.model.View() }

func keyMsg(key string) tea.KeyMsg {
	switch key {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case "space":
		return tea.KeyMsg{Type: tea.KeySpace, Runes: []rune(" ")}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	}
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
}

func answers(cmd tea.Cmd) []tea.Msg {
	var own []tea.Msg
	for _, msg := range collect(cmd) {
		switch msg.(type) {
		case pendingMsg, choicesMsg, refreshedMsg, previewMsg:
			own = append(own, msg)
		}
	}
	return own
}

func collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	out := make(chan tea.Msg, 1)
	go func() { out <- cmd() }()
	select {
	case msg := <-out:
		batch, isBatch := msg.(tea.BatchMsg)
		if !isBatch {
			return []tea.Msg{msg}
		}
		var msgs []tea.Msg
		for _, inner := range batch {
			msgs = append(msgs, collect(inner)...)
		}
		return msgs
	// The fakes answer at once: only a cursor blink or a spinner tick waits longer.
	case <-time.After(settleWithin):
		return nil
	}
}

func (d *driver) trail() []string {
	var lines []string
	for _, line := range strings.Split(d.view(), "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, domain.GlyphSuccess) || strings.HasPrefix(trimmed, "⊘") {
			lines = append(lines, strings.TrimSpace(strings.TrimLeft(trimmed, domain.GlyphSuccess+"⊘")))
		}
	}
	return lines
}

func fillAll(d *driver) {
	d.typed("feat/a")
	d.key("enter")
	for range 6 {
		d.key("enter")
	}
}

func requestOf(t *testing.T, raw json.RawMessage) request {
	t.Helper()
	var decoded request
	require.NoError(t, json.Unmarshal(raw, &decoded))
	return decoded
}

func (w *world) recapView(t *testing.T) string {
	t.Helper()
	step := w.session.recapStep()
	step.Model, step.Build, step.OnEnter = w.session.recapModel(), nil, nil
	d := start(t, components.RunWizardParams{Steps: []components.Step{step}})
	return ansi.Strip(d.view())
}

func previewOf(detail string, warnings ...kernel.Warning) *dispatch.CommandReply {
	return &dispatch.CommandReply{Result: dispatch.Result[json.RawMessage, json.RawMessage]{
		Plan: kernel.Plan[json.RawMessage]{Detail: json.RawMessage(detail), Warnings: warnings},
	}}
}

func dispatchDryRun() dispatch.Options { return dispatch.Options{DryRun: true} }
