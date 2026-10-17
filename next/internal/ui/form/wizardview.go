package form

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

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

func (w *Wizard) breadcrumb() string {
	title := w.recapTitle()
	if !w.atRecap() {
		title = w.script.Steps[w.current].Label()
	}
	muted := w.styles.Muted
	return muted.Render(fmt.Sprintf("%sStep %d/%d", output.Indent, w.current+1, w.last())) + muted.Render(HelpSeparator) + w.styles.accent.Render(title)
}

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

func (w *Wizard) canGoBack() bool {
	for s := w.current - 1; s >= 0; s-- {
		if w.marks[s].asked {
			return true
		}
	}
	return false
}
