package runview

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
)

func resize(model Model, width, height int) Model {
	return update(model, tea.WindowSizeMsg{Width: width, Height: height})
}

// The frame is what is written to a terminal holding nothing else: one line per
// row, none wider than the screen. A pane drawn one column too wide wraps every
// row under it and the whole view shears.
func TestFrameFitsTheTerminalExactly(t *testing.T) {
	sizes := []struct{ width, height int }{
		{120, 40}, {100, 24}, {80, 20}, {60, 15}, {40, 10}, {20, 6}, {8, 3},
	}
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api"), running("web"), stopped("migrate")},
		Streams: []string{"api", "web"},
	})
	h.streams["api"].Feed([]byte(strings.Repeat("a wide line of job output\r\n", 60)))
	h.waitForPane(t, "api", "a wide line")

	for _, size := range sizes {
		t.Run(fmt.Sprintf("%dx%d", size.width, size.height), func(t *testing.T) {
			frame := resize(h.model, size.width, size.height).View()
			lines := strings.Split(frame, "\n")
			if len(lines) != size.height {
				t.Fatalf("frame has %d lines, want %d", len(lines), size.height)
			}
			for index, line := range lines {
				if width := lipgloss.Width(line); width > size.width {
					t.Fatalf("line %d is %d columns wide, want at most %d: %q", index, width, size.width, line)
				}
			}
		})
	}
}

func TestNarrowTerminalDropsTheJobList(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})

	if !strings.Contains(ansi.Strip(resize(h.model, 100, 24).View()), domain.RunViewJobsTitle) {
		t.Fatal("the job list is missing on a wide terminal")
	}
	if strings.Contains(ansi.Strip(resize(h.model, 50, 24).View()), domain.RunViewJobsTitle) {
		t.Fatal("the job list was kept on a terminal too narrow to hold it beside a pane")
	}
}

func TestJobListCarriesMarkAndUptime(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api"), stopped("migrate")},
		Streams: []string{"api"},
	})

	frame := ansi.Strip(h.model.View())
	want := []string{
		domain.RunViewCursorMark + domain.RunViewMarkRunning + " api",
		domain.RunViewMarkStopped + " migrate",
		"1m",
	}
	for _, line := range want {
		if !strings.Contains(frame, line) {
			t.Fatalf("frame = %q, want %q", frame, line)
		}
	}
	// Uptime reads a job that is running: a stopped one dates a run that is over.
	if strings.Count(frame, "1m") != 1 {
		t.Fatal("an uptime was rendered for a job that is not running")
	}
}

func TestPaneTitleNamesTheJobAndWhereItsBytesComeFrom(t *testing.T) {
	live := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})
	if got := ansi.Strip(live.model.View()); !strings.Contains(got, "api · running") ||
		!strings.Contains(got, domain.RunViewPaneLiveLabel) {
		t.Fatalf("frame = %q, want the job, its status and its live origin", got)
	}

	history := newHarness(t, harnessParams{
		Views: []runlogs.JobView{stopped("api")},
		Lines: map[string][]string{"api": {"bye"}},
	})
	if got := ansi.Strip(history.model.View()); !strings.Contains(got, domain.RunViewPaneHistoryLabel) {
		t.Fatalf("frame = %q, want the pane to say it is showing the log file", got)
	}
}

func TestFooterSwitchesToTheFilterPrompt(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})

	if !strings.Contains(ansi.Strip(h.model.View()), domain.RunViewHelpBrowse) {
		t.Fatal("the footer does not carry the browse keys")
	}

	h.press(t, key("/"))
	h.press(t, key("a"))
	footer := ansi.Strip(h.model.View())
	if !strings.Contains(footer, domain.RunViewFilterPrompt+"a") {
		t.Fatalf("footer = %q, want the filter box and what was typed into it", footer)
	}
	if !strings.Contains(footer, domain.RunViewHelpFilter) {
		t.Fatal("the footer does not say how to leave the filter")
	}

	h.press(t, namedKey(tea.KeyEnter))
	if applied := ansi.Strip(h.model.View()); !strings.Contains(applied, "filter \"a\"") {
		t.Fatalf("footer = %q, want an applied filter to keep saying it is on", applied)
	}
}

// The emulator's whole point is that a job's colours survive; the frame must not
// strip or reset them on the way out.
func TestFrameKeepsTheJobsTruecolor(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})
	h.streams["api"].Feed([]byte("\x1b[38;2;255;0;0mfailed\x1b[0m\r\n"))
	h.waitForPane(t, "api", "failed")

	if !strings.Contains(h.model.View(), "38;2;255;0;0") {
		t.Fatal("the pane's truecolor was lost in the frame")
	}
}

// A frame too short for the whole report cuts it down. The line that says how
// to take the band off the screen is the one that has to survive: without it
// the reader is left with a report they cannot remove over a pane they came to
// read.
func TestShortFrameKeepsTheWayOutOfTheReport(t *testing.T) {
	h := abortedHarness(t)

	frame := ansi.Strip(resize(h.model, 40, 7).View())

	if !strings.Contains(frame, domain.RunViewAbortTitle) {
		t.Fatalf("frame = %q, want what the run has to say", frame)
	}
	if !strings.Contains(frame, domain.RunViewAbortDismiss) {
		t.Fatalf("frame = %q, want the line that dismisses the band", frame)
	}
}

// A pane is fed at the size it is drawn at: an emulator wider than its box
// pushes the border off the row.
func TestPanesAreSizedFromTheLayout(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})

	model := resize(h.model, 120, 40)
	layout := model.layout()
	entry, held := model.panes.entry("api")
	if !held {
		t.Fatal("the selected job has no pane")
	}
	if got := entry.pane.Size(); got.Cols != layout.PaneCols || got.Rows != layout.PaneRows {
		t.Fatalf("pane size = %+v, want the layout's %dx%d", got, layout.PaneCols, layout.PaneRows)
	}
}

// The first line a job writes is not a continuation of its name: the pane sets
// its output off from its title the way the sidebar sets the jobs off from
// theirs, and the emulator's row budget accounts for that blank line — a row
// the layout does not know about is a row the panel overflows by.
func TestPaneSetsTheOutputOffFromTheTitle(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views: []runlogs.JobView{stopped("migrate")},
		Lines: map[string][]string{"migrate": {"applied 3 migrations"}},
	})
	h.waitForPane(t, h.model.selected, "applied 3 migrations")

	lines := strings.Split(ansi.Strip(h.model.View()), "\n")
	title, output := -1, -1
	for i, line := range lines {
		if strings.Contains(line, "migrate ·") {
			title = i
		}
		if strings.Contains(line, "applied 3 migrations") {
			output = i
		}
	}
	if title < 0 || output < 0 {
		t.Fatalf("frame does not hold both the title and the output:\n%s", strings.Join(lines, "\n"))
	}
	if output != title+2 {
		t.Errorf("output on row %d, title on row %d: want one blank row between them", output, title)
	}
	if got := len(lines); got != h.model.height {
		t.Errorf("frame is %d rows, want the terminal's %d: the blank row overflowed it", got, h.model.height)
	}
}

// A url already carries the port it answers on, so a title spelling both says
// the same thing twice — and, in the dashboard's logs preview, contradicted the
// header one line above it, which had only ever shown the url.
func TestAPaneTitleShowsTheUrlAloneWhenTheRunObservedBoth(t *testing.T) {
	view := running("web")
	key := viewKey(view)

	model := Model{}
	model.sequence.ports = map[jobKey]map[string]int{key: {"PORT": 3010}}
	model.sequence.urls = map[jobKey]string{key: "http://web.dev-crm.shop.localhost"}

	status := model.statusWithAddress(view)
	if !strings.Contains(status, "http://web.dev-crm.shop.localhost") {
		t.Fatalf("status = %q, want the url the run observed", status)
	}
	if strings.Contains(status, "3010") {
		t.Errorf("status = %q, want no port beside the url that already carries one", status)
	}
}

// With no url to show, the ports are the address: they are what the reader has
// left to reach the job with.
func TestAPaneTitleFallsBackToThePortsWhenNoNameIsPublished(t *testing.T) {
	view := running("web")
	key := viewKey(view)

	model := Model{}
	model.sequence.ports = map[jobKey]map[string]int{key: {"PORT": 3010}}

	if status := model.statusWithAddress(view); !strings.Contains(status, "3010") {
		t.Errorf("status = %q, want the ports when nothing publishes a name", status)
	}
}
