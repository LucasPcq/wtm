package form

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	planText    = "Computing the plan…"
	recapTitle  = "Apply this plan?"
	failedTitle = "Cannot apply this plan"
	reviewTitle = "Review & confirm"
	staleText   = "the facts changed since the preview"
	applyValue  = "apply"
	cancelValue = "cancel"
	applyLabel  = "Yes, apply"
	cancelLabel = "No, cancel"
	detailHang  = "    "
)

func (w *Wizard) previewing() tea.Cmd {
	w.planned, w.failure = false, nil
	w.recap = NewSelectList(SelectParams{})
	return tea.Batch(w.load(planText), func() tea.Msg {
		return previewMsg{err: w.script.Preview(w.ctx)}
	})
}

func (w *Wizard) previewed(m previewMsg) {
	w.loading = ""
	options := []Option{{Label: cancelLabel, Value: cancelValue}}
	if failure := domain.AsError(domain.AsErrorParams{Err: m.err}); failure != nil {
		w.failure = failure
	} else {
		w.planned = true
		options = append([]Option{{Label: applyLabel, Value: applyValue}, {Separator: true}}, options...)
	}
	w.recap = NewSelectList(SelectParams{Options: options})
}

func (w *Wizard) confirmed() tea.Cmd {
	if w.recap.Value() == applyValue {
		w.ending = Finished
	} else {
		w.ending = Cancelled
	}
	return tea.Quit
}

func (w *Wizard) recapTitle() string {
	if w.failure != nil {
		return failedTitle
	}
	return recapTitle
}

func (w *Wizard) recapBody() string {
	switch {
	case w.failure != nil:
		return w.failureText()
	case !w.planned:
		return planText
	}
	plan := w.script.Plan(w.recapWidth() - RecapBox.GetHorizontalPadding())
	if !w.stale {
		return plan
	}
	return output.Warning.Render(output.GlyphAttention) + " " + staleText + "\n\n" + plan
}

func (w *Wizard) failureText() string {
	first := w.failure.Error()
	if len(w.failure.Fields) > 0 {
		first = w.failure.Message
	}
	lines := []string{output.Danger.Render(output.GlyphFailure) + " " + first}
	for _, found := range w.failure.Fields {
		lines = append(lines, detailHang+found.Message(w.script.Label(found.Path)))
	}
	return strings.Join(lines, "\n")
}

func (w *Wizard) recapWidth() int {
	return max(w.size.Width-8, 24)
}

func (w *Wizard) review() string {
	return RecapBox.Width(w.recapWidth()).Render(Accent.Render(reviewTitle) + "\n\n" + w.recapBody())
}
