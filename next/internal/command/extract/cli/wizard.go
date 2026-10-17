package cli

import (
	"context"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/ui/form"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// A wizard is extract's form over the ui/form engine: the request as the flags gave it and as answered so far, whether the target was picked as a worktree to create, the questions observed last, the plan previewed, the outcome of the apply, and the steps that are asked.
type wizard struct {
	ctx       context.Context
	app       useCase
	given     extract.Request // as the flags gave it: a step given a flag is not asked, and a step gone back before returns to it
	req       extract.Request
	newTarget bool
	opts      domain.RunOptions
	q         extract.Questions
	preview   *extract.Outcome
	outcome   extract.Outcome
	steps     []form.Step
}

type wizardParams struct {
	App     useCase
	Request extract.Request
	Options domain.RunOptions
}

// newWizard builds the wizard of params.Request, each step knowing its place in it.
func newWizard(ctx context.Context, params wizardParams) *wizard {
	w := &wizard{ctx: ctx, app: params.App, given: params.Request, req: params.Request, opts: params.Options}

	w.steps = []form.Step{
		&selectStep{at: at{w: w, step: stepSource}},
		&filesStep{at: at{w: w, step: stepFiles}},
		&selectStep{at: at{w: w, step: stepTarget}},
		&branchStep{at: at{w: w, step: stepBranch}},
		&fromStep{selectStep{at: at{w: w, step: stepFrom}}},
		&selectStep{at: at{w: w, step: stepIsolation}},
		&selectStep{at: at{w: w, step: stepFastForward}},
		&selectStep{at: at{w: w, step: stepMode}},
		&selectStep{at: at{w: w, step: stepOnConflict}},
	}

	return w
}

// form is the script the engine walks: the steps in the order asked, and how the wizard observes, previews, applies and labels a field.
func (w *wizard) form() form.Params {
	return form.Params{
		Script: form.Script{
			Steps:      w.steps,
			Observe:    w.observe,
			Preview:    w.previewPlan,
			Plan:       w.plan,
			Label:      label,
			ApplyLabel: w.applyLabel,
			Apply:      w.apply,
			Stale:      domain.PlanStale,
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
	return stepLabels[a.step]
}

// Description is what the step's answer decides.
func (a at) Description() string {
	return a.w.description(a.step)
}

// Standing is where the step's answer stands on the questions observed last.
func (a at) Standing() form.Standing {
	return a.w.standing(a.step)
}

// Reset returns the step's field to what the flags gave.
func (a at) Reset() {
	a.w.reset(a.step)
}

// selectStep asks one of a list of options: the source, the target, the source branch, the isolation, the source update, the mode or what to do on conflict.
type selectStep struct {
	at
	list *form.SelectList
}

// Open builds the list of the step's options.
func (s *selectStep) Open() tea.Cmd {
	s.list = s.w.list(s.step)
	return nil
}

// Field is the select list.
func (s *selectStep) Field() form.Field {
	return s.list
}

// Answer records the option selected, and whether to remember it.
func (s *selectStep) Answer() string {
	return s.w.pick(s.step, s.list.Value(), s.list.Remember())
}

// fromStep asks the source branch of a created target in a list the user can refresh.
type fromStep struct {
	selectStep
}

// Refreshable reports whether the list can be rebuilt now: not while the user types a filter.
func (s *fromStep) Refreshable() bool {
	return !s.list.Filtering()
}

// Refreshed rebuilds the list from the branches read again, keeping the branch selected.
func (s *fromStep) Refreshed() {
	kept := s.list.Value()
	s.list = s.w.list(s.step)
	s.list.StartOn(kept)
}

// filesStep asks the changes to extract in a multi-select, none picked at first.
type filesStep struct {
	at
	picks *form.MultiSelect
}

// Open builds the multi-select of the source's changes, those answered so far picked.
func (s *filesStep) Open() tea.Cmd {
	s.picks = s.w.files()
	return nil
}

// Field is the multi-select.
func (s *filesStep) Field() form.Field {
	return s.picks
}

// Answer records the changes picked as the request's files.
func (s *filesStep) Answer() string {
	s.w.req.Files = s.picks.Values()
	return fileCount(len(s.w.req.Files))
}

// branchStep asks the branch of a created target, checked against the repository once typed.
type branchStep struct {
	at
	text *form.Text
}

// Open builds the text field on the branch answered so far and starts the cursor blinking.
func (s *branchStep) Open() tea.Cmd {
	s.text = form.NewText(s.w.req.To, s.w.branchProblem)
	return textinput.Blink
}

// Field is the text field.
func (s *branchStep) Field() form.Field {
	return s.text
}

// Answer records the branch typed as the request's target.
func (s *branchStep) Answer() string {
	s.w.req.To = s.text.Value()
	return s.w.req.To
}

// pick records value as the answer of the select step s, and whether to remember it; it returns value as the trail shows it.
func (w *wizard) pick(s step, value string, remember bool) string {
	switch s {
	case stepSource:
		w.req.Source = value
	case stepTarget:
		return w.target(value)
	case stepFrom:
		w.req.From = value
	case stepIsolation:
		w.req.Isolation = project.Isolation(value)
		w.remember(stepPaths[s], remember)
	case stepFastForward:
		// The options are the field's own, true and false, which Answer always takes.
		w.req, _ = w.req.Answer(stepPaths[s], []string{value})
		w.remember(stepPaths[s], remember)
		return yesNo(w.req.FastForward != nil && *w.req.FastForward)
	case stepMode:
		w.req, _ = w.req.Answer(stepPaths[s], []string{value})
		return mode(w.req.Keep != nil && *w.req.Keep)
	case stepOnConflict:
		w.req.OnConflict = extract.OnConflict(value)
	case stepFiles, stepBranch, stepRecap, stepCount:
	}

	return value
}

// target records the target picked: an existing worktree, or one to create, whose branch the next step asks.
func (w *wizard) target(value string) string {
	if value != createTarget {
		w.newTarget, w.req.To = false, value
		return value
	}

	if !w.creating() {
		w.req.To = ""
	}
	w.newTarget = true
	return newWorktree
}

// remember records whether the answer of the field at path is to be remembered.
func (w *wizard) remember(path string, on bool) {
	w.req.Remember = slices.DeleteFunc(slices.Clone(w.req.Remember), func(p string) bool { return p == path })
	if on {
		w.req.Remember = append(w.req.Remember, path)
	}
}

// reset returns the field of step s to what the flags gave.
func (w *wizard) reset(s step) {
	switch s {
	case stepSource:
		w.req.Source = w.given.Source
	case stepFiles:
		w.req.Files = w.given.Files
	case stepTarget:
		w.req.To, w.newTarget = w.given.To, false
	case stepBranch:
		if w.creating() {
			w.req.To = w.given.To
		}
	case stepFrom:
		w.req.From = w.given.From
	case stepIsolation:
		w.req.Isolation = w.given.Isolation
		w.remember(stepPaths[s], slices.Contains(w.given.Remember, stepPaths[s]))
	case stepFastForward:
		w.req.FastForward = w.given.FastForward
		w.remember(stepPaths[s], slices.Contains(w.given.Remember, stepPaths[s]))
	case stepMode:
		w.req.Keep = w.given.Keep
	case stepOnConflict:
		w.req.OnConflict = w.given.OnConflict
	case stepRecap, stepCount:
	}
}

// creating reports whether the target is a worktree to create: picked as one, or a branch that no worktree other than the source holds.
func (w *wizard) creating() bool {
	to := w.req.To
	held := slices.ContainsFunc(w.q.To.Options, func(o domain.Option) bool { return o.Value == to })
	return w.newTarget || to != "" && to != w.req.Source && !held
}
