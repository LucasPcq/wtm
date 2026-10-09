package dashboard

import (
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

// A user who never set the run module up gets v0.27.1's dashboard back: two
// tabs, and a panel titled Detail rather than a DETAIL / LOGS pair whose second
// half could only ever say there is nothing to show.
func TestWithoutRunJobsTheDashboardIsThatOfV0271(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	view := stripANSI(model.View())

	if strings.Contains(view, domain.DashboardTabServices) {
		t.Error("the Services tab is drawn for a project without a run module")
	}
	if strings.Contains(view, domain.DashboardPanelTabLogs) {
		t.Error("the LOGS tab is drawn for a project without a run module")
	}
	if !strings.Contains(view, domain.DashboardDetailTitle) {
		t.Error("the panel lost its Detail title")
	}

	for range 4 {
		model = update(model, namedKey(tea.KeyTab))
		if model.tab == tabServices {
			t.Fatal("tab reached a Services tab that is not drawn")
		}
	}
	if model = update(model, key(domain.KeyRunLogs)); model.logsOpen() {
		t.Error("L opened a logs view that is not drawn")
	}
}

func TestWithRunJobsTheRunTabsAreDrawn(t *testing.T) {
	model := withRunJobs(newTestModel(t, testWidth, testHeight, "a", "b"))
	view := stripANSI(model.View())
	for _, want := range []string{domain.DashboardTabServices, domain.DashboardPanelTabLogs} {
		if !strings.Contains(view, want) {
			t.Errorf("view misses %q", want)
		}
	}
}

// A run.toml that cannot be read keeps its tabs — jobs may still be up — but
// they say why nothing is listed instead of reading as an empty project.
func TestAnInvalidRunTomlSaysSoInTheRunTabs(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	model = update(model, jobsMsg{configErr: errors.New("toml: line 3: expected value"), known: true})

	logs, _ := model.openLogsTab()
	if view := stripANSI(logs.View()); !strings.Contains(view, "toml: line 3") {
		t.Errorf("logs view does not name the cause:\n%s", view)
	}
	services, _ := model.selectTab(tabServices)
	if view := stripANSI(services.View()); !strings.Contains(view, "toml: line 3") {
		t.Errorf("Services tab does not name the cause:\n%s", view)
	}
}

// run.toml removed while the Services tab is on screen: the tab it stood on is
// gone, so the list takes its place.
func TestLosingTheRunModuleLeavesTheServicesTab(t *testing.T) {
	model := withRunJobs(newTestModel(t, testWidth, testHeight, "a", "b"))
	model, _ = model.selectTab(tabServices)

	model = update(model, jobsMsg{known: true})
	if model.tab != tabWorktrees {
		t.Errorf("tab = %d, want the list once the Services tab is gone", model.tab)
	}
}
