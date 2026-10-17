package cli

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	applyLabel  = "Yes, apply"
	cancelLabel = "No, cancel"
	detailHang  = "    "
)

func (w *wizard) previewing() tea.Cmd {
	w.preview, w.failure = nil, nil
	w.recap = form.NewSelectList(form.SelectParams{})
	req := w.req
	return tea.Batch(w.load(planText), func() tea.Msg {
		outcome, err := w.app.Run(w.ctx, req, operation.Options{DryRun: true, Surface: surfaceTUI})
		return previewMsg{outcome: outcome, err: err}
	})
}

func (w *wizard) previewed(m previewMsg) {
	w.loading = ""
	options := []form.Option{{Label: cancelLabel, Value: cancelValue}}
	if failure := asError(m.err); failure != nil {
		w.failure = failure
	} else {
		w.preview = &m.outcome
		options = append([]form.Option{{Label: applyLabel, Value: applyValue}, {Separator: true}}, options...)
	}
	w.recap = form.NewSelectList(form.SelectParams{Options: options})
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
	plan := planTextOf(planTextParams{Plan: w.preview.Plan, Warnings: w.preview.Warnings, Width: w.recapWidth() - form.RecapBox.GetHorizontalPadding()})
	if !w.stale {
		return plan
	}
	return output.Warning.Render(output.GlyphAttention) + " " + staleText + "\n\n" + plan
}

func (w *wizard) failureText() string {
	first := w.failure.Error()
	if len(w.failure.Fields) > 0 {
		first = w.failure.Message
	}
	lines := []string{output.Danger.Render(output.GlyphFailure) + " " + first}
	for _, found := range w.failure.Fields {
		lines = append(lines, detailHang+found.Message(label(found.Path)))
	}
	return strings.Join(lines, "\n")
}

func (w *wizard) recapWidth() int {
	return max(w.size.Width-8, 24)
}

func (w *wizard) review() string {
	return form.RecapBox.Width(w.recapWidth()).Render(form.Accent.Render(reviewTitle) + "\n\n" + w.recapBody())
}
