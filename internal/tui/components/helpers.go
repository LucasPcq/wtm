package components

import (
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// TerminalWidth is how wide the surface may draw, zero when there is nothing to
// measure. A wizard step builds its body before bubbletea has sized it, so a
// step whose body is a table has no other way to know what it has to fit in.
func TerminalWidth() int {
	cols, _, err := term.GetSize(int(os.Stderr.Fd()))
	if err != nil || cols <= 0 {
		return 0
	}
	return cols
}

// PrintableWidth returns the visible character count of s, ignoring ANSI escape sequences.
func PrintableWidth(s string) int {
	inEscape := false
	n := 0
	for _, r := range s {
		if r == '\x1b' {
			inEscape = true
			continue
		}
		if inEscape {
			if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
				inEscape = false
			}
			continue
		}
		n++
	}
	return n
}

type renderVarGroupsParams struct {
	Groups []domain.NamespaceVarGroup
	Width  int
}

// renderVarGroups lays the vocabulary out as a small aligned table, each group
// wrapping under its own first variable. One run-on line stopped being readable
// as soon as a job declared more than one port. Shared by the two steps that ask
// for a value drawing on it — the namespace commands and the [[env]] templates.
func renderVarGroups(params renderVarGroupsParams) string {
	if len(params.Groups) == 0 {
		return ""
	}

	labelWidth := rules.NamespaceVarLabelWidth(params.Groups)
	indent := styles.Indent + domain.NamespaceVarIndent
	room := max(params.Width-PrintableWidth(indent)-labelWidth-len(domain.NamespaceVarSep), domain.CmdListMinWidth)

	var b strings.Builder
	b.WriteString("\n\n")
	b.WriteString(styles.Indent)
	b.WriteString(styles.Muted.Render(domain.NamespaceVarsHeading))

	for _, group := range params.Groups {
		for i, line := range rules.WrapVars(group.Vars, room) {
			label := group.Label
			if i > 0 {
				label = ""
			}
			b.WriteString("\n")
			b.WriteString(indent)
			b.WriteString(styles.Muted.Render(fmt.Sprintf(domain.NamespaceVarRowFmt,
				labelWidth, label, strings.Join(line, domain.NamespaceVarSep))))
		}
	}
	return b.String()
}

// listWindowParams describes a scrollable body: Total rows, of which Height fit
// on screen, and the row span [Top, Bottom] the cursor occupies — Top being the
// group heading above it when it has one, so scrolling never orphans a row from
// the name that says what it belongs to.
type listWindowParams struct {
	Offset int
	Height int
	Total  int
	Top    int
	Bottom int
}

// clampListOffset scrolls the body as little as it takes to bring the cursor's
// span back on screen. A list that outgrows the terminal would otherwise push
// the wizard's breadcrumb and its answered steps off the top.
func clampListOffset(params listWindowParams) int {
	if params.Height <= 0 || params.Total <= params.Height {
		return 0
	}

	offset := min(params.Offset, params.Total-params.Height)
	if params.Bottom >= offset+params.Height {
		offset = params.Bottom - params.Height + 1
	}
	if params.Top < offset && params.Bottom-params.Top < params.Height {
		offset = params.Top
	}
	return max(offset, 0)
}

type bodyWindowParams struct {
	Rows   []string
	Offset int
	Height int
	Top    int
	Bottom int
	// Extras is what hangs below the list and never scrolls — an open input's
	// context, an error banner. The window is sized around it.
	Extras string
}

// windowBody joins the rows visible at the settled offset, and returns that
// offset so a model can remember where it scrolled to. Callers use both: View
// takes the body, Update takes the offset.
func windowBody(params bodyWindowParams) (body string, offset int) {
	// A model bubbletea has not sized yet draws whole; once sized, the extras
	// take their rows off the top of the budget and the list keeps at least one,
	// since an open input taller than the terminal must not uncap the list.
	if params.Height <= 0 {
		return strings.Join(params.Rows, "\n") + params.Extras, 0
	}
	height := max(params.Height-strings.Count(params.Extras, "\n"), 1)
	if height >= len(params.Rows) {
		return strings.Join(params.Rows, "\n") + params.Extras, 0
	}

	offset = clampListOffset(listWindowParams{
		Offset: params.Offset, Height: height, Total: len(params.Rows),
		Top: params.Top, Bottom: params.Bottom,
	})
	return strings.Join(params.Rows[offset:offset+height], "\n") + params.Extras, offset
}
