package form

import (
	"encoding/json"
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
)

// recapStep is the plan of a dry run on the request filled so far: nothing
// is applied before the user has seen what it does.
func (s *session) recapStep() components.Step {
	return components.Step{
		Name:    recapName,
		Recap:   true,
		Model:   recapList(recapParams{}),
		Build:   s.buildRecap,
		OnEnter: func([]components.Step) tea.Cmd { return pending },
		Summary: components.SelectSummary,
	}
}

// buildRecap shows no option until the plan arrives, so enter cannot apply a
// plan nobody has seen.
func (s *session) buildRecap(prev []components.Step) any {
	request := s.requestFrom(prev)
	s.request, s.preview, s.failure = request, nil, nil
	s.pending = &load{text: planText, run: func() tea.Msg {
		reply, err := s.params.Entry.Call(s.ctx, dispatch.Call{Request: request, Options: dispatch.Options{
			DryRun:           true,
			IgnoreRemembered: s.params.Options.IgnoreRemembered,
			Surface:          dispatch.SurfaceTUI,
			CorrelationID:    s.params.Options.CorrelationID,
		}})
		return previewMsg{reply: reply, err: err}
	}}
	return recapList(recapParams{Description: planText})
}

func (s *session) previewed(w *components.WizardModel, m previewMsg) {
	w.SetLoading(false)
	reply, isCommand := m.reply.(dispatch.CommandReply)
	switch {
	case m.err != nil:
		s.failure = m.err
	case !isCommand:
		s.failure = errors.New(notACommand)
	default:
		s.preview = &reply
	}
	w.UpdateStepModel(len(s.fields), func(any) any { return s.recapModel() })
}

// recapModel is the plan as the command shows it, with its warnings, or why
// there is none; only a plan can be applied.
func (s *session) recapModel() components.SelectListModel {
	return recapList(recapParams{Description: s.recapText(), Apply: s.preview != nil})
}

func (s *session) recapText() string {
	if s.preview == nil {
		return s.describe(s.failure)
	}
	lines := []string{s.plan(s.preview.Plan.Detail)}
	for _, warning := range s.preview.Plan.Warnings {
		lines = append(lines, domain.GlyphAttention+" "+message(warning.Code, warning.Params))
	}
	return strings.Join(lines, "\n")
}

func (s *session) plan(detail json.RawMessage) string {
	if s.params.Plan == nil {
		return string(detail)
	}
	return strings.TrimRight(s.params.Plan(detail), "\n")
}

type recapParams struct {
	Description string
	Apply       bool
}

func recapList(params recapParams) components.SelectListModel {
	var items []components.SelectItem
	if params.Apply {
		items = append(items, components.SelectItem{Label: applyLabel, Value: applyValue}, components.SelectItem{Separator: true})
	}
	if params.Description != planText {
		items = append(items, components.SelectItem{Label: domain.WizardCancelLabel, Value: domain.WizardCancelValue})
	}
	return components.NewSelectList(components.NewSelectListParams{Title: recapTitle, Description: params.Description, Items: items})
}

// describe is a failure as the recap says it: its message, then each field or
// blocker it names.
func (s *session) describe(err error) string {
	var failure kernel.Error
	if !errors.As(err, &failure) {
		return err.Error()
	}
	base := failure.Base()
	lines := []string{message(base.Code, base.Params)}
	switch detailed := failure.(type) {
	case *kernel.InvalidError:
		for _, field := range detailed.Fields {
			lines = append(lines, s.fieldMessage(field))
		}
	case *kernel.RefusedError:
		for _, blocker := range detailed.Blockers {
			lines = append(lines, message(blocker.Code, blocker.Params))
		}
	case *kernel.Failure:
	}
	return strings.Join(lines, "\n")
}
