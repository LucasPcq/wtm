package runview

import (
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

func focusKey() tea.KeyMsg { return namedKey(tea.KeyEnter) }

func exitFocusKey() tea.KeyMsg { return namedKey(tea.KeyEsc) }

func focusedHarness(t *testing.T, jobs ...string) *testHarness {
	t.Helper()
	views := make([]runlogs.JobView, 0, len(jobs))
	for _, job := range jobs {
		views = append(views, running(job))
	}
	h := newHarness(t, harnessParams{Views: views, Streams: jobs})
	h.press(t, focusKey())
	if !h.model.focused {
		t.Fatal("the pane did not take the keyboard")
	}
	return h
}

// keyTypeProbe walks Bubbletea's KeyType range in both directions: the control
// keys sit above zero, the named ones below, and neither bound is exported.
const keyTypeProbe = 500

// Every key Bubbletea can name has to reach the job in focus. One that encodes
// to nothing is a keystroke the reader watched go nowhere, which reads as a
// frozen terminal rather than as an unsupported key.
func TestEveryKeyBubbleteaCanNameReachesTheJob(t *testing.T) {
	for kind := tea.KeyType(-keyTypeProbe); kind < keyTypeProbe; kind++ {
		name := kind.String()
		if name == "" || kind == tea.KeyRunes {
			continue
		}
		if len(rules.EncodeKeyStroke(domain.KeyStroke{Name: name})) == 0 {
			t.Errorf("%s encodes to nothing: the job never hears it", name)
		}
	}
}

// In focus every key belongs to the job — Ctrl+C first of all, since
// interrupting the child is what focus is for.
func TestFocusSendsEveryKeyToTheJob(t *testing.T) {
	h := focusedHarness(t, "api", "web")

	h.press(t, key("a"))
	h.press(t, namedKey(tea.KeyCtrlC))
	h.press(t, namedKey(tea.KeyDown))
	h.press(t, namedKey(tea.KeyEnter))

	want := []string{"a", "\x03", "\x1b[B", "\r"}
	if got := h.streams["api"].Written(); !equalStrings(got, want) {
		t.Fatalf("the job received %q, want %q", got, want)
	}
	if h.model.selected != "api" {
		t.Fatalf("selected = %q: a key in focus moved the cursor instead of reaching the job", h.model.selected)
	}
	if len(h.streams["web"].Written()) != 0 {
		t.Fatal("a keystroke reached a job that is not the one in focus")
	}
}

// A lone exit key belongs to the job — vim leaves insert mode, a prompt
// clears. Only a second one right behind it is the reader asking to leave.
func TestOneExitKeyGoesToTheJobAndKeepsFocus(t *testing.T) {
	h := focusedHarness(t, "api")

	h.press(t, exitFocusKey())

	if !h.model.focused {
		t.Fatal("a single exit key gave the keyboard back, so the job can never receive one")
	}
	if written := strings.Join(h.streams["api"].Written(), ""); written != "\x1b" {
		t.Fatalf("the job received %q, want the escape it is entitled to", written)
	}
}

func TestTheExitKeyTwiceGivesTheKeyboardBack(t *testing.T) {
	h := focusedHarness(t, "api")

	h.press(t, exitFocusKey())
	h.press(t, exitFocusKey())

	if h.model.focused {
		t.Fatal("two exit keys did not give the keyboard back")
	}
	if written := strings.Join(h.streams["api"].Written(), ""); written != "\x1b" {
		t.Fatalf("the job received %q, want only the first of the two", written)
	}

	h.press(t, key("j"))
	if h.model.focused || strings.Join(h.streams["api"].Written(), "") != "\x1b" {
		t.Fatal("keys still reach the job after leaving focus")
	}
}

// Two escapes a minute apart are two keystrokes for the job, not a way out.
func TestExitKeysTooFarApartStayWithTheJob(t *testing.T) {
	h := focusedHarness(t, "api")

	h.press(t, exitFocusKey())
	h.model.lastExitKey = h.model.lastExitKey.Add(-domain.RunViewFocusExitWindow - time.Second)
	h.press(t, exitFocusKey())

	if !h.model.focused {
		t.Fatal("escapes a minute apart were read as a way out")
	}
	if written := strings.Join(h.streams["api"].Written(), ""); written != "\x1b\x1b" {
		t.Fatalf("the job received %q, want both escapes", written)
	}
}

// Anything typed in between makes them two keystrokes for the job.
func TestAKeyBetweenTheTwoExitKeysKeepsFocus(t *testing.T) {
	h := focusedHarness(t, "api")

	h.press(t, exitFocusKey())
	h.press(t, key("j"))
	h.press(t, exitFocusKey())

	if !h.model.focused {
		t.Fatal("an interrupted repeat still gave the keyboard back")
	}
	if written := strings.Join(h.streams["api"].Written(), ""); written != "\x1bj\x1b" {
		t.Fatalf("the job received %q, want every keystroke in order", written)
	}
}

func TestQuitKeysReachTheJobInFocus(t *testing.T) {
	h := focusedHarness(t, "api")

	next, cmd := h.model.Update(key("q"))
	h.model = next.(Model)
	if cmd == nil {
		t.Fatal("q reached neither the view nor the job")
	}
	if _, quit := cmd().(tea.QuitMsg); quit {
		t.Fatal("q left the view instead of reaching the job in focus")
	}
	if got := h.streams["api"].Written(); len(got) != 1 || got[0] != "q" {
		t.Fatalf("the job received %q, want the q it was typed", got)
	}
}

// A pane fed by a log file has no stdin to type into, and a keystroke going
// nowhere reads as a frozen terminal.
func TestFocusIsRefusedWithoutALiveStream(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views: []runlogs.JobView{stopped("migrate")},
		Lines: map[string][]string{"migrate": {"done"}},
	})

	h.press(t, focusKey())

	if h.model.focused {
		t.Fatal("focus was taken on a pane with no job behind it")
	}
	want := fmt.Sprintf(domain.RunViewNotAttachableFmt, "migrate")
	if !strings.Contains(ansi.Strip(h.model.View()), want) {
		t.Fatalf("frame does not carry the refusal %q", want)
	}

	h.press(t, key("j"))
	if strings.Contains(ansi.Strip(h.model.View()), want) {
		t.Fatal("the refusal stayed on screen past the keystroke it answered")
	}
}

func TestFocusIsNamedInTheFrame(t *testing.T) {
	h := focusedHarness(t, "api")

	frame := ansi.Strip(h.model.View())
	if !strings.Contains(frame, domain.RunViewFocusExitKey) {
		t.Fatalf("frame = %q, want the footer to name the key that leaves focus", frame)
	}
	if !strings.Contains(frame, domain.RunViewFocusLabel) {
		t.Fatal("nothing on the frame says the pane holds the keyboard")
	}
	if strings.Contains(frame, domain.RunViewHelpBrowse) {
		t.Fatal("the browse keys are still offered while the job has the keyboard")
	}
}

// The keyboard goes back to the view when there is nothing left to type into.
func TestFocusEndsWithTheStream(t *testing.T) {
	h := focusedHarness(t, "api")

	h.model = update(h.model, streamEndedMsg{key: "api"})

	if h.model.focused {
		t.Fatal("the keyboard was left with a job whose output ended")
	}
	if h.model.panes.hasStream() {
		t.Fatal("the subscription of a job that stopped writing was kept")
	}
	if _, held := h.model.panes.entry("api"); !held {
		t.Fatal("the pane was dropped with the stream: what the job printed is what is left of it")
	}
}

func TestMovingTheCursorLeavesFocus(t *testing.T) {
	h := focusedHarness(t, "api", "web")

	h.model, _ = h.model.setSelection("web")

	if h.model.focused {
		t.Fatal("the keyboard was left pointing at a job the cursor no longer shows")
	}
}

// x/vt never reflows: the job's own process is the only thing that can redraw
// at a new width, and it does not know about it until its PTY does.
func TestResizeReachesTheJobsPTY(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})

	model, cmd := updateCmd(h.model, tea.WindowSizeMsg{Width: 120, Height: 40})
	if cmd == nil {
		t.Fatal("resizing the window did not resize the job")
	}
	cmd()

	layout := model.layout()
	sizes := h.streams["api"].Sizes()
	if len(sizes) == 0 {
		t.Fatal("the job's PTY was never sized")
	}
	if last := sizes[len(sizes)-1]; last.Cols != layout.PaneCols || last.Rows != layout.PaneRows {
		t.Fatalf("PTY sized to %+v, want the pane's %dx%d", last, layout.PaneCols, layout.PaneRows)
	}

	if _, again := updateCmd(model, tea.WindowSizeMsg{Width: 120, Height: 40}); again != nil {
		t.Fatal("a window size that did not change was still sent to the daemon")
	}
}

// The window may move while the daemon is answering an attach, and the size the
// subscription opened with is the one it was asked for, not the one on screen.
func TestAttachCorrectsASizeTheWindowOutgrew(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})
	h.model.panes.release("api")

	stream := runlogstest.NewStream()
	t.Cleanup(func() { stream.Close() })
	model, cmd := updateCmd(h.model, attachedMsg{key: "api", stream: stream, size: PaneSize{Cols: 1, Rows: 1}})
	if cmd == nil {
		t.Fatal("an attach that opened at a stale size did not correct it")
	}
	cmd()

	layout := model.layout()
	sizes := stream.Sizes()
	if len(sizes) != 1 || sizes[0].Cols != layout.PaneCols || sizes[0].Rows != layout.PaneRows {
		t.Fatalf("PTY sized to %+v, want the pane's %dx%d", sizes, layout.PaneCols, layout.PaneRows)
	}

	sized := runlogstest.NewStream()
	t.Cleanup(func() { sized.Close() })
	if _, none := updateCmd(model, attachedMsg{key: "api", stream: sized, size: model.paneSize()}); none != nil {
		t.Fatal("an attach that opened at the right size was corrected anyway")
	}
}

func equalStrings(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for index := range got {
		if got[index] != want[index] {
			return false
		}
	}
	return true
}
