package components

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

func twoRoutes() RouteListModel {
	return NewRouteList(NewRouteListParams{
		Rows: []domain.PortRouteRow{
			{Job: "web", Port: "VITE_PORT", Base: 5173, File: "apps/web/.env", Route: domain.PortRouteEnv},
			{Job: "api", Port: "PORT", Base: 4001, File: "apps/api/.env", Route: domain.PortRouteCommand},
		},
	})
}

func press(m RouteListModel, key string) RouteListModel {
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)})
	return updated
}

func TestRouteListReadsBackWhatItWasPreFilledWith(t *testing.T) {
	routes := twoRoutes().Routes()

	if routes[domain.PortRef{Job: "web", Name: "VITE_PORT"}] != domain.PortRouteEnv || routes[domain.PortRef{Job: "api", Name: "PORT"}] != domain.PortRouteCommand {
		t.Fatalf("got %+v", routes)
	}
}

func TestRouteListSwitchesOneRowAndLeavesTheOther(t *testing.T) {
	routes := press(twoRoutes(), " ").Routes()

	if routes[domain.PortRef{Job: "web", Name: "VITE_PORT"}] != domain.PortRouteCommand {
		t.Fatalf("the cursor's row must switch, got %+v", routes)
	}
	if routes[domain.PortRef{Job: "api", Name: "PORT"}] != domain.PortRouteCommand {
		t.Fatalf("the other row must not move, got %+v", routes)
	}
}

func TestRouteListSwitchesBack(t *testing.T) {
	if routes := press(press(twoRoutes(), " "), " ").Routes(); routes[domain.PortRef{Job: "web", Name: "VITE_PORT"}] != domain.PortRouteEnv {
		t.Fatalf("got %+v", routes)
	}
}

func TestRouteListDoesNotMutateTheRowsItWasGiven(t *testing.T) {
	model := twoRoutes()
	press(model, " ")

	if model.Rows()[0].Route != domain.PortRouteEnv {
		t.Fatal("the wizard rebuilds a step from its rows; toggling must not reach back into them")
	}
}

func TestRouteListNamesTheFileTheEnvRouteWritesTo(t *testing.T) {
	if got := twoRoutes().View(); !strings.Contains(got, "apps/web/.env") {
		t.Fatalf("got %q", got)
	}
}

func TestRouteListMarksAFileNothingProvisions(t *testing.T) {
	model := NewRouteList(NewRouteListParams{Rows: []domain.PortRouteRow{
		{Job: "reports", Port: "PORT", Base: 5177, File: "apps/reports/.env", AddTarget: true, Route: domain.PortRouteEnv},
	}})

	if got := model.View(); !strings.Contains(got, domain.PortKeyTargetSuffix) {
		t.Fatalf("a file the project does not provision must say so: %q", got)
	}
}

func TestRouteListDoneRowEndsTheStep(t *testing.T) {
	model := twoRoutes()
	for i := 0; i < 2; i++ {
		updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
		model = updated
	}
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if !updated.Done() {
		t.Fatal("enter on the Done row ends the step")
	}
}

func TestRouteListKeepsAJobsTwoPortsOnTheirOwnRoutes(t *testing.T) {
	model := NewRouteList(NewRouteListParams{Rows: []domain.PortRouteRow{
		{Job: "web", Port: "PORT", Base: 3000, File: ".env", Route: domain.PortRouteEnv},
		{Job: "web", Port: "ADMIN_PORT", Base: 3001, File: ".env", Route: domain.PortRouteCommand},
	}})

	routes := model.Routes()
	if routes[domain.PortRef{Job: "web", Name: "PORT"}] != domain.PortRouteEnv {
		t.Fatalf("got %+v — one port's answer must not be swallowed by the other's", routes)
	}
	if routes[domain.PortRef{Job: "web", Name: "ADMIN_PORT"}] != domain.PortRouteCommand {
		t.Fatalf("got %+v", routes)
	}
}
