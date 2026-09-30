package dashboard

import (
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

func (m Model) renderTree(layout domain.DashboardLayout) string {
	return m.renderPanel(panelParams{
		Rect:  layout.List,
		Title: domain.DashboardTreeTitle,
		Body:  m.treeBody(layout),
		Zone:  zoneTree,
	})
}

func (m Model) treeBody(layout domain.DashboardLayout) []string {
	width := layout.List.Width - borderWidth - paddingWidth
	switch {
	case m.treeErr != nil:
		return []string{styles.DashboardEmpty.Render(truncate(m.treeErr.Error(), width))}
	case !m.treeLoaded:
		return []string{styles.DashboardEmpty.Render(truncate(domain.DashboardLoadingTree, width))}
	case len(m.treeRows) == 0:
		return []string{styles.DashboardEmpty.Render(truncate(domain.DashboardEmptyTree, width))}
	}

	end := min(m.treeOffset+layout.TreeRows, len(m.treeRows))
	lines := make([]string, 0, max(end-m.treeOffset, 0)*domain.DashboardTreeRowHeight)
	for index := m.treeOffset; index < end; index++ {
		lines = append(lines, m.marks().Mark(treeRowZone(index), m.renderTreeRow(index, width)))
		lines = append(lines, m.treeSpacer(index, width))
	}
	return lines
}

// treeSpacer is the breathing line under a row. It carries the gutter down to the
// next row, so the tree stays connected; between two roots it carries nothing,
// which is what sets one tree apart from the next.
func (m Model) treeSpacer(index, width int) string {
	if index+1 >= len(m.treeRows) {
		return ""
	}
	prefix := rules.TreeSpacerPrefix(m.treeRows[index+1].Prefix)
	if prefix == "" {
		return ""
	}
	return rowIndent + styles.DashboardTreeGutter.Render(truncate(prefix, max(width-rowBarWidth, 0)))
}

// renderTreeRow draws one node on one line: the gutter its depth earned it, the
// branch, then what its state amounts to. The selected row carries the same
// accent bar and tint as a list row, so the two tabs read as one dashboard.
func (m Model) renderTreeRow(index, width int) string {
	row := m.treeRows[index]
	inner := max(width-rowBarWidth, 0)

	if index == m.treeCursor {
		bar := styles.DashboardRowBar.Render(rowBar + " ")
		left := row.Prefix + treeGlyph(row.Node) + " " + row.Node.Branch
		plain := spread(left, strings.Join(m.treeBadgeTexts(row.Node), treeBadgeGap), inner)
		return bar + styles.DashboardRowSelected.Width(inner).Render(plain)
	}

	left := styles.DashboardTreeGutter.Render(row.Prefix) + m.styleTreeGlyph(row.Node) + " " + m.styleTreeBranch(row.Node)
	// The badges sit flush right, so the names stay the one column the eye follows
	// down the gutter instead of ending wherever their length lands.
	return rowIndent + spread(left, m.styleTreeBadges(row.Node), inner)
}

func treeGlyph(node domain.TreeNode) string {
	if node.IsVirtual {
		return domain.DashboardTreeVirtualGlyph
	}
	return domain.DashboardTreeNodeGlyph
}

func (m Model) styleTreeGlyph(node domain.TreeNode) string {
	if node.IsVirtual {
		return styles.DashboardTreeVirtual.Render(domain.DashboardTreeVirtualGlyph)
	}
	if node.IsMain {
		return styles.DashboardTreeRoot.Render(domain.DashboardTreeNodeGlyph)
	}
	return styles.DashboardTreeNode.Render(domain.DashboardTreeNodeGlyph)
}

func (m Model) styleTreeBranch(node domain.TreeNode) string {
	switch {
	case node.IsVirtual:
		return styles.DashboardTreeVirtual.Render(node.Branch)
	case node.IsMain:
		return styles.DashboardTreeRoot.Render(node.Branch)
	}
	return styles.DashboardRowName.Render(node.Branch)
}

func (m Model) styleTreeBadges(node domain.TreeNode) string {
	parts := make([]string, 0, 4)
	for _, badge := range rules.TreeBadges(node) {
		parts = append(parts, styleTreeBadge(badge, m.treeBadgeText(badge, node)))
	}
	if tag := m.activeTreeTag(node); tag != "" {
		parts = append(parts, styles.DashboardRowMeta.Render(tag))
	}
	return strings.Join(parts, treeBadgeGap)
}

// activeTreeTag names the worktree the shell is currently inside, the same
// "you are here" tag the list already shows on its meta line — so both
// views say the same thing about the same worktree.
func (m Model) activeTreeTag(node domain.TreeNode) string {
	if m.activeBranch == "" || node.Branch != m.activeBranch {
		return ""
	}
	return domain.DetailYouAreHere
}

// treeBadgeGap keeps the badges apart from one another: run together they read as
// one blob rather than as the two or three distinct facts they are.
const treeBadgeGap = "  "

func (m Model) treeBadgeTexts(node domain.TreeNode) []string {
	badges := rules.TreeBadges(node)
	texts := make([]string, 0, len(badges)+1)
	for _, badge := range badges {
		texts = append(texts, m.treeBadgeText(badge, node))
	}
	if tag := m.activeTreeTag(node); tag != "" {
		texts = append(texts, tag)
	}
	return texts
}

func (m Model) treeBadgeText(badge domain.TreeBadge, node domain.TreeNode) string {
	switch badge {
	case domain.TreeBadgeVirtual:
		return domain.DashboardTreeVirtual
	case domain.TreeBadgeRunning:
		return fmt.Sprintf(domain.TreeBadgeRunningFmt, node.Status.RunningJobs)
	case domain.TreeBadgePR:
		return fmt.Sprintf(domain.DashboardTreePRFmt, node.Status.PR.Number)
	case domain.TreeBadgeAhead:
		return fmt.Sprintf(domain.DashboardTreeAheadFmt, domain.BadgeTextBase, domain.BadgeGlyphAhead, node.Status.CommitsAhead)
	case domain.TreeBadgeOrigin:
		return treeOriginText(node.Status)
	case domain.TreeBadgeRebasing:
		return domain.TreeBadgeRebasingText
	case domain.TreeBadgeDirty:
		return domain.TreeBadgeDirtyText
	case domain.TreeBadgeNeedsSync:
		return domain.TreeBadgeNeedsSyncText
	case domain.TreeBadgeCycle:
		return domain.TreeBadgeCycleText
	}
	return ""
}

func styleTreeBadge(badge domain.TreeBadge, text string) string {
	switch badge {
	case domain.TreeBadgeRebasing, domain.TreeBadgeDirty, domain.TreeBadgeNeedsSync, domain.TreeBadgeCycle:
		return styles.DashboardTreeWarn.Render(text)
	case domain.TreeBadgeRunning:
		return styles.Success.Render(text)
	}
	return styles.DashboardRowMeta.Render(text)
}

func treeOriginText(status domain.TreeNodeStatus) string {
	switch status.OriginState {
	case domain.DivergenceBehind:
		return fmt.Sprintf(domain.DashboardTreeAheadFmt, domain.BadgeTextOrigin, domain.BadgeGlyphBehind, status.OriginBehind)
	case domain.DivergenceAhead:
		return fmt.Sprintf(domain.DashboardTreeAheadFmt, domain.BadgeTextOrigin, domain.BadgeGlyphAhead, status.OriginAhead)
	default:
		return fmt.Sprintf(domain.DashboardTreeDivergedFmt, domain.BadgeTextOrigin,
			domain.BadgeGlyphAhead, status.OriginAhead, domain.BadgeGlyphBehind, status.OriginBehind)
	}
}

// treeRowPoint is the selected tree row, so the keyboard opens the context menu
// where the mouse would have — under the row, not over it.
func (m Model) treeRowPoint() domain.Rect {
	layout := m.layout()
	top := layout.List.Y + domain.DashboardChromeHeight - 1 + domain.DashboardTitleGap
	return domain.Rect{
		X: layout.List.X + borderWidth,
		Y: top + (m.treeCursor-m.treeOffset)*domain.DashboardTreeRowHeight,
	}
}
