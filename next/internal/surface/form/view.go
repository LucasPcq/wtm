package form

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/surface/output"
	"github.com/LucasPcq/wtm/next/internal/wtm"
)

func (w *wizard) View() string {
	switch {
	case w.finished || w.aborted:
		return ""
	case !w.loaded:
		return "\n" + w.status() + "\n"
	}
	head, tail := w.head(), w.tail()
	body := size{Width: w.size.Width}
	if w.size.Height > 0 {
		body.Height = max(minListRows, w.size.Height-strings.Count(head+tail, "\n"))
	}
	return head + w.field().view(body) + tail
}

func (w *wizard) status() string {
	return statusBox.Render(w.spinner.View() + " " + muted.Render(w.loading))
}

func (w *wizard) head() string {
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
	if w.current == stepRecap {
		return b.String() + w.review() + "\n\n"
	}
	b.WriteString(muted.Render(indented(wrap(output.Describe(paths[w.current]), w.size.Width-len(indent)-1))) + "\n\n")
	return b.String()
}

func (w *wizard) breadcrumb() string {
	title := labels[w.current]
	if w.current == stepRecap {
		title = w.recapTitle()
	}
	return muted.Render(fmt.Sprintf("%sStep %d/%d", indent, w.current+1, w.last())) + muted.Render(helpSeparator) + accent.Render(title)
}

func (w *wizard) summaries() string {
	var lines []string
	for s := range w.current {
		m := w.marks[s]
		switch {
		case m.ruled != "":
			lines = append(lines, muted.Render(indent+"· "+labels[s]+" — "+m.ruled))
		default:
			lines = append(lines, summary.Render(indent+"✓ "+labels[s]+": "+m.line))
		}
	}
	return strings.Join(lines, "\n")
}

func (w *wizard) tail() string {
	return "\n\n" + muted.Render(w.helpLine()) + "\n"
}

func (w *wizard) helpLine() string {
	h := w.field().help()
	if h.Modal != "" {
		return h.Modal
	}
	var parts []string
	if !h.Rowless {
		parts = append(parts, "↑↓ navigate")
	}
	parts = append(parts, h.Actions...)
	if w.current == stepFrom {
		parts = append(parts, "r refresh")
	}
	parts = append(parts, "enter confirm")
	if w.canGoBack() {
		parts = append(parts, "esc back")
	} else {
		parts = append(parts, "esc cancel")
	}
	return indent + strings.Join(parts, helpSeparator)
}

func (w *wizard) canGoBack() bool {
	for s := w.current - 1; s >= 0; s-- {
		if w.marks[s].asked {
			return true
		}
	}
	return false
}

func asError(err error) *wtm.Error {
	return wtm.AsError(wtm.AsErrorParams{Err: err})
}
