package form

import (
	"errors"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/dispatch"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
)

func (s *session) recapStep() components.Step {
	return components.Step{
		Name:    recapName,
		Recap:   true,
		Model:   recapList(recapParams{Title: recapTitle}),
		Build:   s.buildRecap,
		OnEnter: func([]components.Step) tea.Cmd { return pending },
		Summary: components.SelectSummary,
	}
}

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
	return recapList(recapParams{Title: recapTitle, Description: planText})
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
	s.redraw(w)
}

func (s *session) resized(w *components.WizardModel, m tea.WindowSizeMsg) {
	s.width = m.Width
	if s.preview != nil || s.failure != nil {
		s.redraw(w)
	}
}

func (s *session) redraw(w *components.WizardModel) {
	w.UpdateStepModel(len(s.fields), func(any) any { return s.recapModel() })
}

func (s *session) recapModel() components.SelectListModel {
	if s.preview == nil {
		return recapList(recapParams{Title: failedTitle, Description: s.describe(s.failure)})
	}
	rows := s.params.Plan(s.preview.Plan.Detail)
	return recapList(recapParams{Title: recapTitle, Description: s.recapText(rows), Apply: len(rows) > 0})
}

func (s *session) recapText(rows []styles.PlanRow) string {
	warnings := make([]string, 0, len(s.preview.Plan.Warnings))
	for _, warning := range s.preview.Plan.Warnings {
		warnings = append(warnings, s.message(warning.Code, warning.Params))
	}
	plan := styles.RenderPlan(styles.PlanParams{Rows: rows, Warnings: warnings, Width: styles.RecapBodyWidth(styles.IntroParams{Width: s.width})})
	if !s.stale {
		return plan
	}
	return styles.Warning.Render(domain.GlyphAttention) + " " + s.message(kernel.CodePlanStale, nil) + "\n\n" + plan
}

type recapParams struct {
	Title       string
	Description string
	Apply       bool
}

// While the plan loads the recap offers no option: enter cannot apply a plan nobody has seen.
func recapList(params recapParams) components.SelectListModel {
	var items []components.SelectItem
	if params.Apply {
		items = append(items, components.SelectItem{Label: applyLabel, Value: applyValue}, components.SelectItem{Separator: true})
	}
	if params.Description != planText {
		items = append(items, components.SelectItem{Label: domain.WizardCancelLabel, Value: domain.WizardCancelValue})
	}
	return components.NewSelectList(components.NewSelectListParams{Title: params.Title, Description: params.Description, Items: items})
}

func (s *session) describe(err error) string {
	failed := styles.DangerText.Render(domain.GlyphFailure) + " "
	var failure kernel.Error
	if !errors.As(err, &failure) {
		return failed + err.Error()
	}
	base := failure.Base()
	lines := []string{failed + s.message(base.Code, base.Params)}
	switch detailed := failure.(type) {
	case *kernel.InvalidError:
		for _, field := range detailed.Fields {
			lines = append(lines, detailIndent+s.fieldMessage(field))
		}
	case *kernel.RefusedError:
		for _, blocker := range detailed.Blockers {
			lines = append(lines, detailIndent+s.message(blocker.Code, blocker.Params))
		}
	case *kernel.Failure:
	}
	return strings.Join(lines, "\n")
}
