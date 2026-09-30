package dashboard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
)

func openHelp(t *testing.T, width, height int) Model {
	t.Helper()
	model := newTestModel(t, width, height, "a", "b")
	return update(model, key(domain.KeyHelp))
}

// The overlay is pasted whole and clipped by the frame it lands on, so a box
// taller than the screen loses its bottom border rather than scrolling.
func TestHelpOverlayNeverOutgrowsTheScreen(t *testing.T) {
	for _, screen := range [][2]int{{120, 40}, {100, 24}, {80, 20}, {60, 30}, {40, 12}, {30, 8}} {
		model := Model{width: screen[0], height: screen[1]}
		box, rect := model.helpBox()
		if box == "" {
			continue
		}
		if width := lipgloss.Width(box); width > screen[0] {
			t.Errorf("%dx%d: box is %d wide", screen[0], screen[1], width)
		}
		if height := lipgloss.Height(box); height > screen[1] {
			t.Errorf("%dx%d: box is %d tall", screen[0], screen[1], height)
		}
		if rect.Y+lipgloss.Height(box) > screen[1] {
			t.Errorf("%dx%d: box at Y=%d runs off the bottom", screen[0], screen[1], rect.Y)
		}
	}
}

func TestHelpOverlayPairsSectionsSideBySideWhenWide(t *testing.T) {
	box, _ := Model{width: 120, height: 40, runConfig: runningJobs}.helpBox()

	for _, band := range [][2]string{
		{domain.DashboardHelpSectionNav, domain.DashboardHelpSectionAct},
		{domain.DashboardHelpSectionRun, domain.DashboardHelpSectionMouse},
	} {
		if !hasLineWith(box, band[0], band[1]) {
			t.Errorf("%s and %s must head the same band", band[0], band[1])
		}
	}
}

func TestHelpOverlayStacksSectionsWhenNarrow(t *testing.T) {
	box, _ := Model{width: 60, height: 60, runConfig: runningJobs}.helpBox()

	if hasLineWith(box, domain.DashboardHelpSectionNav, domain.DashboardHelpSectionAct) {
		t.Error("a narrow screen holds one column, not two")
	}
	for _, title := range []string{
		domain.DashboardHelpSectionNav, domain.DashboardHelpSectionAct,
		domain.DashboardHelpSectionRun, domain.DashboardHelpSectionMouse,
		domain.DashboardHelpSectionView,
	} {
		if !strings.Contains(box, title) {
			t.Errorf("section %s dropped from the narrow layout", title)
		}
	}
}

func TestHelpOverlayScrollsOnAScreenTooShortForIt(t *testing.T) {
	model := openHelp(t, 120, 14)
	before, _ := model.helpBox()

	model = update(model, namedKey(tea.KeyDown))
	if model.helpScroll != 1 {
		t.Fatalf("helpScroll = %d after down, want 1", model.helpScroll)
	}
	if after, _ := model.helpBox(); after == before {
		t.Error("scrolling must change what the overlay shows")
	}
	if !strings.Contains(before, domain.DashboardHelpHintScroll) {
		t.Error("a scrollable reference must say so in its hint")
	}
}

func TestHelpScrollStopsAtBothEnds(t *testing.T) {
	model := openHelp(t, 120, 14)

	model = update(model, namedKey(tea.KeyUp))
	if model.helpScroll != 0 {
		t.Errorf("helpScroll = %d at the top, want 0", model.helpScroll)
	}

	model = update(model, key("G"))
	bottom := model.helpScroll
	model = update(model, namedKey(tea.KeyDown))
	if model.helpScroll != bottom {
		t.Errorf("helpScroll = %d past the bottom, want %d", model.helpScroll, bottom)
	}
	if bottom == 0 {
		t.Error("a scrollable reference must have somewhere to scroll to")
	}
}

func TestHelpDoesNotScrollWhenItAllFits(t *testing.T) {
	model := openHelp(t, testWidth, testHeight)

	model = update(model, namedKey(tea.KeyDown))

	if model.helpScroll != 0 {
		t.Errorf("helpScroll = %d, want 0: the whole reference is on screen", model.helpScroll)
	}
	if box, _ := model.helpBox(); strings.Contains(box, domain.DashboardHelpHintScroll) {
		t.Error("a reference that fits must not offer to scroll")
	}
}

func TestHelpReopensAtTheTop(t *testing.T) {
	model := openHelp(t, 120, 14)
	model = update(model, key("G"))

	model = update(model, key(domain.KeyHelp))
	model = update(model, key(domain.KeyHelp))

	if model.helpScroll != 0 {
		t.Errorf("helpScroll = %d on reopen, want 0", model.helpScroll)
	}
}

// The overlay documents the mouse, so it is the last place that may ignore it.
func TestClickBesideTheHelpOverlayClosesIt(t *testing.T) {
	model := openHelp(t, testWidth, testHeight)
	_, rect := model.helpBox()

	model = update(model, click(rect.X+rect.Width/2, rect.Y+rect.Height/2))
	if !model.showHelp {
		t.Fatal("a click inside the overlay must not close it")
	}

	model = update(model, click(0, 0))
	if model.showHelp {
		t.Error("a click beside the overlay must close it")
	}
}

func TestWheelScrollsTheHelpOverlay(t *testing.T) {
	model := openHelp(t, 120, 14)

	model = update(model, wheel(1, 1, tea.MouseButtonWheelDown))
	if model.helpScroll != 1 {
		t.Fatalf("helpScroll = %d after a wheel down, want 1", model.helpScroll)
	}

	model = update(model, wheel(1, 1, tea.MouseButtonWheelUp))
	if model.helpScroll != 0 {
		t.Errorf("helpScroll = %d after a wheel up, want 0", model.helpScroll)
	}
}

func hasLineWith(box string, parts ...string) bool {
	for _, line := range strings.Split(box, "\n") {
		found := true
		for _, part := range parts {
			found = found && strings.Contains(line, part)
		}
		if found {
			return true
		}
	}
	return false
}

func TestHelpOverlayListsRunKeysOnlyWhenTheProjectRunsJobs(t *testing.T) {
	without := newTestModel(t, 160, 60, "a", "b")
	if box, _ := update(without, key(domain.KeyHelp)).helpBox(); strings.Contains(box, domain.DashboardHelpTextRunLogs) {
		t.Errorf("help lists %q for a project without run.toml:\n%s", domain.DashboardHelpTextRunLogs, box)
	}

	with := withRunJobs(newTestModel(t, 160, 60, "a", "b"))
	if box, _ := update(with, key(domain.KeyHelp)).helpBox(); !strings.Contains(box, domain.DashboardHelpTextRunLogs) {
		t.Errorf("help misses %q for a project that runs jobs:\n%s", domain.DashboardHelpTextRunLogs, box)
	}
}

var runningJobs = domain.RunConfig{Jobs: []domain.JobConfig{{Name: "web"}}}
