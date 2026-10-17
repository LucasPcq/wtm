package form

import (
	"context"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/app"
	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/surface/output"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	readingText  = "Reading the repository…"
	refreshText  = "Refreshing…"
	planText     = "Computing the plan…"
	recapTitle   = "Apply this plan?"
	failedTitle  = "Cannot apply this plan"
	reviewTitle  = "Review & confirm"
	staleText    = "the facts changed since the preview"
	applyValue   = "apply"
	cancelValue  = "cancel"
	none         = "none"
	trail        = " · "
	keyInterrupt = "ctrl+c"
	keyRefresh   = "r"
	defaultWidth = 80
	minListRows  = 3
)

type verdict int

const (
	stay verdict = iota
	done
	back
)

type size struct {
	Width  int
	Height int
}

type help struct {
	Actions []string
	Modal   string
	Rowless bool
}

type field interface {
	update(msg tea.Msg) (verdict, tea.Cmd)
	view(size size) string
	help() help
}

type wizard struct {
	ctx      context.Context
	app      App
	given    create.Request
	req      create.Request
	dryRun   bool
	stale    bool
	answered [stepCount]bool
	q        create.Questions
	loaded   bool
	marks    [stepCount]mark
	current  step
	branches *textList
	from     *selectList
	envFrom  *selectList
	isolate  *selectList
	ff       *confirm
	recap    *selectList
	preview  *app.CreateOutcome
	failure  *wtm.Error
	loading  string
	spinner  spinner.Model
	size     size
	finished bool
	aborted  bool
	err      error
}

type wizardParams struct {
	App     App
	Request create.Request
	DryRun  bool
}

func newWizard(ctx context.Context, params wizardParams) *wizard {
	s := spinner.New()
	s.Spinner, s.Style = spinner.MiniDot, output.Muted
	return &wizard{ctx: ctx, app: params.App, given: params.Request, req: params.Request, dryRun: params.DryRun, spinner: s, size: size{Width: defaultWidth}}
}

func (w *wizard) again() *wizard {
	next := newWizard(w.ctx, wizardParams{App: w.app, Request: w.given, DryRun: w.dryRun})
	next.req, next.stale = w.req, true
	for s := range stepCount {
		next.answered[s] = w.marks[s].asked
	}
	return next
}

type questionsMsg struct {
	questions create.Questions
	err       error
	refresh   bool
}

type previewMsg struct {
	outcome app.CreateOutcome
	err     error
}

func (w *wizard) Init() tea.Cmd {
	return tea.Batch(w.load(readingText), w.observe(false))
}

func (w *wizard) load(text string) tea.Cmd {
	w.loading = text
	return w.spinner.Tick
}

func (w *wizard) observe(refresh bool) tea.Cmd {
	req := w.req
	return func() tea.Msg {
		questions, err := w.app.CreateQuestions(w.ctx, req)
		return questionsMsg{questions: questions, err: err, refresh: refresh}
	}
}

func (w *wizard) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m := msg.(type) {
	case tea.KeyMsg:
		return w, w.key(m)
	case tea.WindowSizeMsg:
		w.size = size{Width: m.Width, Height: m.Height}
		return w, nil
	case spinner.TickMsg:
		if w.loading == "" {
			return w, nil
		}
		var cmd tea.Cmd
		w.spinner, cmd = w.spinner.Update(m)
		return w, cmd
	case questionsMsg:
		return w, w.observed(m)
	case previewMsg:
		w.previewed(m)
		return w, nil
	}
	if field := w.field(); field != nil {
		_, cmd := field.update(msg)
		return w, cmd
	}
	return w, nil
}

func (w *wizard) key(m tea.KeyMsg) tea.Cmd {
	if m.String() == keyInterrupt {
		w.aborted = true
		return tea.Quit
	}
	field := w.field()
	if field == nil || w.loading != "" {
		return nil
	}
	if m.String() == keyRefresh && w.current == stepFrom && !w.from.filtering {
		return tea.Batch(w.load(refreshText), w.observe(true))
	}
	verdict, cmd := field.update(m)
	switch verdict {
	case done:
		return w.answer()
	case back:
		return w.back()
	case stay:
	}
	return cmd
}

func (w *wizard) observed(m questionsMsg) tea.Cmd {
	w.loading = ""
	if m.err != nil {
		w.err = m.err
		return tea.Quit
	}
	w.q = m.questions
	if !m.refresh {
		w.loaded = true
		return w.enter(stepBranches)
	}
	kept := w.from.value()
	w.from = w.sourceList()
	w.from.startOn(kept)
	return nil
}

func (w *wizard) last() step {
	if w.dryRun {
		return stepRecap
	}
	return stepCount
}

func (w *wizard) enter(from step) tea.Cmd {
	for s := from; s < w.last(); s++ {
		w.marks[s] = w.decide(s)
		if w.marks[s].asked {
			w.current = s
			return w.open(s)
		}
	}
	w.finished = true
	return tea.Quit
}

func (w *wizard) open(s step) tea.Cmd {
	switch s {
	case stepBranches:
		w.branches = newTextList(textListParams{Entries: w.req.Branches, Check: w.branchProblem, Badge: w.branchBadge})
		return textinput.Blink
	case stepFrom:
		w.from = w.sourceList()
	case stepEnvFrom:
		w.envFrom = choiceList(w.choices(s))
	case stepIsolation:
		w.isolate = choiceList(w.choices(s))
	case stepFastForward:
		w.ff = newConfirm(w.q.FastForward.Value)
	case stepRecap:
		return w.previewing()
	case stepIfNotExists, stepCount:
	}
	return nil
}

func (w *wizard) field() field {
	if !w.loaded || w.finished || w.aborted {
		return nil
	}
	switch w.current {
	case stepBranches:
		return w.branches
	case stepFrom:
		return w.from
	case stepEnvFrom:
		return w.envFrom
	case stepIsolation:
		return w.isolate
	case stepFastForward:
		return w.ff
	case stepRecap:
		return w.recap
	case stepIfNotExists, stepCount:
	}
	return nil
}

func (w *wizard) answer() tea.Cmd {
	s := w.current
	var shown string
	switch s {
	case stepBranches:
		w.req.Branches = w.branches.entries
		shown = strings.Join(w.req.Branches, ", ")
	case stepFrom:
		w.req.From = w.from.value()
		shown = w.req.From
	case stepEnvFrom:
		shown = w.choose(chosen{Step: s, Value: w.envFrom.value(), Remember: w.envFrom.remember})
	case stepIsolation:
		shown = w.choose(chosen{Step: s, Value: w.isolate.value(), Remember: w.isolate.remember})
	case stepFastForward:
		w.req.FastForward = new(w.ff.yes())
		shown = yesNo(w.ff.yes())
	case stepRecap:
		return w.confirmed()
	case stepIfNotExists, stepCount:
	}
	w.marks[s].line = w.summarized(answer{Step: s, Shown: shown})
	if err := w.reobserve(); err != nil {
		w.err = err
		return tea.Quit
	}
	return w.enter(s + 1)
}

type chosen struct {
	Step     step
	Value    string
	Remember bool
}

func (w *wizard) choose(c chosen) string {
	if c.Step == stepIsolation {
		w.req.Isolation = project.Isolation(c.Value)
	} else {
		w.req.EnvFrom = project.EnvStrategy(c.Value)
	}
	path := paths[c.Step]
	w.req.Remember = slices.DeleteFunc(slices.Clone(w.req.Remember), func(p string) bool { return p == path })
	if c.Remember {
		w.req.Remember = append(w.req.Remember, path)
	}
	return c.Value
}

func (w *wizard) confirmed() tea.Cmd {
	if w.recap.value() == applyValue {
		w.finished = true
	} else {
		w.aborted = true
	}
	return tea.Quit
}

func (w *wizard) reobserve() error {
	questions, err := w.app.CreateQuestions(w.ctx, w.req)
	w.q = questions
	return err
}

func (w *wizard) back() tea.Cmd {
	to := w.current - 1
	for to >= 0 && !w.marks[to].asked {
		to--
	}
	if to < 0 {
		w.aborted = true
		return tea.Quit
	}
	for s := to + 1; s <= w.current; s++ {
		w.reset(s)
	}
	w.current = to
	if err := w.reobserve(); err != nil {
		w.err = err
		return tea.Quit
	}
	return nil
}

func (w *wizard) reset(s step) {
	w.marks[s] = mark{}
	switch s {
	case stepBranches:
		w.req.Branches = w.given.Branches
	case stepFrom:
		w.req.From = w.given.From
	case stepEnvFrom:
		w.choose(chosen{Step: s, Value: string(w.given.EnvFrom), Remember: slices.Contains(w.given.Remember, paths[s])})
	case stepIsolation:
		w.choose(chosen{Step: s, Value: string(w.given.Isolation), Remember: slices.Contains(w.given.Remember, paths[s])})
	case stepFastForward:
		w.req.FastForward = w.given.FastForward
	case stepRecap, stepIfNotExists, stepCount:
	}
}
