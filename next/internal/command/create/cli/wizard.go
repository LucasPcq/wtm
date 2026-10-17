package cli

import (
	"context"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/ui/form"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type wizard struct {
	ctx      context.Context
	app      useCase
	given    create.Request
	req      create.Request
	dryRun   bool
	report   domain.Report
	q        create.Questions
	preview  *create.Outcome
	outcome  create.Outcome
	branches *branchesStep
	from     *sourceStep
	envFrom  *choiceStep
	isolate  *choiceStep
	ff       *fastForwardStep
}

type wizardParams struct {
	App     useCase
	Request create.Request
	DryRun  bool
	Report  domain.Report
}

func newWizard(ctx context.Context, params wizardParams) *wizard {
	w := &wizard{ctx: ctx, app: params.App, given: params.Request, req: params.Request, dryRun: params.DryRun, report: params.Report}
	w.branches = &branchesStep{at: at{w: w, step: stepBranches}}
	w.from = &sourceStep{at: at{w: w, step: stepFrom}}
	w.envFrom = &choiceStep{at: at{w: w, step: stepEnvFrom}}
	w.isolate = &choiceStep{at: at{w: w, step: stepIsolation}}
	w.ff = &fastForwardStep{at: at{w: w, step: stepFastForward}}
	return w
}

func (w *wizard) form() form.Params {
	return form.Params{
		Script: form.Script{
			Steps:   []form.Step{w.branches, w.from, w.envFrom, w.isolate, w.ff, ifNotExistsStep{at: at{w: w, step: stepIfNotExists}}},
			Observe: w.observe,
			Preview: w.previewPlan,
			Plan:    w.plan,
			Label:   label,
			Apply:   w.apply,
			Stale:   stale,
		},
		DryRun: w.dryRun,
	}
}

func (w *wizard) observe(ctx context.Context) (func(), error) {
	questions, err := w.app.Questions(ctx, w.req)
	return func() { w.q = questions }, err
}

type at struct {
	w    *wizard
	step step
}

func (a at) Label() string {
	return labels[a.step]
}

func (a at) Description() string {
	return descriptions[paths[a.step]]
}

func (a at) Standing() form.Standing {
	return a.w.standing(a.step)
}

func (a at) Reset() {
	a.w.reset(a.step)
}

type branchesStep struct {
	at
	list *form.TextList
}

func (s *branchesStep) Open() tea.Cmd {
	s.list = form.NewTextList(s.w.req.Branches, s.w.branchProblem, s.w.branchBadge)
	return textinput.Blink
}

func (s *branchesStep) Field() form.Field {
	return s.list
}

func (s *branchesStep) Answer() string {
	s.w.req.Branches = s.list.Entries()
	return strings.Join(s.w.req.Branches, ", ")
}

type sourceStep struct {
	at
	list *form.SelectList
}

func (s *sourceStep) Open() tea.Cmd {
	s.list = s.w.sourceList()
	return nil
}

func (s *sourceStep) Field() form.Field {
	return s.list
}

func (s *sourceStep) Answer() string {
	s.w.req.From = s.list.Value()
	return s.w.req.From
}

func (s *sourceStep) Refreshable() bool {
	return !s.list.Filtering()
}

func (s *sourceStep) Refreshed() {
	kept := s.list.Value()
	s.list = s.w.sourceList()
	s.list.StartOn(kept)
}

type choiceStep struct {
	at
	list *form.SelectList
}

func (s *choiceStep) Open() tea.Cmd {
	s.list = choiceList(s.w.choices(s.step))
	return nil
}

func (s *choiceStep) Field() form.Field {
	return s.list
}

func (s *choiceStep) Answer() string {
	return s.w.choose(s.step, s.list.Value(), s.list.Remember())
}

type fastForwardStep struct {
	at
	box *form.Confirm
}

func (s *fastForwardStep) Open() tea.Cmd {
	s.box = form.NewConfirm(s.w.q.FastForward.Value)
	return nil
}

func (s *fastForwardStep) Field() form.Field {
	return s.box
}

func (s *fastForwardStep) Answer() string {
	s.w.req.FastForward = new(s.box.Yes())
	return yesNo(s.box.Yes())
}

type ifNotExistsStep struct {
	at
}

func (ifNotExistsStep) Open() tea.Cmd {
	return nil
}

func (ifNotExistsStep) Field() form.Field {
	return nil
}

func (ifNotExistsStep) Answer() string {
	return ""
}

func (w *wizard) choose(s step, value string, remember bool) string {
	if s == stepIsolation {
		w.req.Isolation = project.Isolation(value)
	} else {
		w.req.EnvFrom = project.EnvStrategy(value)
	}
	path := paths[s]
	w.req.Remember = slices.DeleteFunc(slices.Clone(w.req.Remember), func(p string) bool { return p == path })
	if remember {
		w.req.Remember = append(w.req.Remember, path)
	}
	return value
}

func (w *wizard) reset(s step) {
	switch s {
	case stepBranches:
		w.req.Branches = w.given.Branches
	case stepFrom:
		w.req.From = w.given.From
	case stepEnvFrom:
		w.choose(s, string(w.given.EnvFrom), slices.Contains(w.given.Remember, paths[s]))
	case stepIsolation:
		w.choose(s, string(w.given.Isolation), slices.Contains(w.given.Remember, paths[s]))
	case stepFastForward:
		w.req.FastForward = w.given.FastForward
	case stepRecap, stepIfNotExists, stepCount:
	}
}
