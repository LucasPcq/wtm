package dashboard

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
)

func TestThePanelHeadsWithItsTwoTabs(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}

	lines := model.panelTabLines(60)

	if len(lines) != 2 {
		t.Fatalf("lines = %d, want the tabs and the rule under the active one", len(lines))
	}
	head := stripANSI(lines[0])
	if !strings.Contains(head, domain.DashboardPanelTabDetail) || !strings.Contains(head, domain.DashboardPanelTabLogs) {
		t.Errorf("line = %q, want both tabs named", head)
	}
	for index, line := range lines {
		if got := lipgloss.Width(stripANSI(line)); got != 60 {
			t.Errorf("line %d width = %d, want the line to fill the panel", index, got)
		}
	}
}

// The rule under the active tab is what says which one is active, exactly as it
// does on the main tab bar — a panel tab that only differed by weight did not
// read as a tab at all.
func TestThePanelRuleFollowsTheActiveTab(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}

	onDetail := stripANSI(model.panelTabLines(60)[1])
	model.panelTab = panelLogs
	onLogs := stripANSI(model.panelTabLines(60)[1])

	if onDetail == onLogs {
		t.Error("the rule did not move with the active tab")
	}
	active := strings.Index(onDetail, domain.DashboardActiveRuleGlyph)
	if next := strings.Index(onLogs, domain.DashboardActiveRuleGlyph); next <= active {
		t.Errorf("rule starts at %d then %d, want it further right under LOGS", active, next)
	}
}

func TestTheFreshnessMarkerKeepsItsPlaceOnTheTabLine(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.detailLoading, model.detailSince = "a", time.Now().Add(-time.Hour)

	line := stripANSI(model.panelTabLines(60)[0])

	if !strings.Contains(line, domain.DashboardRefreshing) {
		t.Errorf("line = %q, want the marker kept: it dates the panel, not the tab", line)
	}
}

// truncateRendered must never see a marked label: a cut through a zone marker
// silently breaks that zone's bounds, and the LOGS tab became unclickable.
func TestThePanelTabsStayClickableWhenTheMarkerCrowdsThem(t *testing.T) {
	model := logsModel(t, RunParams{}, "a")
	model.runConfig = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
	model.detailLoading, model.detailSince = "a", time.Now().Add(-time.Hour)

	for _, width := range []int{60, 30, 24, 20, 12} {
		lines := model.panelTabLines(width)
		if got := lipgloss.Width(stripANSI(lines[0])); got > width {
			t.Errorf("width %d: line is %d wide, want it to fit", width, got)
		}
		for _, label := range panelTabLabels {
			plain := stripANSI(lines[0])
			if strings.Contains(plain, label) {
				continue
			}
			// A tab that does not fit is dropped whole, never cut through.
			if strings.Contains(plain, label[:2]) {
				t.Errorf("width %d: %q was cut rather than dropped: %q", width, label, plain)
			}
		}
	}
}
