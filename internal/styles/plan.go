package styles

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
)

const planGap = "  "

// PlanRow is one line of a plan as every surface shows it: what the plan sets,
// and to what.
type PlanRow struct {
	Label string
	Value string
}

type PlanParams struct {
	Rows     []PlanRow
	Warnings []string
}

// RenderPlan is the plan of a command as the command line's dry run and the
// form's recap both draw it: its rows, labels aligned, then its warnings.
func RenderPlan(params PlanParams) string {
	width := 0
	for _, row := range params.Rows {
		width = max(width, lipgloss.Width(row.Label))
	}
	lines := make([]string, 0, len(params.Rows)+len(params.Warnings))
	for _, row := range params.Rows {
		pad := strings.Repeat(" ", width-lipgloss.Width(row.Label))
		lines = append(lines, Muted.Render(row.Label)+pad+planGap+row.Value)
	}
	for _, warning := range params.Warnings {
		lines = append(lines, Warning.Render(domain.GlyphAttention)+" "+warning)
	}
	return strings.Join(lines, "\n")
}
