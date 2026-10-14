package form

import (
	"encoding/json"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
)

type decision struct {
	asked   bool
	settled string
	reason  string
	offered kernel.Value
	origin  kernel.Origin
}

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
