package dashboard

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
	"github.com/LucasPcq/wtm/internal/tui/worktreepicker"
)

const (
	rowBar    = "▌"
	rowIndent = "  "
)

func (m Model) renderList(layout domain.DashboardLayout) string {
	return m.renderPanel(panelParams{
		Rect:  layout.List,
		Title: domain.DashboardListTitle,
		Body:  m.listBody(layout),
		Zone:  zoneList,
	})
}

func (m Model) listBody(layout domain.DashboardLayout) []string {
	width := layout.List.Width - borderWidth - paddingWidth
	if !m.loaded {
		return []string{styles.DashboardEmpty.Render(truncate(domain.LoadingWorktreesText, width))}
	}
	if len(m.statuses) == 0 {
		return []string{styles.DashboardEmpty.Render(truncate(domain.DashboardEmptyList, width))}
	}

	end := min(m.offset+layout.ListRows, len(m.statuses))
	rows := make([]string, 0, max(end-m.offset, 0)*(domain.DashboardRowHeight+domain.DashboardRowGap))
	for index := m.offset; index < end; index++ {
		if index > m.offset {
			rows = append(rows, "")
		}
		rows = append(rows, m.marks().Mark(rowZone(index), strings.Join(m.renderRow(index, width), "\n")))
	}
	return rows
}

// renderRow draws one worktree over two lines: what it is called and, under it,
// what its state amounts to. The selected one is tinted across its whole width
// and carries the accent bar, so the row the keyboard is on reads as a block
// rather than as a marker. The worktree the shell is currently in is named by
// a "you are here" tag on the meta line (rowMeta), the same vocabulary as
// "from main" or "PR #67" — a bare glyph on the name said nothing on its own.
// A worktree a background run holds shows its progress instead: the spinner
// replaces the state pill, and the stage the run posted replaces the meta
// line — the same vocabulary as the freshness contract, the spinner says
// "in progress", never "broken".
func (m Model) renderRow(index, width int) []string {
	status := m.statuses[index]
	inner := max(width-rowBarWidth, 0)

	name := status.Branch

	pill := worktreepicker.BuildStatus(status)
	pillText, pillRendered := pill.Text, pill.Render()
	metaPlain, metaColored := m.rowMeta(status, false), m.rowMeta(status, true)
	if op, locked := m.ops.holding(status.Branch); locked {
		pillText, pillRendered = m.spinner.View(), m.spinner.View()
		stage := op.stageFor(status.Branch)
		metaPlain, metaColored = stage, stage
	}

	badge := m.runningBadge(status)

	if index == m.cursor {
		bar := styles.DashboardRowBar.Render(rowBar + " ")
		line := spread(name, pillText, inner)
		rowStyle := styles.DashboardRowSelected
		if status.Branch == m.flashBranch && rules.FlashLit(rules.FlashParams{
			Since: m.flashSince, Now: time.Now(), Duration: domain.DashboardRowFlash,
		}) {
			rowStyle = styles.DashboardRowFlashBright
		}
		return []string{
			bar + rowStyle.Width(inner).Bold(true).Render(line),
			bar + rowStyle.Width(inner).Render(metaLine(metaLineParams{Meta: metaPlain, Badge: badge, Inner: inner})),
		}
	}

	line := spread(styles.DashboardRowName.Render(name), pillRendered, inner)
	// Both lines are padded to the same width: the row is one clickable block, and
	// a short second line would cut its zone short.
	return []string{
		rowIndent + line,
		rowIndent + metaLine(metaLineParams{
			Meta:    metaColored,
			Badge:   badge,
			Colored: true,
			Inner:   inner,
		}),
	}
}

// runningBadge is what the worktree has up. It hangs at the end of the meta
// line, under the state pill, rather than taking its rank among the tags: a
// worktree with no parent would otherwise carry it first, and the eye would
// have to look for it row by row.
func (m Model) runningBadge(status domain.WorktreeStatus) string {
	count := m.running[status.Path]
	if count == 0 {
		return ""
	}
	return fmt.Sprintf(domain.TreeBadgeRunningFmt, count)
}

type metaLineParams struct {
	Meta    string
	Badge   string
	Colored bool
	Inner   int
}

// metaLine pads to the full width either way: the row is one clickable block,
// and a badge is never allowed to push the meta out of the line it shares. The
// badge is styled here, never before: an empty string rendered through a style
// comes back as a pair of escapes — zero columns wide, but not empty, which
// would cost the meta a column on every row that runs nothing.
func metaLine(params metaLineParams) string {
	if params.Badge == "" {
		return pad(truncateRendered(params.Meta, params.Inner), params.Inner)
	}
	return spread(params.Meta, styleMeta(params.Badge, params.Colored, styles.Success), params.Inner)
}

// rowBarWidth is the gutter the accent bar and its space take, kept off the
// tinted span so the bar keeps its own color.
const rowBarWidth = 2

// rowMeta is the second line of a row: the same tags the pickers show, joined
// into one line rather than crowded against the branch name. A selected row is
// tinted as one span, so it takes them uncolored.
func (m Model) rowMeta(status domain.WorktreeStatus, colored bool) string {
	parts := make([]string, 0, 4)
	if parent := m.parents[status.Branch]; parent != "" {
		parts = append(parts, styleMeta(domain.DashboardMetaFromPrefix+parent, colored, styles.DashboardRowMeta))
	}
	for _, tag := range worktreepicker.BuildTags(worktreepicker.BuildTagsParams{Status: status, PRs: m.prs}) {
		if colored {
			parts = append(parts, tag.Render())
			continue
		}
		parts = append(parts, tag.Text)
	}
	if status.Branch != "" && status.Branch == m.activeBranch {
		parts = append(parts, styleMeta(domain.DetailYouAreHere, colored, styles.DashboardRowMeta))
	}
	if len(parts) == 0 {
		return styleMeta(domain.DashboardMetaNothing, colored, styles.DashboardRowMeta)
	}
	return strings.Join(parts, styleMeta(domain.DashboardMetaSeparator, colored, styles.DashboardRowMeta))
}

func styleMeta(text string, colored bool, style lipgloss.Style) string {
	if colored {
		return style.Render(text)
	}
	return text
}
