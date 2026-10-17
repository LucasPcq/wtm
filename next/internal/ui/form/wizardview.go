package form

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// View draws the wizard: the breadcrumb, the trail of the steps settled, the step asked or the recap, and the help line, the field taking the rows left; only the spinner before the first read lands, and nothing once the wizard has ended.
func (w *Wizard) View() string {
	switch {
	case w.ending != Asking:
		return ""
	case !w.loaded:
		return "\n" + w.status() + "\n"
	}

	head, tail := w.head(), w.tail()
	body := Size{Width: w.size.Width}
	if w.size.Height > 0 {
		body.Height = max(minListRows, w.size.Height-strings.Count(head+tail, "\n"))
	}

	return head + w.field().View(body, w.styles) + tail
}

func (w *Wizard) status() string {
	return w.styles.statusBox.Render(w.spinner.View() + " " + w.styles.Muted.Render(w.loading))
}

// head draws what stands above the field: the breadcrumb, the spinner while loading, the trail, then the recap or the step's description.
func (w *Wizard) head() string {
	var b strings.Builder
	b.WriteString("\n" + w.breadcrumb() + "\n\n")

	if w.loading != "" {
		b.WriteString(w.status() + "\n\n")
	}
	if summaries := w.summaries(); summaries != "" {
		b.WriteString(summaries + "\n")
	}
	if w.current > 0 {
		b.WriteString("\n")
	}

	if w.atRecap() {
		return b.String() + w.review() + "\n\n"
	}

	b.WriteString(w.styles.Muted.Render(Indented(Wrap(w.script.Steps[w.current].Description(), w.size.Width-len(output.Indent)-1))) + "\n\n")
	return b.String()
}

// breadcrumb draws "Step n/m" and the title of the step asked.
func (w *Wizard) breadcrumb() string {
	title := w.recapTitle()
	if !w.atRecap() {
		title = w.script.Steps[w.current].Label()
	}

	muted := w.styles.Muted
	return muted.Render(fmt.Sprintf("%sStep %d/%d", output.Indent, w.current+1, w.last())) + muted.Render(HelpSeparator) + w.styles.accent.Render(title)
}

// summaries draws the trail: a line per step before the one asked, its answer and where it comes from, or why it does not apply.
func (w *Wizard) summaries() string {
	var lines []string
	for s := range w.current {
		m := w.marks[s]
		label := w.script.Steps[s].Label()
		switch {
		case m.ruled != "":
			lines = append(lines, w.styles.Muted.Render(output.Indent+"· "+label+" — "+m.ruled))
		default:
			lines = append(lines, w.styles.summary.Render(output.Indent+"✓ "+label+": "+m.line))
		}
	}
	return strings.Join(lines, "\n")
}

func (w *Wizard) tail() string {
	return "\n\n" + w.styles.Muted.Render(w.helpLine()) + "\n"
}

// helpLine draws the keys the step asked answers to, or the field's own line while it is in a mode of its own.
func (w *Wizard) helpLine() string {
	h := w.field().Help()
	if h.Modal != "" {
		return h.Modal
	}

	var parts []string
	if !h.Rowless {
		parts = append(parts, "↑↓ navigate")
	}
	parts = append(parts, h.Actions...)
	if _, ok := w.refresher(); ok {
		parts = append(parts, "r refresh")
	}
	parts = append(parts, "enter confirm")

	if w.canGoBack() {
		parts = append(parts, "esc back")
	} else {
		parts = append(parts, "esc cancel")
	}

	return output.Indent + strings.Join(parts, HelpSeparator)
}

// canGoBack reports whether a step was asked before the current one, so esc goes back rather than cancels.
func (w *Wizard) canGoBack() bool {
	for s := w.current - 1; s >= 0; s-- {
		if w.marks[s].asked {
			return true
		}
	}
	return false
}
