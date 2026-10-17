package form

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/app"
	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/surface/output"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/run"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type step int

const (
	stepBranches step = iota
	stepFrom
	stepEnvFrom
	stepIsolation
	stepFastForward
	stepIfNotExists
	stepRecap
	stepCount
)

var labels = [stepCount]string{"Branches", "Source branch", "Env strategy", "Isolation", "Source update", "If not exists", "Confirm"}

var paths = [stepCount]string{"branches", "from", "env_from", "isolation", "fast_forward", "if_not_exists", ""}

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
	ruledByFlag  = "set by its flag only"
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

type mark struct {
	asked   bool
	settled string
	ruled   string
	summary string
	offered standing
}

type standing struct {
	value   string
	flagged bool
	origin  create.Origin
	skip    string
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
	s.Spinner, s.Style = spinner.MiniDot, muted
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

func (w *wizard) standing(s step) standing {
	q := w.q
	switch s {
	case stepBranches:
		return standing{value: strings.Join(q.Branches.Given, ", "), flagged: len(w.given.Branches) > 0}
	case stepFrom:
		return standing{value: q.From.Value, flagged: w.given.From != "", origin: q.From.Origin}
	case stepEnvFrom:
		return standing{value: string(q.EnvFrom.Value), flagged: w.given.EnvFrom != "", origin: q.EnvFrom.Origin}
	case stepIsolation:
		return standing{value: string(q.Isolation.Value), flagged: w.given.Isolation != "", origin: q.Isolation.Origin, skip: q.Isolation.Skip}
	case stepFastForward:
		return standing{value: yesNo(q.FastForward.Value), flagged: w.given.FastForward != nil, origin: q.FastForward.Origin, skip: q.FastForward.Skip}
	case stepIfNotExists:
		return standing{skip: ruledByFlag}
	case stepRecap, stepCount:
	}
	return standing{}
}

func (w *wizard) decide(s step) mark {
	st := w.standing(s)
	value := cmp.Or(st.value, none)
	switch {
	case s == stepRecap:
		return mark{asked: true}
	case st.skip != "":
		return mark{ruled: st.skip}
	case w.erred(paths[s]):
		return mark{asked: true, offered: st}
	case st.flagged:
		return mark{settled: value + trail + output.Flag(paths[s])}
	case w.answered[s]:
		return mark{settled: value}
	case st.origin == create.OriginRemembered:
		return mark{settled: value + trail + string(create.OriginRemembered)}
	}
	return mark{asked: true, offered: st}
}

func (w *wizard) erred(path string) bool {
	return slices.ContainsFunc(w.q.Errors, func(problem wtm.FieldError) bool {
		return problem.Path == path || strings.HasPrefix(problem.Path, path+"[")
	})
}

func (w *wizard) open(s step) tea.Cmd {
	q := w.q
	switch s {
	case stepBranches:
		w.branches = newTextList(textListParams{Entries: w.req.Branches, Check: w.branchProblem, Badge: w.branchBadge})
		return textinput.Blink
	case stepFrom:
		w.from = w.sourceList()
	case stepEnvFrom:
		w.envFrom = choiceList(choiceParams{Options: names(q.EnvFrom.Options), Value: string(q.EnvFrom.Value), Remembered: slices.Contains(w.req.Remember, paths[s])})
	case stepIsolation:
		w.isolate = choiceList(choiceParams{Options: names(q.Isolation.Options), Value: string(q.Isolation.Value), Remembered: slices.Contains(w.req.Remember, paths[s])})
	case stepFastForward:
		w.ff = newConfirm(q.FastForward.Value)
	case stepRecap:
		return w.previewing()
	case stepIfNotExists, stepCount:
	}
	return nil
}

func (w *wizard) sourceList() *selectList {
	from := w.q.From
	options := make([]option, 0, len(from.Options))
	for _, o := range from.Options {
		options = append(options, option{Label: o.Value, Value: o.Value, Badge: o.Badge})
	}
	if at := slices.IndexFunc(options, func(o option) bool { return o.Value == from.Pinned }); at > 0 {
		pinned := options[at]
		options = append([]option{pinned}, slices.Delete(options, at, at+1)...)
	}
	return newSelectList(selectParams{Options: options, Start: cmp.Or(from.Value, from.Pinned)})
}

type choiceParams struct {
	Options    []string
	Value      string
	Remembered bool
}

func choiceList(params choiceParams) *selectList {
	options := make([]option, 0, len(params.Options))
	for _, value := range params.Options {
		options = append(options, option{Label: value, Value: value})
	}
	return newSelectList(selectParams{Options: options, Start: params.Value, Toggle: true, ToggleOn: params.Remembered})
}

func (w *wizard) branchProblem(entries []string, from int) error {
	req := w.req
	req.Branches = entries
	questions, err := w.app.CreateQuestions(w.ctx, req)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return problem(wtm.FieldError{Path: paths[stepBranches], Template: "{path} is required"})
	}
	for _, found := range questions.Errors {
		index, entry := strings.CutPrefix(found.Path, paths[stepBranches]+"[")
		at, err := strconv.Atoi(strings.TrimSuffix(index, "]"))
		if entry && err == nil && at >= from {
			return problem(found)
		}
	}
	return nil
}

func problem(found wtm.FieldError) error {
	return errors.New(found.Message(label(found.Path)))
}

func label(path string) string {
	name, index, indexed := strings.Cut(path, "[")
	at := slices.Index(paths[:], name)
	if at < 0 {
		return path
	}
	if indexed {
		return labels[at] + "[" + index
	}
	return labels[at]
}

func (w *wizard) branchBadge(entry string) string {
	at := slices.IndexFunc(w.q.Branches.Options, func(o create.Option) bool { return o.Value == entry })
	if at < 0 {
		return ""
	}
	return w.q.Branches.Options[at].Badge
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
		w.req.EnvFrom = project.EnvStrategy(w.envFrom.value())
		w.remember(rememberParams{Path: paths[s], On: w.envFrom.remember})
		shown = string(w.req.EnvFrom)
	case stepIsolation:
		w.req.Isolation = run.Isolation(w.isolate.value())
		w.remember(rememberParams{Path: paths[s], On: w.isolate.remember})
		shown = string(w.req.Isolation)
	case stepFastForward:
		w.req.FastForward = new(w.ff.yes())
		shown = yesNo(w.ff.yes())
	case stepRecap:
		return w.confirmed()
	case stepIfNotExists, stepCount:
	}
	offered := w.marks[s].offered
	kept := offered.origin == create.OriginConfig || offered.origin == create.OriginDefault
	w.marks[s].summary = shown
	if kept && shown == offered.value {
		w.marks[s].summary = shown + trail + string(offered.origin)
	}
	if err := w.reobserve(); err != nil {
		w.err = err
		return tea.Quit
	}
	return w.enter(s + 1)
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

type rememberParams struct {
	Path string
	On   bool
}

func (w *wizard) remember(params rememberParams) {
	w.req.Remember = slices.DeleteFunc(slices.Clone(w.req.Remember), func(path string) bool { return path == params.Path })
	if params.On {
		w.req.Remember = append(w.req.Remember, params.Path)
	}
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
		w.req.EnvFrom = w.given.EnvFrom
		w.remember(rememberParams{Path: paths[s], On: slices.Contains(w.given.Remember, paths[s])})
	case stepIsolation:
		w.req.Isolation = w.given.Isolation
		w.remember(rememberParams{Path: paths[s], On: slices.Contains(w.given.Remember, paths[s])})
	case stepFastForward:
		w.req.FastForward = w.given.FastForward
	case stepRecap, stepIfNotExists, stepCount:
	}
}

func yesNo(yes bool) string {
	if yes {
		return "yes"
	}
	return "no"
}

func names[T ~string](values []T) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		out = append(out, string(value))
	}
	return out
}
