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

// previewing computes the plan off the event loop, the recap offering no choice until it lands.
func (w *Wizard) previewing() tea.Cmd {
	w.planned, w.failure = false, nil
	w.recap = NewSelectList(SelectParams{})
	return tea.Batch(w.load(planText), func() tea.Msg {
		apply, err := w.script.Preview(w.ctx)
		return previewMsg{apply: apply, err: err}
	})
}

// previewed installs the plan a preview computed and offers to apply it, or, when the preview failed, keeps its failure and offers only to cancel.
func (w *Wizard) previewed(m previewMsg) {
	w.loading = ""
	m.apply()

	options := []Option{{Label: cancelLabel, Value: cancelValue}}
	if failure := domain.AsError(domain.AsErrorParams{Err: m.err}); failure != nil {
		w.failure = failure
	} else {
		w.planned = true
		options = append([]Option{{Label: w.applyLabel(), Value: applyValue}, {Separator: true}}, options...)
	}

	w.recap = NewSelectList(SelectParams{Options: options})
}

func (w *Wizard) applyLabel() string {
	if w.script.ApplyLabel == nil {
		return applyLabel
	}
	return w.script.ApplyLabel()
}

// confirmed ends the wizard on the recap's choice.
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

// recapBody draws the plan, warned stale when it changed since the plan the user confirmed, or the preview's failure.
func (w *Wizard) recapBody() string {
	switch {
	case w.failure != nil:
		return w.failureText()
	case !w.planned:
		return planText
	}

	plan := w.script.Plan(w.recapWidth()-w.styles.recapBox.GetHorizontalPadding(), w.styles.Styles)
	if !w.stale {
		return plan
	}
	return w.styles.Warning.Render(output.GlyphAttention) + " " + staleText + "\n\n" + plan
}

// failureText draws the preview's failure: its message, then each field it refuses under the label of the step that sets it.
func (w *Wizard) failureText() string {
	first := w.failure.Error()
	if len(w.failure.Fields) > 0 {
		first = w.failure.Message
	}

	lines := []string{w.styles.Danger.Render(output.GlyphFailure) + " " + first}
	for _, found := range w.failure.Fields {
		lines = append(lines, detailHang+found.Message(w.script.Label(found.Path)))
	}
	return strings.Join(lines, "\n")
}

func (w *Wizard) recapWidth() int {
	return max(w.size.Width-8, 24)
}

// review draws the recap's box: its title and its body.
func (w *Wizard) review() string {
	return w.styles.recapBox.Width(w.recapWidth()).Render(w.styles.accent.Render(reviewTitle) + "\n\n" + w.recapBody())
}
