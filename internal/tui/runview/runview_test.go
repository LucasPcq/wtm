package runview

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

const (
	testWidth  = 100
	testHeight = 24
)

func running(name string) runlogs.JobView {
	return runlogs.JobView{
		Name:       name,
		Kind:       domain.JobKindService,
		Status:     domain.JobStatusRunning,
		StartedAt:  time.Now().Add(-90 * time.Second),
		Attachable: true,
	}
}

func stopped(name string) runlogs.JobView {
	return runlogs.JobView{Name: name, Kind: domain.JobKindService, Status: domain.JobStatusStopped}
}

func update(model Model, msg tea.Msg) Model {
	next, _ := model.Update(msg)
	return next.(Model)
}

func updateCmd(model Model, msg tea.Msg) (Model, tea.Cmd) {
	next, cmd := model.Update(msg)
	return next.(Model), cmd
}

func key(name string) tea.KeyMsg { return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(name)} }

func namedKey(kind tea.KeyType) tea.KeyMsg { return tea.KeyMsg{Type: kind} }

// exec runs one command and returns what it produced. A nil command is a test
// failure: every caller here asserts on a message the model was supposed to ask
// for.
func exec(t *testing.T, cmd tea.Cmd) tea.Msg {
	t.Helper()
	if cmd == nil {
		t.Fatal("expected a command, got none")
	}
	return cmd()
}

type testHarness struct {
	model   Model
	board   *runlogstest.Board
	streams map[string]*runlogstest.Stream
}

type harnessParams struct {
	Views   []runlogs.JobView
	Lines   map[string][]string
	Job     string
	Streams []string
}

// newHarness opens a view over a scripted board and hands it the job list,
// including the create command that selecting the first job triggers.
func newHarness(t *testing.T, params harnessParams) *testHarness {
	t.Helper()

	streams := map[string]*runlogstest.Stream{}
	scripted := map[string]runlogs.Stream{}
	for _, name := range params.Streams {
		stream := runlogstest.NewStream()
		streams[name] = stream
		scripted[name] = stream
	}
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views:   params.Views,
		Streams: scripted,
		Lines:   params.Lines,
	})

	model := New(Params{Board: board, Job: params.Job})
	t.Cleanup(func() { model.panes.closeAll() })
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})

	harness := &testHarness{model: model, board: board, streams: streams}
	harness.load(t, params.Views)
	return harness
}

// load delivers a job list and follows through whatever it asked for — the
// attach or the history read that fills the selected job's pane.
func (h *testHarness) load(t *testing.T, views []runlogs.JobView) {
	t.Helper()
	h.board.SetViews(views)
	model, cmd := updateCmd(h.model, jobsMsg{jobs: views})
	h.model = h.follow(t, model, cmd)
}

func (h *testHarness) follow(t *testing.T, model Model, cmd tea.Cmd) Model {
	t.Helper()
	if cmd == nil {
		return model
	}
	return update(model, cmd())
}

func (h *testHarness) press(t *testing.T, msg tea.KeyMsg) {
	t.Helper()
	next, cmd := h.model.Update(msg)
	h.model = h.follow(t, next.(Model), cmd)
}

// waitForPane blocks until the job's pane shows the text, which a reader
// goroutine writes into it off the render loop.
func (h *testHarness) waitForPane(t *testing.T, job jobKey, want string) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		entry, held := h.model.panes.entry(job)
		if held && strings.Contains(ansi.Strip(entry.pane.Render()), want) {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("pane of %s never showed %q", job, want)
}

func TestSelectionStartsOnTheRequestedJob(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api"), running("web")},
		Job:     "web",
		Streams: []string{"api", "web"},
	})

	if h.model.selected != "web" {
		t.Fatalf("selected = %q, want the job the view was opened on", h.model.selected)
	}
	if got := h.board.AttachedJobs(); len(got) != 1 || got[0] != "web" {
		t.Fatalf("attached %v, want only the selected job", got)
	}
}

func TestSelectionFallsBackToTheFirstJob(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api"), running("web")},
		Job:     "gone",
		Streams: []string{"api", "web"},
	})

	if h.model.selected != "api" {
		t.Fatalf("selected = %q, want the first job when the requested one is unknown", h.model.selected)
	}
}

func TestArrowsMoveTheSelectionAndStopAtTheEnds(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api"), running("web"), running("db")},
		Streams: []string{"api", "web", "db"},
	})

	h.press(t, namedKey(tea.KeyUp))
	if h.model.selected != "api" {
		t.Fatalf("selected = %q, want the first job to hold at the top", h.model.selected)
	}
	h.press(t, namedKey(tea.KeyDown))
	h.press(t, key("j"))
	if h.model.selected != "db" {
		t.Fatalf("selected = %q, want down twice to reach the third job", h.model.selected)
	}
	h.press(t, key("j"))
	if h.model.selected != "db" {
		t.Fatalf("selected = %q, want the last job to hold at the bottom", h.model.selected)
	}
}

func TestLiveChunksReachTheSelectedPane(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})

	h.streams["api"].Feed([]byte("listening on :3000\r\n"))
	h.waitForPane(t, "api", "listening on :3000")

	if !strings.Contains(ansi.Strip(h.model.View()), "listening on :3000") {
		t.Fatal("the frame does not show what the pane holds")
	}
}

// A job with no live stream reads back its log file instead, and is never
// subscribed to: the daemon has nothing to hand out for it.
func TestStoppedJobShowsItsHistoryAndIsNeverAttached(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views: []runlogs.JobView{stopped("migrate")},
		Lines: map[string][]string{"migrate": {"applied 3 migrations", "done"}},
	})

	frame := ansi.Strip(h.model.View())
	for _, want := range []string{"applied 3 migrations", "done", domain.RunViewPaneHistoryLabel} {
		if !strings.Contains(frame, want) {
			t.Fatalf("frame = %q, want %q from the log file", frame, want)
		}
	}
	if got := h.board.AttachedJobs(); len(got) != 0 {
		t.Fatalf("attached %v, want nothing for a job with no live stream", got)
	}
}

// The pane switches sides when the job does: a job that was read from its log
// file is attached as soon as it runs, and the file's lines are not left under
// the stream's replay.
func TestPaneSwitchesFromHistoryToLive(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{stopped("api")},
		Lines:   map[string][]string{"api": {"exited yesterday"}},
		Streams: []string{"api"},
	})
	if entry, _ := h.model.panes.entry("api"); entry.source != sourceHistory {
		t.Fatalf("source = %v, want the log file for a stopped job", entry.source)
	}

	h.load(t, []runlogs.JobView{running("api")})

	entry, held := h.model.panes.entry("api")
	if !held || entry.source != sourceLive {
		t.Fatalf("source = %v (held=%v), want the live stream once the job runs", entry.source, held)
	}
	if strings.Contains(ansi.Strip(entry.pane.Render()), "exited yesterday") {
		t.Fatal("the log file's lines were left in the pane the stream now replays into")
	}
}

// Only the selected job holds a subscription: a second one would double a job's
// output on screen and size its PTY behind the pane being read.
func TestLeavingAJobReleasesItsStream(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api"), running("web")},
		Streams: []string{"api", "web"},
	})

	h.press(t, namedKey(tea.KeyDown))

	if !h.streams["api"].Closed() {
		t.Fatal("the stream of the job the cursor left is still subscribed")
	}
	if h.streams["web"].Closed() {
		t.Fatal("the stream of the selected job was closed")
	}
	if _, held := h.model.panes.entry("api"); held {
		t.Fatal("the pane of the job the cursor left was kept, holding its cells for nothing")
	}
}

// An attach that lands after the cursor moved on belongs to nobody: left open
// it would hold the job's output queue and its PTY size.
// A pane fed by the log file is dropped like any other when the cursor leaves
// it: it costs a styled cell per column ever written, and one History call
// builds it again.
func TestLeavingAStoppedJobDropsItsHistoryPane(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{stopped("migrate"), running("api")},
		Lines:   map[string][]string{"migrate": {"ERROR relation does not exist"}},
		Streams: []string{"api"},
	})
	if _, held := h.model.panes.entry("migrate"); !held {
		t.Fatal("the stopped job never got a pane")
	}

	h.press(t, namedKey(tea.KeyDown))

	if _, held := h.model.panes.entry("migrate"); held {
		t.Fatal("the log-file pane outlived the cursor that left it")
	}

	h.press(t, namedKey(tea.KeyUp))

	if got := len(h.board.HistoryParams()); got != 2 {
		t.Fatalf("History was called %d times, want the pane rebuilt from the file", got)
	}
	if !strings.Contains(ansi.Strip(h.model.View()), "ERROR relation does not exist") {
		t.Fatal("the pane came back without what the log file holds")
	}
}

func TestAttachForAJobTheCursorLeftIsClosed(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api"), running("web")},
		Streams: []string{"api", "web"},
	})
	h.press(t, namedKey(tea.KeyDown))

	late := runlogstest.NewStream()
	h.model = update(h.model, attachedMsg{key: "api", stream: late})

	if !late.Closed() {
		t.Fatal("a stream that arrived for a job the cursor left was kept open")
	}
	if entry, held := h.model.panes.entry("api"); held && entry.source == sourceLive {
		t.Fatal("a late stream installed a pane for a job that is not selected")
	}
}

// The subscription carries the size the job's PTY is opened at, so what the
// job draws matches the pane it is drawn in from its first byte. Without it the
// job wraps its output against the daemon's default and the emulator only ever
// sees the wrapped text.
func TestAttachCarriesThePaneSizeItOpensAt(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})

	attached := h.board.AttachParams()
	if len(attached) != 1 {
		t.Fatalf("Attach was called %d times, want once", len(attached))
	}
	layout := h.model.layout()
	if layout.PaneCols == 0 || layout.PaneRows == 0 {
		t.Fatal("the layout left the pane no size to attach at")
	}
	if want := (runlogs.Size{Cols: layout.PaneCols, Rows: layout.PaneRows}); attached[0].Size != want {
		t.Fatalf("attached at %+v, want the layout's %+v", attached[0].Size, want)
	}
}

// A job list arriving while an attach is in flight must not start a second one:
// the pane would be rebuilt under the reader, the daemon would replay the job's
// history into it again, and the first subscription would be closed from under
// the goroutine reading it.
func TestAJobListArrivingMidAttachDoesNotAttachTwice(t *testing.T) {
	stream := runlogstest.NewStream()
	views := []runlogs.JobView{running("api")}
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views:   views,
		Streams: map[string]runlogs.Stream{"api": stream},
	})

	model := New(Params{Board: board})
	t.Cleanup(func() {
		model.cancel()
		model.panes.closeAll()
	})
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})

	model, attach := updateCmd(model, jobsMsg{jobs: views})
	if attach == nil {
		t.Fatal("the selected job was never attached")
	}
	if model.pending != "api" {
		t.Fatalf("pending = %q, want the job whose attach is in flight", model.pending)
	}

	if _, cmd := updateCmd(model, jobsMsg{jobs: views}); cmd != nil {
		t.Fatal("the poll started a second attach behind the one in flight")
	}
	if attached := board.AttachedJobs(); len(attached) != 0 {
		t.Fatalf("Attach ran for %v before the first one answered", attached)
	}

	update(model, exec(t, attach))
	if attached := board.AttachedJobs(); len(attached) != 1 {
		t.Fatalf("Attach ran for %v, want api once", attached)
	}
}

func TestDetachClosesEveryStreamAndQuits(t *testing.T) {
	for _, name := range []string{"q", "ctrl+c"} {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, harnessParams{
				Views:   []runlogs.JobView{running("api")},
				Streams: []string{"api"},
			})

			next, cmd := h.model.Update(keyOf(name))
			h.model = next.(Model)
			if cmd == nil || cmd() != tea.Quit() {
				t.Fatal("want the view to quit")
			}
			if !h.streams["api"].Closed() {
				t.Fatal("a stream was left subscribed after detaching")
			}
		})
	}
}

func keyOf(name string) tea.KeyMsg {
	if name == "ctrl+c" {
		return namedKey(tea.KeyCtrlC)
	}
	return key(name)
}

func TestFilterNarrowsTheListAndMovesTheSelection(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api"), running("web"), running("webhook")},
		Streams: []string{"api", "web", "webhook"},
	})

	h.press(t, key("/"))
	h.press(t, key("W"))

	if !h.model.filtering {
		t.Fatal("the filter box did not open")
	}
	if h.model.selected != "web" {
		t.Fatalf("selected = %q, want the filter to move the cursor off a hidden job", h.model.selected)
	}
	if names := visibleNames(h.model); len(names) != 2 || names[0] != "web" || names[1] != "webhook" {
		t.Fatalf("visible = %v, want the case-insensitive matches", names)
	}

	h.press(t, namedKey(tea.KeyEsc))
	if h.model.filtering || h.model.filter != "" {
		t.Fatalf("esc left filtering=%v filter=%q", h.model.filtering, h.model.filter)
	}
	if len(visibleNames(h.model)) != 3 {
		t.Fatal("esc did not give the whole list back")
	}
}

func TestFilterWithoutAMatchSaysSoAndSelectsNothing(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})

	h.press(t, key("/"))
	h.press(t, key("z"))

	if h.model.selected != "" {
		t.Fatalf("selected = %q, want nothing selected when nothing matches", h.model.selected)
	}
	if want := "No job matches \"z\""; !strings.Contains(ansi.Strip(h.model.View()), want) {
		t.Fatalf("frame does not carry %q", want)
	}
}

func TestBackspaceWidensTheFilter(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api"), running("web")},
		Streams: []string{"api", "web"},
	})

	h.press(t, key("/"))
	h.press(t, key("w"))
	h.press(t, namedKey(tea.KeyBackspace))

	if h.model.filter != "" {
		t.Fatalf("filter = %q, want backspace to clear the last rune", h.model.filter)
	}
	if len(visibleNames(h.model)) != 2 {
		t.Fatal("the list did not widen back")
	}
}

func TestKeysDoNotDriveTheViewWhileFiltering(t *testing.T) {
	// Both jobs match the keystroke, so the cursor has no reason to move — and
	// would land on the second one if "j" were still read as a down key.
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("job-a"), running("job-b")},
		Streams: []string{"job-a", "job-b"},
	})

	h.press(t, key("/"))
	h.press(t, key("j"))

	if h.model.filter != "j" {
		t.Fatalf("filter = %q, want the keystroke to land in the box", h.model.filter)
	}
	if h.model.selected != "job-a" {
		t.Fatalf("selected = %q, want a keystroke typed into the box not to move the cursor", h.model.selected)
	}
}

func visibleNames(model Model) []string {
	names := make([]string, 0, len(model.jobs))
	for _, view := range model.visible() {
		names = append(names, view.Name)
	}
	return names
}

func TestEmptyJobListNamesRunToml(t *testing.T) {
	h := newHarness(t, harnessParams{})

	frame := ansi.Strip(h.model.View())
	if !strings.Contains(frame, domain.RunViewEmptyMessage) {
		t.Fatalf("frame = %q, want the empty-state message", frame)
	}
}

func TestRefreshErrorIsShownAndNotFatal(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})

	h.model = update(h.model, jobsMsg{jobs: h.model.jobs, err: errors.New("daemon is not running")})

	if !strings.Contains(ansi.Strip(h.model.View()), "daemon is not running") {
		t.Fatal("the frame does not carry the refresh failure")
	}
}

// The redraw clock runs while a stream feeds a pane and stops once none does,
// so an idle view schedules no further frames.
func TestRedrawClockRunsOnlyWhileAStreamFeeds(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})

	if !h.model.ticking {
		t.Fatal("attaching a stream did not start the redraw clock")
	}
	if _, cmd := updateCmd(h.model, frameMsg{}); cmd == nil {
		t.Fatal("the clock stopped while a stream was still feeding a pane")
	}

	h.model.panes.closeAll()
	next, cmd := updateCmd(h.model, frameMsg{})
	if cmd != nil {
		t.Fatal("the clock kept ticking with nothing left to draw")
	}
	if next.ticking {
		t.Fatal("the model still believes a frame is scheduled")
	}
}

// A model too busy to drain its mailbox must not lose the end of a stream:
// nothing else reports it, so the subscription would stay registered for the
// life of the view — a dead pane redrawn on the clock, and a focus writing
// keystrokes into a closed stream.
func TestStreamEndIsNotLostWhenTheMailboxIsFull(t *testing.T) {
	stream := runlogstest.NewStream()
	msgs := make(chan tea.Msg, domain.RunViewMsgBuffer)
	for range cap(msgs) {
		msgs <- frameMsg{}
	}

	returned := make(chan struct{})
	go func() {
		defer close(returned)
		readStream(readParams{
			Key:    "api",
			Stream: stream,
			Pane:   NewPane(PaneParams{}),
			Msgs:   msgs,
			Done:   make(chan struct{}),
		})
	}()
	stream.Close()

	select {
	case <-returned:
		t.Fatal("the reader gave the end of the stream up rather than wait for room")
	case <-time.After(100 * time.Millisecond):
	}

	for {
		select {
		case msg := <-msgs:
			if _, isEnd := msg.(streamEndedMsg); isEnd {
				return
			}
		case <-time.After(2 * time.Second):
			t.Fatal("the mailbox emptied without the end of the stream ever arriving")
		}
	}
}

// The wait for the mailbox has to end somewhere. Leaving the view cancels its
// context, and a reader still holding its last message gives up there rather
// than outliving the program.
func TestStreamReaderGivesUpOnceTheViewIsGone(t *testing.T) {
	stream := runlogstest.NewStream()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	returned := make(chan struct{})
	go func() {
		defer close(returned)
		readStream(readParams{
			Key:    "api",
			Stream: stream,
			Pane:   NewPane(PaneParams{}),
			Msgs:   make(chan tea.Msg),
			Done:   ctx.Done(),
		})
	}()

	stream.Close()
	cancel()

	select {
	case <-returned:
	case <-time.After(2 * time.Second):
		t.Fatal("the reader is still offering the end of a stream nobody will take")
	}
}

func TestRefreshCommandReadsTheBoardAgain(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})
	before := h.board.Refreshes()

	msg, ok := exec(t, h.model.refreshCmd()).(jobsMsg)
	if !ok {
		t.Fatal("the refresh command did not answer with a job list")
	}
	if h.board.Refreshes() != before+1 {
		t.Fatal("the refresh command did not re-read the daemon's view")
	}
	if len(msg.jobs) != 1 || msg.jobs[0].Name != "api" {
		t.Fatalf("jobs = %v, want what the board lists", msg.jobs)
	}
}

func TestScrollKeysMoveThroughThePanesHistory(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})
	h.streams["api"].Feed([]byte(strings.Repeat("line\r\n", 200)))
	h.waitForPane(t, "api", "line")

	h.press(t, namedKey(tea.KeyPgUp))
	entry, _ := h.model.panes.entry("api")
	if entry.pane.ScrollOffset() == 0 {
		t.Fatal("pgup did not move the pane back through its history")
	}
	if !strings.Contains(ansi.Strip(h.model.View()), "scrolled") {
		t.Fatal("the pane's title does not say it is scrolled back")
	}

	h.press(t, key("G"))
	if entry.pane.ScrollOffset() != 0 {
		t.Fatal("G did not bring the pane back to the live tail")
	}
}
