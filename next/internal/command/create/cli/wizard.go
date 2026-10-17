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

// A wizard is create's form over the ui/form engine: the request as the flags gave it and as answered so far, the questions observed last, the plan previewed, the outcome of the apply, and the steps that are asked.
type wizard struct {
	ctx      context.Context
	app      useCase
	given    create.Request // as the flags gave it: a step given a flag is not asked, and a step gone back before returns to it
	req      create.Request
	opts     domain.RunOptions
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
	Options domain.RunOptions
}

// newWizard builds the wizard of params.Request, each step knowing its place in it.
func newWizard(ctx context.Context, params wizardParams) *wizard {
	w := &wizard{ctx: ctx, app: params.App, given: params.Request, req: params.Request, opts: params.Options}

	w.branches = &branchesStep{at: at{w: w, step: stepBranches}}
	w.from = &sourceStep{at: at{w: w, step: stepFrom}}
	w.envFrom = &choiceStep{at: at{w: w, step: stepEnvFrom}}
	w.isolate = &choiceStep{at: at{w: w, step: stepIsolation}}
	w.ff = &fastForwardStep{at: at{w: w, step: stepFastForward}}

	return w
}

// form is the script the engine walks: the steps in the order asked, and how the wizard observes, previews, applies and labels a field.
func (w *wizard) form() form.Params {
	return form.Params{
		Script: form.Script{
			Steps:   []form.Step{w.branches, w.from, w.envFrom, w.isolate, w.ff, ifNotExistsStep{at: at{w: w, step: stepIfNotExists}}},
			Observe: w.observe,
			Preview: w.previewPlan,
			Plan:    w.plan,
			Label:   label,
			Apply:   w.apply,
			Stale:   domain.PlanStale,
		},
		DryRun: w.opts.DryRun,
	}
}

// observe reads the questions for the request as answered so far.
func (w *wizard) observe(ctx context.Context) (func(), error) {
	questions, err := w.app.Questions(ctx, w.req)
	return func() { w.q = questions }, err
}

// An at is a step's place in the wizard, which gives every step its label, description, standing and reset.
type at struct {
	w    *wizard
	step step
}

// Label is the step's name in the breadcrumb and the trail.
func (a at) Label() string {
	return labels[a.step]
}

// Description is the help of the field the step answers.
func (a at) Description() string {
	if field := a.w.field(a.step); field != nil {
		return field.Spec().Help
	}

	return ""
}

// Standing is where the step's answer stands on the questions observed last.
func (a at) Standing() form.Standing {
	return a.w.standing(a.step)
}

// Reset returns the step's field to what the flags gave.
func (a at) Reset() {
	a.w.reset(a.step)
}

// branchesStep asks the branches in a text list, each entry checked against the repository as it is typed.
type branchesStep struct {
	at
	list *form.TextList
}

// Open builds the list from the branches answered so far and starts the cursor blinking.
func (s *branchesStep) Open() tea.Cmd {
	s.list = form.NewTextList(s.w.req.Branches, s.w.branchProblem, s.w.branchBadge)
	return textinput.Blink
}

// Field is the text list.
func (s *branchesStep) Field() form.Field {
	return s.list
}

// Answer records the entries as the request's branches.
func (s *branchesStep) Answer() string {
	s.w.req.Branches = s.list.Entries()
	return strings.Join(s.w.req.Branches, ", ")
}

// sourceStep asks the source branch in a list the user can refresh.
type sourceStep struct {
	at
	list *form.SelectList
}

// Open builds the list of the source branches.
func (s *sourceStep) Open() tea.Cmd {
	s.list = s.w.sourceList()
	return nil
}

// Field is the select list.
func (s *sourceStep) Field() form.Field {
	return s.list
}

// Answer records the branch selected as the request's source.
func (s *sourceStep) Answer() string {
	s.w.req.From = s.list.Value()
	return s.w.req.From
}

// Refreshable reports whether the list can be rebuilt now: not while the user types a filter.
func (s *sourceStep) Refreshable() bool {
	return !s.list.Filtering()
}

// Refreshed rebuilds the list from the branches read again, keeping the branch selected.
func (s *sourceStep) Refreshed() {
	kept := s.list.Value()
	s.list = s.w.sourceList()
	s.list.StartOn(kept)
}

// choiceStep asks the env strategy or the isolation, with an "always use this answer" toggle.
type choiceStep struct {
	at
	list *form.SelectList
}

// Open builds the list of the choice.
func (s *choiceStep) Open() tea.Cmd {
	s.list = s.w.choiceList(s.step)
	return nil
}

// Field is the select list.
func (s *choiceStep) Field() form.Field {
	return s.list
}

// Answer records the value selected and whether to remember it.
func (s *choiceStep) Answer() string {
	return s.w.choose(s.step, s.list.Value(), s.list.Remember())
}

// fastForwardStep asks whether to fast-forward the sources behind origin.
type fastForwardStep struct {
	at
	box *form.Confirm
}

// Open builds the confirmation, on the answer so far.
func (s *fastForwardStep) Open() tea.Cmd {
	s.box = form.NewConfirm(s.w.q.FastForward.Value)
	return nil
}

// Field is the confirmation.
func (s *fastForwardStep) Field() form.Field {
	return s.box
}

// Answer records the confirmation as the request's fast-forward.
func (s *fastForwardStep) Answer() string {
	s.w.req.FastForward = new(s.box.Yes())
	return yesNo(s.box.Yes())
}

// ifNotExistsStep is never asked, its flag the only way to set it: it stands in the trail and the recap.
type ifNotExistsStep struct {
	at
}

// Open builds nothing: the step is never asked.
func (ifNotExistsStep) Open() tea.Cmd {
	return nil
}

// Field is nil: the step is never asked.
func (ifNotExistsStep) Field() form.Field {
	return nil
}

// Answer records nothing: the step is never asked.
func (ifNotExistsStep) Answer() string {
	return ""
}

// choose records value as the answer of the choice step s, and whether to remember it; it returns value as the trail shows it.
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

// reset returns the field of step s to what the flags gave.
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
