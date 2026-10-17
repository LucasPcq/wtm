package components

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/styles"
)

// The inline-filter behavior shared by SelectListModel and MultiSelectModel:
// press "/" to filter, type to narrow by case-insensitive label substring,
// backspace-to-empty or esc to leave. Each model keeps its own filter/filtering/
// filtered fields (their cursor and navigation semantics differ) and delegates
// the identical matching, prompt rendering, and height math to these helpers.

// filterMatchParams holds inputs for filterVisible.
type filterMatchParams struct {
	query    string
	count    int
	label    func(int) string
	eligible func(int) bool
}

// filterVisible returns the indices of items kept by an inline filter: every i in
// [0,count) whose label matches query by case-insensitive substring. With an empty
// query all items are kept; eligible (e.g. "not a separator") is applied only while
// filtering so structural rows stay visible in the unfiltered list.
func filterVisible(params filterMatchParams) []int {
	visible := make([]int, 0, params.count)
	lower := strings.ToLower(params.query)
	for i := 0; i < params.count; i++ {
		if params.query != "" && params.eligible != nil && !params.eligible(i) {
			continue
		}
		if params.query != "" && !strings.Contains(strings.ToLower(params.label(i)), lower) {
			continue
		}
		visible = append(visible, i)
	}
	return visible
}

// filterPromptParams holds inputs for renderFilterPrompt.
type filterPromptParams struct {
	query         string
	active        bool
	selectedCount int
}

// renderFilterPrompt renders the "/ query" header shown while an inline filter is
// active or applied: the query, a block cursor while the input is focused, and an
// optional "(N selected)" suffix (omitted when selectedCount <= 0), then a blank
// line. Returns "" when the filter chrome should be hidden.
func renderFilterPrompt(params filterPromptParams) string {
	if !params.active && params.query == "" {
		return ""
	}
	var b strings.Builder
	b.WriteString(styles.FilterPrompt.Render("/ " + params.query))
	if params.active {
		b.WriteString(styles.FilterCursor.Render(" "))
	}
	if params.selectedCount > 0 {
		b.WriteString(styles.Muted.Render(fmt.Sprintf("  (%d selected)", params.selectedCount)))
	}
	b.WriteString("\n\n")
	return b.String()
}

// filterOverhead is the number of vertical lines renderFilterPrompt occupies (0
// when the filter chrome is hidden), for visible-height math.
func filterOverhead(active bool, query string) int {
	if active || query != "" {
		return 2
	}
	return 0
}
