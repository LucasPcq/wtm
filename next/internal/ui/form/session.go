package form

import (
	"cmp"
	"context"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/LucasPcq/wtm/next/internal/ui/widget"
)

// A Session is a command's form as the wizard walks it: the request as given and as answered so far, the questions observed last, the plan previewed and the outcome of the apply.
// The request is always the one given with the answer of each step still answered applied in step order, so going back before a step forgets its answer without a reset of its own.
type Session[R Request[R], Q Questions, O Outcome] struct {
	ctx     context.Context
	command *Command[R, Q, O]
	app     UseCase[R, Q, O]
	given   R
	opts    domain.RunOptions
	answers map[int]stepAnswer
	req     R
	q       Q
	form    domain.Form // the form of q, or the schema before the first read
	preview *O
	outcome O
	steps   []Step
}

// A stepAnswer is what one step answered: its fields' answers, and the path whose answer it remembers or forgets, if any.
type stepAnswer struct {
	answers  []domain.Answered
	remember string
	on       bool
}

// Session returns a session that asks app the request req as given, under opts (DryRun, CorrelationID and Report; the session sets the surface and the fingerprint), with a step per field of the command's form: the command's own where Special gives one, else the step of the field's kind.
func (c *Command[R, Q, O]) Session(ctx context.Context, app UseCase[R, Q, O], req R, opts domain.RunOptions) *Session[R, Q, O] {
	s := &Session[R, Q, O]{ctx: ctx, command: c, app: app, given: req, req: req, opts: opts, answers: map[int]stepAnswer{}, form: c.form()}

	for at, field := range s.form.Fields {
		var step Step
		if c.Special != nil {
			step = c.Special(s, at, field)
		}
		if step == nil {
			step = fieldStep(s, at, field)
		}
		s.steps = append(s.steps, step)
	}

	return s
}

// Script is what the wizard walks: the steps, how the session observes, previews, draws, labels and applies, and whether it is a dry run.
func (s *Session[R, Q, O]) Script() Script {
	script := Script{
		Steps:   s.steps,
		Observe: s.observe,
		Preview: s.previewPlan,
		Plan:    func(width int, st *output.Styles) string { return s.command.Preview(*s.preview).text(width, st) },
		Label:   s.Label,
		Apply:   s.apply,
		Stale:   domain.PlanStale,
		DryRun:  s.opts.DryRun,
	}
	if s.command.io != nil {
		script.CommandLine = s.command.io.CommandLine
	}
	if s.command.ApplyLabel != nil {
		script.ApplyLabel = func() string { return s.command.ApplyLabel(*s.preview) }
	}
	script.Empty = func() bool { return (*s.preview).Empty() }

	return script
}

// ----------------------------------------------------------------------------
// What a step reads

// Request is the request as answered so far.
func (s *Session[R, Q, O]) Request() R {
	return s.req
}

// Preview is the plan the recap showed, nil before it lands or when it failed.
func (s *Session[R, Q, O]) Preview() *O {
	return s.preview
}

// Questions are the questions observed last.
func (s *Session[R, Q, O]) Questions() Q {
	return s.q
}

// Asked is the place of step at in the session, for a command's own step to embed.
func (s *Session[R, Q, O]) Asked(at int) Asked {
	return Asked{sheet: s, at: at}
}

// TextStep is the step of the text field at, for a command's own step to embed.
func (s *Session[R, Q, O]) TextStep(at int) *TextStep {
	return &TextStep{Asked: s.Asked(at)}
}

// MultiSelectStep is the step of the multi-select field at, for a command's own step to embed.
func (s *Session[R, Q, O]) MultiSelectStep(at int) *MultiSelectStep {
	return &MultiSelectStep{Asked: s.Asked(at)}
}

// Step is the step at, among those built before it: a command's own step finds the one it pairs with.
func (s *Session[R, Q, O]) Step(at int) Step {
	return s.steps[at]
}

// Field is the field of step at, on the questions observed last.
func (s *Session[R, Q, O]) Field(at int) domain.Field {
	return s.form.Fields[at]
}

// Words are the words of the field of step at, on the questions observed last.
func (s *Session[R, Q, O]) Words(at int) Words {
	return s.command.Words(s.q, s.Field(at))
}

// Problems are the problems of the questions observed last that name the field at path or one of its entries.
func (s *Session[R, Q, O]) Problems(path string) []domain.FieldError {
	return domain.ProblemsOf(s.form.Problems, path)
}

// Standing is where the answer of step at stands on the questions observed last: its value as the trail shows it, where it comes from, why it is not asked, whether it is refused, and the flag that gave it, if the command line did.
func (s *Session[R, Q, O]) Standing(at int) Standing {
	field := s.Field(at)
	spec := field.Spec()
	st := Standing{Value: trailOf(field, s.Words(at)), Origin: spec.Origin, Skip: spec.Skip, Reason: s.reason(spec.Skip), Erred: len(s.Problems(spec.Path)) > 0}
	st.Given = len(s.given.Values(spec.Path)) > 0
	if st.Given && !positional(worded{Field: field, Words: s.Words(at)}) {
		st.Flag = s.command.Flags().Spell(spec.Path)
	}

	return st
}

// reason is how the trail words why a step is not asked: the command's own words, else the wizard's, else the code.
func (s *Session[R, Q, O]) reason(skip domain.Skip) string {
	if s.command.Skipped != nil {
		if words := s.command.Skipped(s, skip); words != "" {
			return words
		}
	}

	return cmp.Or(Skipped(skip), string(skip))
}

// Problem is found as a step shows it: under label, followed by the step that gets past it, if any.
func (s *Session[R, Q, O]) Problem(found domain.FieldError, label string) error {
	err := Problem(found, label)
	if found.FollowUp == nil {
		return err
	}

	return &widget.Problem{Message: err.Error(), Next: []output.NextStep{output.FollowUpStep(*found.FollowUp, s.command.io.CommandLine)}}
}

// Label spells a field path with the label of the first field that answers it, keeping an entry's index: branches[1] reads Branches[1].
func (s *Session[R, Q, O]) Label(path string) string {
	return labelOf(s.command.schema(), path)
}

// Check observes the questions of the request as answered so far with answers given too, without keeping them: a step checks what is typed before it answers.
func (s *Session[R, Q, O]) Check(answers ...domain.Answered) (Q, error) {
	trial, err := domain.Fold(s.req, answers)
	if err != nil {
		var none Q
		return none, err
	}

	return s.app.Questions(s.ctx, trial)
}

// ----------------------------------------------------------------------------
// What a step answers

// Answer records answers as those of step at, in place of what it answered before.
func (s *Session[R, Q, O]) Answer(at int, answers ...domain.Answered) {
	s.answers[at] = stepAnswer{answers: answers}
	s.rebuild()
}

// remembering records answers as those of step at, and whether to remember the answer of the field at path.
func (s *Session[R, Q, O]) remembering(at int, path string, on bool, answers ...domain.Answered) {
	s.answers[at] = stepAnswer{answers: answers, remember: path, on: on}
	s.rebuild()
}

// Forget forgets what step at answered, the user having gone back before it.
func (s *Session[R, Q, O]) Forget(at int) {
	delete(s.answers, at)
	s.rebuild()
}

// rebuild answers the request given with each step's answer in step order, then the fields to remember.
func (s *Session[R, Q, O]) rebuild() {
	req := s.given
	remember := slices.Clone(s.given.Values(domain.PathRemember))

	at := make([]int, 0, len(s.answers))
	for step := range s.answers {
		at = append(at, step)
	}
	slices.Sort(at)

	for _, step := range at {
		answered := s.answers[step]
		// A step answers its field with one of the field's own values, which the request always takes.
		req, _ = domain.Fold(req, answered.answers)
		if answered.remember == "" {
			continue
		}

		remember = slices.DeleteFunc(remember, func(p string) bool { return p == answered.remember })
		if answered.on {
			remember = append(remember, answered.remember)
		}
	}

	s.req, _ = req.Answer(domain.PathRemember, remember)
}

// ----------------------------------------------------------------------------
// Observe, preview, apply

// observe reads the questions for the request as answered so far.
func (s *Session[R, Q, O]) observe(ctx context.Context) (func(), error) {
	questions, err := s.app.Questions(ctx, s.req)
	return func() {
		if err == nil {
			s.q, s.form = questions, questions.Form()
		}
	}, err
}

// previewPlan dry-runs the request as answered, for the recap to show the plan its apply will be held to.
func (s *Session[R, Q, O]) previewPlan(ctx context.Context) (func(), error) {
	opts := s.opts
	opts.DryRun, opts.Surface, opts.Report = true, domain.SurfaceTUI, nil
	outcome, err := s.app.Run(ctx, s.req, opts)
	if err != nil {
		return func() { s.preview = nil }, err
	}

	return func() { s.preview = &outcome }, nil
}

// apply runs the request as answered, held to the fingerprint of the plan previewed unless on a dry run; a stale plan leaves the outcome alone, since the wizard starts again.
func (s *Session[R, Q, O]) apply(ctx context.Context) error {
	opts := s.opts
	opts.Surface = domain.SurfaceTUI
	if !opts.DryRun {
		opts.ExpectFingerprint = (*s.preview).PlanFingerprint()
	}

	outcome, err := s.app.Run(ctx, s.req, opts)
	if !domain.PlanStale(err) {
		s.outcome = outcome
	}

	return err
}
