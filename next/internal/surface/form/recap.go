package form

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/app"
	"github.com/LucasPcq/wtm/next/internal/surface/output"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	applyLabel  = "Yes, apply"
	cancelLabel = "No, cancel"
	detailHang  = "    "
)

func (w *wizard) previewing() tea.Cmd {
	w.preview, w.failure = nil, nil
	w.recap = newSelectList(selectParams{})
	req := w.req
	return tea.Batch(w.load(planText), func() tea.Msg {
		outcome, err := w.app.Create(w.ctx, app.CreateParams{Request: req, DryRun: true, Surface: surface})
		return previewMsg{outcome: outcome, err: err}
	})
}

func (w *wizard) previewed(m previewMsg) {
	w.loading = ""
	options := []option{{Label: cancelLabel, Value: cancelValue}}
	if failure := asError(m.err); failure != nil {
		w.failure = failure
	} else {
		w.preview = &m.outcome
		options = append([]option{{Label: applyLabel, Value: applyValue}, {Separator: true}}, options...)
	}
	w.recap = newSelectList(selectParams{Options: options})
}

func (w *wizard) recapTitle() string {
	if w.failure != nil {
		return failedTitle
	}
	return recapTitle
}

func (w *wizard) recapBody() string {
	switch {
	case w.failure != nil:
		return w.failureText()
	case w.preview == nil:
		return planText
	}
	plan := output.PlanText(output.PlanTextParams{Plan: w.preview.Plan, Warnings: w.preview.Warnings, Width: w.recapWidth() - recapBox.GetHorizontalPadding()})
	if !w.stale {
		return plan
	}
	return warning.Render(glyphAttention) + " " + staleText + "\n\n" + plan
}

func (w *wizard) failureText() string {
	first := w.failure.Error()
	if len(w.failure.Fields) > 0 {
		first = w.failure.Message
	}
	lines := []string{danger.Render(glyphFailure) + " " + first}
	for _, found := range w.failure.Fields {
		lines = append(lines, detailHang+found.Message(label(found.Path)))
	}
	return strings.Join(lines, "\n")
}

func (w *wizard) recapWidth() int {
	return max(w.size.Width-8, 24)
}

func (w *wizard) review() string {
	return recapBox.Width(w.recapWidth()).Render(accent.Render(reviewTitle) + "\n\n" + w.recapBody())
}
