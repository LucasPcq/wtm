package runview

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

const clickedURL = "http://localhost:4012"

func publishing(name string) runlogs.JobView {
	view := running(name)
	view.Address.URL = clickedURL
	return view
}

// where finds text on the drawn frame, as the reader's pointer would.
func where(t *testing.T, view, text string) (x, y int) {
	t.Helper()
	for row, line := range strings.Split(view, "\n") {
		plain := ansi.Strip(line)
		if at := strings.Index(plain, text); at >= 0 {
			return ansi.StringWidth(plain[:at]), row
		}
	}
	t.Fatalf("%q is nowhere on the frame:\n%s", text, ansi.Strip(view))
	return 0, 0
}

// The view holds the mouse, so a click on an address never reaches the
// terminal: the view has to open it itself.
func TestAClickOnAnAddressOpensIt(t *testing.T) {
	h := newHarness(t, harnessParams{Views: []runlogs.JobView{publishing("web")}})
	opened := ""
	h.model.open = func(url string) error {
		opened = url
		return nil
	}
	x, y := where(t, h.model.View(), clickedURL)

	_, cmd := h.model.Update(tea.MouseMsg{X: x + 2, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease})
	if cmd == nil {
		t.Fatal("a click on the address produced no command")
	}
	cmd()

	if opened != clickedURL {
		t.Errorf("opened %q, want %q", opened, clickedURL)
	}
}

func TestAClickBesideAnAddressOpensNothing(t *testing.T) {
	h := newHarness(t, harnessParams{Views: []runlogs.JobView{publishing("web")}})
	h.model.open = func(string) error {
		t.Error("nothing should have been opened")
		return nil
	}
	x, y := where(t, h.model.View(), clickedURL)

	if _, cmd := h.model.Update(tea.MouseMsg{X: x - 3, Y: y, Button: tea.MouseButtonLeft, Action: tea.MouseActionRelease}); cmd != nil {
		cmd()
	}
}

// `run logs` starts nothing, so the address comes from the board alone.
func TestOpenKeyFallsBackToTheBoardAddress(t *testing.T) {
	h := newHarness(t, harnessParams{Views: []runlogs.JobView{publishing("web")}})
	opened := ""
	h.model.open = func(url string) error {
		opened = url
		return nil
	}

	cmd := h.model.openSelectedURL()
	if cmd == nil {
		t.Fatal("a job the board says publishes an address produced no command")
	}
	cmd()
	if opened != clickedURL {
		t.Errorf("opened %q, want %q", opened, clickedURL)
	}
}
