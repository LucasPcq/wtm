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
