package dashboard

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// An address the output panel keeps declares no zone, and is still one the
// reader expects to follow: the dashboard holds the mouse, so the terminal
// never sees the click.
func TestClickingAnAddressNoZoneDeclaresOpensIt(t *testing.T) {
	const url = "http://localhost:4012"
	model := newTestModel(t, testWidth, testHeight, "a")
	opened := ""
	model.params.URLOpener = func(u string) error {
		opened = u
		return nil
	}
	model = update(model, OutputLineMsg{Text: "web → " + url})
	model.outputExpanded = true
	model = model.reflow()

	x, y, found := 0, 0, false
	for row, line := range strings.Split(model.View(), "\n") {
		plain := ansi.Strip(line)
		if at := strings.Index(plain, url); at >= 0 {
			x, y, found = ansi.StringWidth(plain[:at])+1, row, true
			break
		}
	}
	if !found {
		t.Fatalf("the address is not on the frame:\n%s", ansi.Strip(model.View()))
	}

	_, cmd := updateCmd(model, click(x, y))
	if cmd == nil {
		t.Fatal("clicking the address produced no command")
	}
	cmd()
	if opened != url {
		t.Errorf("opened %q, want %q", opened, url)
	}
}
