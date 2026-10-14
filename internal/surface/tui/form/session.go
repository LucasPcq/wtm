package form

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
)

const rememberShapeFmt = "the remember field holds a %T"

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

func (s *session) fillParams(request json.RawMessage) dispatch.FillParams {
	return dispatch.FillParams{Request: request, Facts: s.facts, IgnoreRemembered: s.params.Options.IgnoreRemembered}
}

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

func (s *session) stateOf(request json.RawMessage) (kernel.Form[json.RawMessage], bool) {
	form, err := s.params.Entry.State(s.fillParams(request))
	if err != nil {
		s.fail(err)
		return form, false
	}
	return form, true
}

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
	Entry   string
}
