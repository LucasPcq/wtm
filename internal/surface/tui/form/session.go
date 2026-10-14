package form

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
)

const rememberShapeFmt = "the remember field holds a %T"

// session is one run of the form. The facts are observed once and kept while
// the user types: a value is checked by State, which is pure, and the choices
// of a field are fetched once per value of what they depend on.
type session struct {
	ctx      context.Context
	params   Params
	fields   []kernel.FieldSpec
	remember bool
	given    json.RawMessage
	flagged  map[string]bool
	ticked   []string
	answered map[string]bool
	facts    dispatch.Observed
	choices  map[string]kernel.Choices
	decided  []decision
	at       int
	pending  *load
	request  json.RawMessage
	preview  *dispatch.CommandReply
	failure  error
	stale    bool
	width    int
	err      error
	program  func(components.RunWizardParams) (components.WizardModel, error)
	loading  func(components.LoadingParams) error
}

// decision is what the form does with a field once the answers before it are
// known: ask it, offering its value, or settle it and say why in the trail.
type decision struct {
	asked   bool
	settled string
	reason  string
	offered kernel.Value
	origin  kernel.Origin
}

type load struct {
	text string
	run  tea.Cmd
}

func newSession(ctx context.Context, params Params) *session {
	var fields []kernel.FieldSpec
	remember := false
	for _, spec := range params.Entry.Schema() {
		if spec.Path == kernel.RememberPath {
			remember = true
			continue
		}
		fields = append(fields, spec)
	}
	return &session{ctx: ctx, params: params, fields: fields, remember: remember, given: params.Request, choices: map[string]kernel.Choices{}}
}

func (s *session) observe() error {
	var observed dispatch.Observed
	err := s.loading(components.LoadingParams{Message: observingText, Animate: true, Work: func() error {
		var err error
		observed, err = s.params.Entry.Observe(s.ctx)
		return err
	}})
	if err != nil {
		return err
	}
	s.setFacts(observed)
	return nil
}

func (s *session) setFacts(observed dispatch.Observed) {
	s.facts = observed
	s.choices = map[string]kernel.Choices{}
}

// flag notes the fields the command line answered, whose flag the trail
// names, and the answers it asked to remember.
func (s *session) flag() error {
	form, err := s.params.Entry.State(s.fillParams(s.given))
	if err != nil {
		return err
	}
	s.flagged = map[string]bool{}
	for _, state := range form.States {
		if state.Status == kernel.FieldProvided {
			s.flagged[state.Path] = true
		}
	}
	remembered := stateAt(form, kernel.RememberPath).Value
	ticked, ok := remembered.(kernel.List)
	if !ok && remembered != nil {
		return fmt.Errorf(rememberShapeFmt, remembered)
	}
	s.ticked = ticked
	return nil
}

// fillParams is the call of State on request: on --ask, the facts forget the
// remembered answers and each field still carries its own.
func (s *session) fillParams(request json.RawMessage) dispatch.FillParams {
	return dispatch.FillParams{Request: request, Facts: s.facts, IgnoreRemembered: s.params.Options.IgnoreRemembered}
}

// fill runs the wizard and says whether the recap was confirmed.
func (s *session) fill() (bool, error) {
	steps, asked := s.steps()
	if s.err != nil {
		return false, s.err
	}
	if !asked && s.params.Options.DryRun {
		s.request = s.given
		return true, nil
	}
	final, err := s.program(components.RunWizardParams{
		Steps:   steps,
		Stderr:  true,
		OnMsg:   s.handle,
		InitCmd: s.initCmd(),
	})
	if s.err != nil {
		return false, s.err
	}
	if err != nil {
		return false, cancelled(err)
	}
	if s.params.Options.DryRun {
		s.request = s.requestFrom(final.Steps())
		return true, nil
	}
	s.answered = map[string]bool{}
	for i, decided := range s.decided {
		if decided.asked {
			s.answered[s.fields[i].Path] = true
		}
	}
	recap, ok := final.Steps()[len(s.fields)].Model.(components.SelectListModel)
	return ok && recap.Value() == applyValue, nil
}

func (s *session) initCmd() tea.Cmd { return pending }

// steps is a step per field, then the recap; the fields before the first one
// asked are settled up front, since only the flags decide them.
func (s *session) steps() ([]components.Step, bool) {
	s.decided = make([]decision, len(s.fields))
	steps := make([]components.Step, 0, len(s.fields)+1)
	for i := range s.fields {
		steps = append(steps, s.step(i))
	}
	if !s.params.Options.DryRun {
		steps = append(steps, s.recapStep())
	}
	form, ok := s.stateOf(s.given)
	if !ok {
		return steps, true
	}
	for i := range s.fields {
		decided := s.decide(i, form)
		if decided.asked {
			return steps, true
		}
		s.decided[i] = decided
		steps[i].Settled = decided.settled
		steps[i].Ruled = decided.reason
	}
	return steps, false
}

func (s *session) step(i int) components.Step {
	spec := s.fields[i]
	return components.Step{
		Name:           labelOf(spec),
		Model:          placeholder(spec),
		Build:          func(prev []components.Step) any { return s.build(i, prev) },
		AutoSkip:       func(components.WizardModel) bool { return !s.decided[i].asked },
		SettledSummary: func() string { return s.decided[i].settled },
		SkipReason:     func() string { return s.decided[i].reason },
		Summary:        func(model any) string { return s.summary(i, model) },
		CanRefresh:     spec.Choices == kernel.ChoicesSearch,
		OnEnter:        func([]components.Step) tea.Cmd { return pending },
	}
}

func (s *session) build(i int, prev []components.Step) any {
	request := s.requestFrom(prev)
	form, ok := s.stateOf(request)
	if !ok {
		return placeholder(s.fields[i])
	}
	s.decided[i] = s.decide(i, form)
	if !s.decided[i].asked {
		return placeholder(s.fields[i])
	}
	s.at = i
	return s.model(fieldParams{Index: i, Request: request, Form: form})
}

// decide asks a field the flags left out, or one a default only proposes; it
// settles what a flag, a remembered answer or an answer before the facts
// changed gave, and leaves out a skipped one with the reason.
func (s *session) decide(i int, form kernel.Form[json.RawMessage]) decision {
	spec := s.fields[i]
	state := stateAt(form, spec.Path)
	switch {
	case state.Status == kernel.FieldSkipped:
		return decision{reason: s.skipReason(spec, state)}
	case len(state.Errors) > 0:
	case state.Status == kernel.FieldProvided || s.answered[spec.Path]:
		return decision{settled: s.settledBy(spec, state)}
	case state.Status == kernel.FieldDefaulted && state.Origin == kernel.OriginRemembered:
		return decision{settled: valueText(state.Value) + domain.RecapRememberedSuffix}
	}
	return decision{asked: true, offered: state.Value, origin: state.Origin}
}

// settledBy is never empty: an empty Settled would not settle the step.
func (s *session) settledBy(spec kernel.FieldSpec, state kernel.FieldState) string {
	shown := valueText(state.Value)
	if shown == "" {
		shown = domain.SummaryNone
	}
	if !s.flagged[spec.Path] {
		return shown
	}
	return trailOf(shown, s.params.Names(spec.Path))
}

func (s *session) skipReason(spec kernel.FieldSpec, state kernel.FieldState) string {
	if state.SkipReason == "" {
		return notNeeded
	}
	return s.message(state.SkipReason, kernel.Params{kernel.ParamPath: labelOf(spec)})
}

// requestFrom is the request the flags gave, with the answer of every step
// asked before and the rememberable ones the user ticked.
func (s *session) requestFrom(prev []components.Step) json.RawMessage {
	request := s.given
	var remembered []string
	offered := false
	for i, step := range prev {
		if i >= len(s.fields) || !s.decided[i].asked {
			continue
		}
		spec := s.fields[i]
		request = s.set(request, spec.Path, valueOf(step.Model))
		list, isList := step.Model.(components.SelectListModel)
		if !isList || !spec.Rememberable {
			continue
		}
		offered = true
		if list.Remembering() {
			remembered = append(remembered, spec.Path)
		}
	}
	if s.remember && offered {
		request = s.set(request, kernel.RememberPath, kernel.List(remembered))
	}
	return request
}

func (s *session) set(request json.RawMessage, path string, value kernel.Value) json.RawMessage {
	updated, err := s.params.Entry.Set(dispatch.SetParams{Request: request, Path: path, Value: value})
	if err != nil {
		s.fail(err)
		return request
	}
	return updated
}

func (s *session) stateOf(request json.RawMessage) (kernel.Form[json.RawMessage], bool) {
	form, err := s.params.Entry.State(s.fillParams(request))
	if err != nil {
		s.fail(err)
		return form, false
	}
	return form, true
}

// fail keeps the first failure of a call to the engine: the wizard quits on
// the next message and the run returns it.
func (s *session) fail(err error) {
	if s.err == nil {
		s.err = err
	}
}

func stateAt(form kernel.Form[json.RawMessage], path string) kernel.FieldState {
	index := slices.IndexFunc(form.States, func(state kernel.FieldState) bool { return state.Path == path })
	if index < 0 {
		return kernel.FieldState{Path: path, Status: kernel.FieldMissing}
	}
	return form.States[index]
}

// check is the first error State finds on the field once value is in it.
func (s *session) check(params checkParams) error {
	path := s.fields[params.Index].Path
	if params.Entry != "" {
		path = params.Entry
	}
	request, err := s.params.Entry.Set(dispatch.SetParams{Request: params.Request, Path: s.fields[params.Index].Path, Value: params.Value})
	if err != nil {
		return err
	}
	form, err := s.params.Entry.State(s.fillParams(request))
	if err != nil {
		return err
	}
	for _, problem := range stateAt(form, s.fields[params.Index].Path).Errors {
		if params.Entry == "" || problem.Path == path {
			return errors.New(s.fieldMessage(problem))
		}
	}
	return nil
}

type checkParams struct {
	Index   int
	Request json.RawMessage
	Value   kernel.Value
	// Entry narrows the check to the errors of one entry of a list.
	Entry string
}
