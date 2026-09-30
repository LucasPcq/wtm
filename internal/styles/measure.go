package styles

import (
	"strings"

	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
)

type TruncateParams struct {
	Value string
	Width int
}

// Truncate cuts a string to Width visible columns, keeping its escape sequences
// intact — a line cut mid-colour would leak that colour onto everything after it.
func Truncate(params TruncateParams) string {
	return ansi.Truncate(params.Value, params.Width, domain.Ellipsis)
}

// ExpandTabs makes a line measurable. A tab is a jump to the next stop, not a
// column, so a line holding one is wider than anything counting its runes says.
func ExpandTabs(line string) string {
	if !strings.Contains(line, "\t") {
		return line
	}
	return strings.ReplaceAll(line, "\t", strings.Repeat(" ", domain.TabWidth))
}
