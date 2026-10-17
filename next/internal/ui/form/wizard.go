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

// A Script is what a command's form asks and does: its steps, and the functions the wizard calls to read the repository, preview the plan and apply it.
type Script struct {
	Steps []Step
	// Observe reads the repository for the request as answered so far. It runs off the event loop and returns the function that installs what it read, which the wizard calls on the loop, even on an error.
	Observe func(ctx context.Context) (func(), error)
	// Preview computes the plan the recap shows, the same way as Observe.
	Preview func(ctx context.Context) (func(), error)
	// Plan draws the plan Preview computed in width columns.
	Plan func(width int, st *output.Styles) string
	// Label returns the label of the step that sets a request's field path, for the fields a failed preview refuses.
	Label func(path string) string
	// ApplyLabel names the recap's option that applies the plan Preview computed; nil offers "Yes, apply".
	ApplyLabel func() string
	// Apply carries out the request once the wizard has finished.
	Apply func(ctx context.Context) error
	// Stale reports whether err, an error of Apply, is a plan the facts changed under, which starts the wizard again.
	Stale func(err error) bool
}

// An Ending is how a wizard ended.
type Ending int

// The endings of a wizard.
const (
	Asking    Ending = iota // the wizard has not ended
	Finished                // every step is settled, and the recap confirmed unless on a dry run
	Cancelled               // the user left: ctrl+c, esc on the first step asked, or "No, cancel" at the recap
)

// A Wizard is the Bubble Tea model of a command's form: it walks a [Script]'s steps to a recap of the plan, then quits with its [Ending].
//
// It is a state machine whose state is data, changed only in Update on a message. The wizard reads the repository before the first step and again after each answer, since an answer changes the steps the facts leave open; a step a flag, a remembered answer or a rule settles is not asked but shown in the trail. Keys typed while it reads are queued and replayed once the read lands.
type Wizard struct {
	ctx      context.Context
	script   Script
	dryRun   bool   // a dry run stops before the recap: its plan is printed, not confirmed
	stale    bool   // the plan the user confirmed changed before it applied: the recap says so
	answered []bool // per step, whether the user answered it in the form a stale plan restarted
	marks    []mark // per step, the recap included, how it was settled
	current  int    // the step asked, or len(script.Steps) at the recap
	loaded   bool   // the first read has landed
	recap    *SelectList
	planned  bool          // the preview has landed with a plan
	failure  *domain.Error // the preview's failure, shown at the recap in place of the plan
	loading  string        // the text beside the spinner, "" when nothing loads
	reading  bool          // a read is in flight: keys wait in queued
	queued   []tea.KeyMsg
	spinner  spinner.Model
	styles   *Styles
	size     Size
	ending   Ending
	err      error // the failed read that ended the wizard
}

// Params is what a wizard walks: its script, and whether it is a dry run, which ends before the recap.
type Params struct {
	Script Script
	DryRun bool
}

// New returns a wizard over params that reads and previews under ctx, drawn without colour until [Run] gives it its stream's styles.
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

// Again returns a new wizard over the same script, for a plan that turned stale as it applied: the steps the user answered keep their answers unless the facts now refuse them, and the recap warns that the facts changed.
func (w *Wizard) Again() *Wizard {
	next := New(w.ctx, Params{Script: w.script, DryRun: w.dryRun})
	next.stale = true
	next.style(w.styles.Styles)

	for s, m := range w.marks {
		next.answered[s] = m.asked
	}
	return next
}

// Ending returns how the wizard ended, [Asking] while it runs.
func (w *Wizard) Ending() Ending {
	return w.ending
}

// Current returns the index of the step asked, len(Steps) at the recap.
func (w *Wizard) Current() int {
	return w.current
}

// ----------------------------------------------------------------------------
// The event loop

// Init starts the spinner and the first read of the repository.
func (w *Wizard) Init() tea.Cmd {
	return tea.Batch(w.load(readingText), w.observe(thenEnter))
}

// load shows text beside the spinner and returns the tick that turns it.
func (w *Wizard) load(text string) tea.Cmd {
	w.loading = text
	return w.spinner.Tick
}

// Update moves the wizard on msg: a key, a new size, the spinner's tick, a read or a preview that landed; any other message goes to the field asked.
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

// key handles a typed key: ctrl+c cancels at once, a key typed during a read waits for it, r refreshes a [Refresher] step, and any other key goes to the field asked.
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

// settle moves the wizard on a field's verdict, and returns the field's own cmd while the field stays.
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

// refresher returns the step asked as a [Refresher], if it is one.
func (w *Wizard) refresher() (Refresher, bool) {
	if w.atRecap() {
		return nil, false
	}

	r, ok := w.script.Steps[w.current].(Refresher)
	return r, ok
}

// ----------------------------------------------------------------------------
// Reading the repository

// A then is what the wizard does once a read lands.
type then int

const (
	thenEnter   then = iota // enter the first step left open
	thenRefresh             // rebuild the field of the step asked
	thenNext                // enter the next step left open
	thenStay                // stay on the step asked, after going back to it
)

// An observedMsg brings what a read of the repository found back to the event loop.
type observedMsg struct {
	apply func()
	err   error
	then  then
}

// A previewMsg brings the plan a preview computed back to the event loop.
type previewMsg struct {
	apply func()
	err   error
}

// observe reads the repository off the event loop and comes back with an observedMsg carrying after.
func (w *Wizard) observe(after then) tea.Cmd {
	w.reading = true
	return func() tea.Msg {
		apply, err := w.script.Observe(w.ctx)
		return observedMsg{apply: apply, err: err, then: after}
	}
}

// observed installs what a read found, ends the wizard if the read failed, else moves on as m says and replays the keys typed meanwhile.
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

// replay handles the queued keys in the order they were typed, until one starts another read or ends the wizard.
func (w *Wizard) replay() tea.Cmd {
	var cmds []tea.Cmd
	for len(w.queued) > 0 && !w.reading && w.ending == Asking {
		typed := w.queued[0]
		w.queued = w.queued[1:]
		cmds = append(cmds, w.key(typed))
	}
	return tea.Batch(cmds...)
}

// ----------------------------------------------------------------------------
// Walking the steps

func (w *Wizard) recapAt() int {
	return len(w.script.Steps)
}

func (w *Wizard) atRecap() bool {
	return w.current == w.recapAt()
}

// last returns the index the walk stops before: the recap's on a dry run, the one after it otherwise.
func (w *Wizard) last() int {
	if w.dryRun {
		return w.recapAt()
	}
	return w.recapAt() + 1
}

// decide returns how step s is settled on the facts read last; the recap is always asked.
func (w *Wizard) decide(s int) mark {
	if s == w.recapAt() {
		return mark{asked: true}
	}
	return decided(w.script.Steps[s].Standing(), w.answered[s])
}

// asks reports whether the facts read last leave a step for the user to answer.
func (w *Wizard) asks() bool {
	for s := range w.script.Steps {
		if w.decide(s).asked {
			return true
		}
	}
	return false
}

// enter settles the steps from `from` on until one is left open, and asks it; it finishes the wizard when none is.
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

// open builds the field of step s, or starts the preview at the recap.
func (w *Wizard) open(s int) tea.Cmd {
	if s == w.recapAt() {
		return w.previewing()
	}
	return w.script.Steps[s].Open()
}

// field returns the field keys go to, nil before the first read lands and once the wizard has ended.
func (w *Wizard) field() Field {
	switch {
	case !w.loaded || w.ending != Asking:
		return nil
	case w.atRecap():
		return w.recap
	}
	return w.script.Steps[w.current].Field()
}

// answer records the answer of the step asked in the trail and reads the repository again before the next step; at the recap it ends the wizard.
func (w *Wizard) answer() tea.Cmd {
	if w.atRecap() {
		return w.confirmed()
	}

	step := w.script.Steps[w.current]
	answered := step.Answer()
	w.marks[w.current].line = summarized(step.Standing(), answered)

	return tea.Batch(w.load(readingText), w.observe(thenNext))
}

// back returns to the last step asked before the current one, forgetting the answers of the steps after it, and reads the repository again; with no step asked before, it cancels the wizard.
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
		w.marks[s], w.answered[s] = mark{}, false
		if s < w.recapAt() {
			w.script.Steps[s].Reset()
		}
	}
	w.current = to

	return tea.Batch(w.load(readingText), w.observe(thenStay))
}
