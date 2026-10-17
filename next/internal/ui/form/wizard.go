package form

import (
	"context"
	"io"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	readingText  = "Reading the repository…"
	refreshText  = "Refreshing…"
	keyInterrupt = "ctrl+c"
	keyRefresh   = "r"
	defaultWidth = 80
	minListRows  = 3
)

type Script struct {
	Steps   []Step
	Observe func(ctx context.Context) (func(), error)
	Preview func(ctx context.Context) (func(), error)
	Plan    func(width int, st *output.Styles) string
	Label   func(path string) string
	Apply   func(ctx context.Context) error
	Stale   func(err error) bool
}

type Ending int

const (
	Asking Ending = iota
	Finished
	Cancelled
)

type Wizard struct {
	ctx      context.Context
	script   Script
	dryRun   bool
	stale    bool
	answered []bool
	marks    []mark
	current  int
	loaded   bool
	recap    *SelectList
	planned  bool
	failure  *domain.Error
	loading  string
	reading  bool
	queued   []tea.KeyMsg
	spinner  spinner.Model
	styles   *Styles
	size     Size
	ending   Ending
	err      error
}

type Params struct {
	Script Script
	DryRun bool
}

func New(ctx context.Context, params Params) *Wizard {
	s := spinner.New()
	s.Spinner = spinner.MiniDot
	count := len(params.Script.Steps) + 1
	w := &Wizard{ctx: ctx, script: params.Script, dryRun: params.DryRun, answered: make([]bool, count), marks: make([]mark, count), spinner: s, size: Size{Width: defaultWidth}}
	w.style(output.StylesOf(io.Discard))
	return w
}

func (w *Wizard) style(base *output.Styles) {
	w.styles = newStyles(base)
	w.spinner.Style = base.Muted
}

func (w *Wizard) Again() *Wizard {
	next := New(w.ctx, Params{Script: w.script, DryRun: w.dryRun})
	next.stale = true
	next.style(w.styles.Styles)
	for s, m := range w.marks {
		next.answered[s] = m.asked
	}
	return next
}

func (w *Wizard) Ending() Ending {
	return w.ending
}

func (w *Wizard) Current() int {
	return w.current
}

type then int

const (
	thenEnter then = iota
	thenRefresh
	thenNext
	thenStay
)

type observedMsg struct {
	apply func()
	err   error
	then  then
}

type previewMsg struct {
	apply func()
	err   error
}

func (w *Wizard) Init() tea.Cmd {
	return tea.Batch(w.load(readingText), w.observe(thenEnter))
}

func (w *Wizard) load(text string) tea.Cmd {
	w.loading = text
	return w.spinner.Tick
}

func (w *Wizard) observe(after then) tea.Cmd {
	w.reading = true
	return func() tea.Msg {
		apply, err := w.script.Observe(w.ctx)
		return observedMsg{apply: apply, err: err, then: after}
	}
}

func (w *Wizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		return w, w.key(m)
	case tea.WindowSizeMsg:
		w.size = Size{Width: m.Width, Height: m.Height}
		return w, nil
	case spinner.TickMsg:
		if w.loading == "" {
			return w, nil
		}
		var cmd tea.Cmd
		w.spinner, cmd = w.spinner.Update(m)
		return w, cmd
	case observedMsg:
		return w, w.observed(m)
	case previewMsg:
		w.previewed(m)
		return w, nil
	}
	if field := w.field(); field != nil {
		return w, w.settle(field.Update(msg))
	}
	return w, nil
}

func (w *Wizard) key(m tea.KeyMsg) tea.Cmd {
	if m.String() == keyInterrupt {
		w.ending = Cancelled
		return tea.Quit
	}
	if w.reading {
		w.queued = append(w.queued, m)
		return nil
	}
	field := w.field()
	if field == nil || w.loading != "" {
		return nil
	}
	if r, ok := w.refresher(); ok && m.String() == keyRefresh && r.Refreshable() {
		return tea.Batch(w.load(refreshText), w.observe(thenRefresh))
	}
	return w.settle(field.Update(m))
}

func (w *Wizard) settle(verdict Verdict, cmd tea.Cmd) tea.Cmd {
	switch verdict {
	case Done:
		return w.answer()
	case Back:
		return w.back()
	case Stay:
	}
	return cmd
}

func (w *Wizard) refresher() (Refresher, bool) {
	if w.atRecap() {
		return nil, false
	}
	r, ok := w.script.Steps[w.current].(Refresher)
	return r, ok
}

func (w *Wizard) observed(m observedMsg) tea.Cmd {
	w.loading, w.reading = "", false
	m.apply()
	if m.err != nil {
		w.err = m.err
		return tea.Quit
	}
	var cmd tea.Cmd
	switch m.then {
	case thenEnter:
		w.loaded = true
		cmd = w.enter(0)
	case thenNext:
		cmd = w.enter(w.current + 1)
	case thenRefresh:
		if r, ok := w.refresher(); ok {
			r.Refreshed()
		}
	case thenStay:
	}
	return tea.Batch(cmd, w.replay())
}

func (w *Wizard) replay() tea.Cmd {
	var cmds []tea.Cmd
	for len(w.queued) > 0 && !w.reading && w.ending == Asking {
		typed := w.queued[0]
		w.queued = w.queued[1:]
		cmds = append(cmds, w.key(typed))
	}
	return tea.Batch(cmds...)
}

func (w *Wizard) recapAt() int {
	return len(w.script.Steps)
}

func (w *Wizard) atRecap() bool {
	return w.current == w.recapAt()
}

func (w *Wizard) last() int {
	if w.dryRun {
		return w.recapAt()
	}
	return w.recapAt() + 1
}

func (w *Wizard) decide(s int) mark {
	if s == w.recapAt() {
		return mark{asked: true}
	}
	return decided(w.script.Steps[s].Standing(), w.answered[s])
}

func (w *Wizard) asks() bool {
	for s := range w.script.Steps {
		if w.decide(s).asked {
			return true
		}
	}
	return false
}

func (w *Wizard) enter(from int) tea.Cmd {
	for s := from; s < w.last(); s++ {
		w.marks[s] = w.decide(s)
		if w.marks[s].asked {
			w.current = s
			return w.open(s)
		}
	}
	w.ending = Finished
	return tea.Quit
}

func (w *Wizard) open(s int) tea.Cmd {
	if s == w.recapAt() {
		return w.previewing()
	}
	return w.script.Steps[s].Open()
}

func (w *Wizard) field() Field {
	switch {
	case !w.loaded || w.ending != Asking:
		return nil
	case w.atRecap():
		return w.recap
	}
	return w.script.Steps[w.current].Field()
}

func (w *Wizard) answer() tea.Cmd {
	if w.atRecap() {
		return w.confirmed()
	}
	step := w.script.Steps[w.current]
	answered := step.Answer()
	w.marks[w.current].line = summarized(step.Standing(), answered)
	return tea.Batch(w.load(readingText), w.observe(thenNext))
}

func (w *Wizard) back() tea.Cmd {
	to := w.current - 1
	for to >= 0 && !w.marks[to].asked {
		to--
	}
	if to < 0 {
		w.ending = Cancelled
		return tea.Quit
	}
	for s := to + 1; s <= w.current; s++ {
		w.marks[s] = mark{}
		if s < w.recapAt() {
			w.script.Steps[s].Reset()
		}
	}
	w.current = to
	return tea.Batch(w.load(readingText), w.observe(thenStay))
}
