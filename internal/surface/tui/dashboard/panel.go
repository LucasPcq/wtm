package dashboard

import (
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

const (
	panelDetail = iota
	panelLogs
)

// panelTabLabels are the panel's two tabs, in order.
var panelTabLabels = [...]string{domain.DashboardPanelTabDetail, domain.DashboardPanelTabLogs}

var panelTabZones = [...]string{zonePanelTabDtl, zonePanelTabLogs}

// panelTabLines head the right-hand panel, in the place its title used to take.
// They borrow the main tab bar's vocabulary — weight, padding and the rule
// under the active one — because that is what already reads as "these are tabs,
// and they are clickable". The freshness marker keeps its flush-right place on
// the first row: it dates the panel, not the tab.
func (m Model) panelTabLines(width int) []string {
	// The marker is measured first and the tabs are dropped whole to fit under
	// it: spread would clip a marked label, and truncateRendered cutting through
	// a zone marker silently breaks that zone's bounds — the LOGS tab became
	// unclickable on a narrow panel.
	marker := m.detailFreshnessMarker()
	budget := width
	if marker != "" {
		budget = max(width-lipgloss.Width(marker)-1, 0)
	}

	rendered := make([]string, 0, len(panelTabLabels))
	activeStart, activeWidth, used := 0, 0, 0
	for index, label := range panelTabLabels {
		style := styles.DashboardTabInactive
		if index == m.panelTab {
			style = styles.DashboardTabActive
		}
		tab := style.Render(label)
		if used+lipgloss.Width(tab) > budget {
			break
		}
		if index == m.panelTab {
			activeStart, activeWidth = used, lipgloss.Width(tab)
		}
		used += lipgloss.Width(tab)
		rendered = append(rendered, m.marks().Mark(panelTabZones[index], tab))
	}

	bar := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)
	if marker != "" && used+lipgloss.Width(marker) <= width {
		bar += strings.Repeat(" ", width-used-lipgloss.Width(marker)) + marker
	}
	return []string{
		pad(bar, width),
		tabRule(tabRuleParams{Width: width, ActiveStart: activeStart, ActiveWidth: activeWidth}),
	}
}
