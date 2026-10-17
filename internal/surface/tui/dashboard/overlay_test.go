package dashboard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/styles"
)

func TestOverlayKeepsWhatSitsAroundTheBox(t *testing.T) {
	base := strings.Join([]string{
		"0123456789",
		"abcdefghij",
		"ABCDEFGHIJ",
	}, "\n")

	pasted := overlay(overlayParams{
		Base: base,
		Box:  "##\n##",
		At:   domain.Rect{X: 3, Y: 1},
	})

	want := []string{"0123456789", "abc##fghij", "ABC##FGHIJ"}
	for index, line := range strings.Split(pasted, "\n") {
		if line != want[index] {
			t.Errorf("line %d = %q, want %q", index, line, want[index])
		}
	}
}

func TestOverlayNeverGrowsTheFrame(t *testing.T) {
	base := strings.Join([]string{"short", strings.Repeat("x", 40), ""}, "\n")

	pasted := overlay(overlayParams{
		Base: base,
		Box:  strings.Repeat("#", 10) + "\n" + strings.Repeat("#", 10),
		At:   domain.Rect{X: 30, Y: 0},
	})

	lines := strings.Split(pasted, "\n")
	if len(lines) != 3 {
		t.Fatalf("%d lines, want the frame's 3", len(lines))
	}
	for index, line := range lines {
		if width := lipgloss.Width(line); width > 40 {
			t.Errorf("line %d is %d columns wide, want the frame's 40 at most", index, width)
		}
	}
}

// A box taller than what is left below its anchor is cut by the frame rather
// than pushing it: an extra row would scroll the alternate screen.
func TestOverlayStopsAtTheLastRow(t *testing.T) {
	pasted := overlay(overlayParams{
		Base: "one\ntwo",
		Box:  "#\n#\n#",
		At:   domain.Rect{X: 0, Y: 1},
	})

	if lines := strings.Split(pasted, "\n"); len(lines) != 2 {
		t.Fatalf("%d lines, want 2", len(lines))
	}
}

func TestOverlayKeepsStyledContentIntactAroundTheBox(t *testing.T) {
	base := styles.DashboardValue.Render("left") + styles.Bold.Render("RIGHT")

	pasted := overlay(overlayParams{Base: base, Box: "##", At: domain.Rect{X: 4}})

	if width := lipgloss.Width(pasted); width != lipgloss.Width(base) {
		t.Errorf("width = %d, want the base's %d", width, lipgloss.Width(base))
	}
	if !strings.Contains(pasted, "left") || !strings.Contains(pasted, "GHT") {
		t.Errorf("the styled text around the box was lost: %q", pasted)
	}
}

func TestTheViewStaysInsideTheTerminalWithAnOverlayOpen(t *testing.T) {
	sizes := [][2]int{{120, 40}, {80, 30}, {100, 12}, {60, 8}, {40, 6}}

	for _, size := range sizes {
		model := newTestModel(t, size[0], size[1], "a", "b", "c")
		model = update(model, key(domain.KeyMenu))
		assertFits(t, model.View(), size)

		model = update(model, namedKey(tea.KeyEsc))
		model = update(model, key(domain.KeyHelp))
		assertFits(t, model.View(), size)

		model, _ = openStepper(t, stepperSession())
		model = update(model, tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		assertFits(t, model.View(), size)
	}
}

func assertFits(t *testing.T, view string, size [2]int) {
	t.Helper()
	lines := strings.Split(view, "\n")
	if len(lines) > size[1] {
		t.Errorf("%dx%d: view is %d lines", size[0], size[1], len(lines))
	}
	for index, line := range lines {
		if width := lipgloss.Width(line); width > size[0] {
			t.Errorf("%dx%d: line %d is %d columns wide", size[0], size[1], index, width)
		}
	}
}

func TestTheHelpFloatsOverTheFrameLikeTheOtherOverlays(t *testing.T) {
	model := newTestModel(t, testWidth, testHeight, "a", "b")
	frame := model.View()

	model = update(model, key(domain.KeyHelp))
	view := model.View()

	if !strings.Contains(view, domain.DashboardHelpTitle) {
		t.Fatal("? must show the key reference")
	}
	if !strings.Contains(view, domain.DashboardListTitle) || !strings.Contains(view, domain.DashboardOutputTitle) {
		t.Error("the dashboard must stay visible around the help, as behind any other overlay")
	}
	if strings.Count(view, "\n") != strings.Count(frame, "\n") {
		t.Errorf("the help changed the frame's height: %d lines against %d",
			strings.Count(view, "\n"), strings.Count(frame, "\n"))
	}
	if _, ok := model.marks().(noMarks); !ok {
		t.Errorf("marks() = %T under the help, want the frame left unmarked", model.marks())
	}
}
