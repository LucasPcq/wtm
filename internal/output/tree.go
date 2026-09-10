package output

import (
	"fmt"
	"io"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/styles"
)

// FormatTree renders the worktree forest as a coloured ASCII tree. The returned
// body is raw (no outer blank lines); the command frames it.
func FormatTree(forest domain.Forest) string {
	if len(forest.Roots) == 0 {
		return UnchangedLine(domain.NoWorktreesMessage)
	}

	lines := make([]treeLine, 0, len(forest.Roots))
	for _, row := range rules.FlattenForest(forest) {
		lines = append(lines, treeLine{
			prefix:      row.Prefix,
			branch:      row.Node.Branch,
			styled:      styleBranch(&row.Node),
			annotations: formatTreeAnnotations(&row.Node),
		})
	}

	labelWidth := 0
	for _, l := range lines {
		labelWidth = max(labelWidth, l.labelWidth())
	}

	var b strings.Builder
	for _, l := range lines {
		b.WriteString(l.render(labelWidth))
		b.WriteString("\n")
	}
	return b.String()
}

// treeLine is one rendered node: its connector prefix, the node it represents,
// and the pre-rendered annotation string.
type treeLine struct {
	prefix      string // plain connector run, e.g. "│  ├─ "
	branch      string // plain branch name (alignment is measured on this)
	styled      string // styled branch label
	annotations string // styled, trailing
}

func (l treeLine) labelWidth() int {
	return len([]rune(l.prefix)) + len([]rune(l.branch))
}

func (l treeLine) render(labelWidth int) string {
	line := styles.Muted.Render(l.prefix) + l.styled
	if l.annotations == "" {
		return styles.Indent + line
	}
	pad := labelWidth - l.labelWidth() + 2
	return styles.Indent + line + strings.Repeat(" ", pad) + l.annotations
}

func styleBranch(node *domain.TreeNode) string {
	if node.IsVirtual {
		return styles.Muted.Render(node.Branch)
	}
	return styles.Bold.Render(node.Branch)
}

// formatTreeAnnotations builds the styled trailing status string for an ASCII node.
func formatTreeAnnotations(node *domain.TreeNode) string {
	parts := make([]string, 0, 6)
	for _, badge := range rules.TreeBadges(*node) {
		switch badge {
		case domain.TreeBadgeVirtual:
			parts = append(parts, styles.Muted.Render(domain.TreeBadgeVirtualText))
		case domain.TreeBadgePR:
			parts = append(parts, formatTreePR(node.Status.PR))
		case domain.TreeBadgeAhead:
			parts = append(parts, styles.Muted.Render(fmt.Sprintf("%s %s%d", domain.BadgeTextBase, domain.BadgeGlyphAhead, node.Status.CommitsAhead)))
		case domain.TreeBadgeOrigin:
			parts = append(parts, styleOriginAnnotation(node.Status))
		case domain.TreeBadgeRebasing:
			parts = append(parts, styles.Warning.Render(domain.TreeBadgeRebasingText))
		case domain.TreeBadgeDirty:
			parts = append(parts, styles.Warning.Render(domain.TreeBadgeDirtyText))
		case domain.TreeBadgeNeedsSync:
			parts = append(parts, styles.Warning.Render(domain.TreeBadgeNeedsSyncText))
		case domain.TreeBadgeCycle:
			parts = append(parts, styles.Warning.Render(domain.TreeBadgeCycleText))
		}
	}
	return strings.Join(parts, "  ")
}

// originAnnotationText renders the plain "origin ↑a ↓b" label (Mermaid form).
func originAnnotationText(s domain.TreeNodeStatus) string {
	switch s.OriginState {
	case domain.DivergenceBehind:
		return fmt.Sprintf("%s %s%d", domain.BadgeTextOrigin, domain.BadgeGlyphBehind, s.OriginBehind)
	case domain.DivergenceAhead:
		return fmt.Sprintf("%s %s%d", domain.BadgeTextOrigin, domain.BadgeGlyphAhead, s.OriginAhead)
	case domain.DivergenceDiverged:
		return fmt.Sprintf("%s %s%d %s%d", domain.BadgeTextOrigin, domain.BadgeGlyphAhead, s.OriginAhead, domain.BadgeGlyphBehind, s.OriginBehind)
	default:
		return ""
	}
}

// styleOriginAnnotation colors the origin label by state (ASCII form).
func styleOriginAnnotation(s domain.TreeNodeStatus) string {
	text := originAnnotationText(s)
	switch s.OriginState {
	case domain.DivergenceBehind:
		return styles.Warning.Render(text)
	case domain.DivergenceDiverged:
		return styles.DangerText.Render(text)
	default:
		return styles.Muted.Render(text)
	}
}

func formatTreePR(pr *domain.WorktreeListPR) string {
	if pr == nil {
		return ""
	}
	label := fmt.Sprintf("PR #%d", pr.Number)
	switch pr.State {
	case domain.PRStateMerged:
		return styles.Muted.Render(label + " merged")
	case domain.PRStateClosed:
		return styles.Muted.Render(label + " closed")
	default:
		return styles.Success.Render(label)
	}
}

// WriteTreeJSON writes the forest as an indented JSON object.
func WriteTreeJSON(w io.Writer, forest domain.Forest) error {
	return encodeJSON(w, forest)
}

// WriteTreeMermaid writes the forest as a Mermaid flowchart (top-down). Node IDs
// are sanitised branch names; labels carry the branch plus a plain-text status
// summary. It is machine output and is never framed.
func WriteTreeMermaid(w io.Writer, forest domain.Forest) error {
	var b strings.Builder
	b.WriteString("flowchart TD\n")

	var walk func(node *domain.TreeNode, parentID string)
	walk = func(node *domain.TreeNode, parentID string) {
		id := mermaidID(node.Branch)
		fmt.Fprintf(&b, "  %s[\"%s\"]\n", id, mermaidLabel(node))
		if parentID != "" {
			fmt.Fprintf(&b, "  %s --> %s\n", parentID, id)
		}
		for i := range node.Children {
			walk(&node.Children[i], id)
		}
	}

	for i := range forest.Roots {
		walk(&forest.Roots[i], "")
	}

	_, err := io.WriteString(w, b.String())
	return err
}

// mermaidID turns a branch name into a safe Mermaid node id (alphanumerics and
// underscores only).
func mermaidID(branch string) string {
	var b strings.Builder
	for _, r := range branch {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	return b.String()
}

func mermaidLabel(node *domain.TreeNode) string {
	parts := []string{strings.ReplaceAll(node.Branch, `"`, "'")}
	for _, badge := range rules.TreeBadges(*node) {
		switch badge {
		case domain.TreeBadgeVirtual:
			parts = append(parts, domain.TreeBadgeVirtualText)
		case domain.TreeBadgePR:
			label := fmt.Sprintf("PR #%d", node.Status.PR.Number)
			if node.Status.PR.State == domain.PRStateMerged || node.Status.PR.State == domain.PRStateClosed {
				label += " " + node.Status.PR.State
			}
			parts = append(parts, label)
		case domain.TreeBadgeAhead:
			parts = append(parts, fmt.Sprintf("%s %s%d", domain.BadgeTextBase, domain.BadgeGlyphAhead, node.Status.CommitsAhead))
		case domain.TreeBadgeOrigin:
			parts = append(parts, originAnnotationText(node.Status))
		case domain.TreeBadgeRebasing:
			parts = append(parts, domain.TreeBadgeRebasingText)
		case domain.TreeBadgeDirty:
			parts = append(parts, domain.TreeBadgeDirtyText)
		case domain.TreeBadgeNeedsSync:
			parts = append(parts, domain.TreeBadgeNeedsSyncText)
		case domain.TreeBadgeCycle:
			parts = append(parts, domain.TreeBadgeCycleText)
		}
	}
	return strings.Join(parts, " ")
}
