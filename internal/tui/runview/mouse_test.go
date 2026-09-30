package runview

import (
	"fmt"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

// longHistory fills a pane past its viewport, so there is scrollback to move
// through at all.
func longHistory(n int) []string {
	lines := make([]string, 0, n)
	for i := range n {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	return lines
}

func wheel(button tea.MouseButton, x int) tea.MouseMsg {
	return tea.MouseMsg{X: x, Y: 5, Button: button, Action: tea.MouseActionPress}
}

// Over the pane the wheel scrolls the job's own scrollback. Without mouse
// tracking the terminal received these events and wrote into the view.
func TestWheelOverThePaneScrollsIt(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views: []runlogs.JobView{stopped("web")},
		Lines: map[string][]string{"web": longHistory(200)},
	})
	h.waitForPane(t, h.model.selected, "line 199")

	entry, held := h.model.panes.entry(h.model.selected)
	if !held {
		t.Fatal("the selected job holds no pane")
	}
	before := entry.pane.ScrollOffset()

	paneX := h.model.layout().Pane.X + 2
	model, _ := h.model.Update(wheel(tea.MouseButtonWheelUp, paneX))
	h.model, _ = model.(Model)

	if entry.pane.ScrollOffset() <= before {
		t.Errorf("ScrollOffset = %d, want it above %d", entry.pane.ScrollOffset(), before)
	}
}

// Over the list the wheel moves the selection, which is the gesture a reader
// expects there — and what already worked before mouse tracking existed only
// because the arrows did it.
func TestWheelOverTheListMovesTheSelection(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("web"), running("api")},
		Streams: []string{"web", "api"},
	})

	first := h.model.selected
	if !h.model.layout().SidebarVisible {
		t.Fatal("the test frame no longer draws the list, so this test exercises nothing")
	}

	model, _ := h.model.Update(wheel(tea.MouseButtonWheelDown, h.model.layout().Sidebar.X))
	next, _ := model.(Model)

	if next.selected == first {
		t.Error("the wheel over the list left the selection where it was")
	}
}

// A job holding the keyboard holds the whole input: moving the cursor out from
// under it with the wheel would send the next keystrokes somewhere else.
func TestWheelIsIgnoredWhileAJobHasTheKeyboard(t *testing.T) {
	h := focusedHarness(t, "web", "api")

	first := h.model.selected
	model, _ := h.model.Update(wheel(tea.MouseButtonWheelDown, h.model.layout().Sidebar.X))
	next, _ := model.(Model)

	if next.selected != first {
		t.Error("the wheel moved the selection while the job had the keyboard")
	}
}
