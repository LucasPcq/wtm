package runview

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/ansi"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

func event(phase runlogs.Phase, job string, step int) runlogs.Event {
	return runlogs.Event{Phase: phase, Job: job, Step: step, Steps: 3}
}

func (h *testHarness) emit(t *testing.T, events ...runlogs.Event) {
	t.Helper()
	for _, e := range events {
		model, cmd := updateCmd(h.model, eventMsg{event: e})
		h.model = h.follow(t, model, cmd)
	}
}

// startedHarness opens a view over a scripted start sequence and runs it to the
// end, the way Init does: the script emits on the sink it is handed, and what
// it emitted is drained into the model.
func startedHarness(t *testing.T, params harnessParams, script func(runlogs.Sink) runlogs.Outcome) *testHarness {
	t.Helper()

	streams := map[string]runlogs.Stream{}
	scripted := map[string]*runlogstest.Stream{}
	for _, name := range params.Streams {
		stream := runlogstest.NewStream()
		scripted[name] = stream
		streams[name] = stream
	}
	board := runlogstest.NewBoard(runlogstest.BoardParams{
		Views:   params.Views,
		Streams: streams,
		Lines:   params.Lines,
	})

	model := New(Params{
		Board: board,
		Job:   params.Job,
		Start: func(_ context.Context, emitter runlogs.Sink) (runlogs.Outcomes, error) {
			return runlogs.Outcomes{script(emitter)}, nil
		},
	})
	t.Cleanup(func() { model.panes.closeAll() })
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})

	h := &testHarness{model: model, board: board, streams: scripted}
	h.model = update(h.model, jobsMsg{jobs: params.Views})

	finished := h.model.startCmd()()
	h.drain(t)
	h.model = update(h.model, finished)
	return h
}

// drain applies everything the run posted, without waiting for a tick.
func (h *testHarness) drain(t *testing.T) {
	t.Helper()
	for {
		select {
		case msg := <-h.model.msgs:
			model, cmd := updateCmd(h.model, msg)
			h.model = h.follow(t, model, cmd)
		default:
			return
		}
	}
}

func TestSequenceProgressIsShownAsItRuns(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api"), running("web"), stopped("migrate")},
		Streams: []string{"api", "web"},
	})
	h.model.following = true

	h.emit(t, event(runlogs.PhaseStarting, "web", 2))

	frame := ansi.Strip(h.model.View())
	if !strings.Contains(frame, "starting 2/3 · web") {
		t.Fatalf("frame = %q, want the step the sequence is on", frame)
	}
	if h.model.selected != "web" {
		t.Fatalf("selected = %q, want the view to follow the job being started", h.model.selected)
	}
	if !strings.Contains(frame, domain.RunViewMarkStarting+" web") {
		t.Fatal("the job being started is not marked in the list")
	}
}

func TestSequenceMarksWhatEachJobDid(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{stopped("migrate"), running("api"), stopped("web")},
		Streams: []string{"api"},
	})

	h.emit(t,
		event(runlogs.PhaseStarting, "migrate", 1),
		event(runlogs.PhaseDone, "migrate", 1),
		event(runlogs.PhaseStarting, "api", 2),
		event(runlogs.PhaseStarted, "api", 2),
	)

	frame := ansi.Strip(h.model.View())
	if !strings.Contains(frame, domain.RunViewMarkDone+" migrate") {
		t.Fatalf("frame = %q, want the finished task marked done", frame)
	}
	// A job nothing was said about keeps the daemon's own mark.
	if !strings.Contains(frame, domain.RunViewMarkStopped+" web") {
		t.Fatalf("frame = %q, want an untouched job to keep its status mark", frame)
	}
}

// A job the run is streaming through its own events is not subscribed to as
// well: the daemon would replay what it already wrote, and the pane would show
// it twice.
func TestJobBeingStartedIsNotAttached(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})
	h.model.panes.release("api")
	h.model.following = true

	h.model = update(h.model, eventMsg{event: event(runlogs.PhaseStarting, "api", 1)})

	if _, cmd := h.model.fillSelectedPane(); cmd != nil {
		t.Fatal("a subscription was opened for a job the run is already feeding")
	}
	if h.model.pending != "" {
		t.Fatalf("pending = %q, want nothing asked of the daemon for that job", h.model.pending)
	}

	// Once the step is over the run stops writing, and the stream is what is
	// left to read the job with.
	h.model = update(h.model, eventMsg{event: event(runlogs.PhaseStarted, "api", 1)})
	if h.model.pending != "api" {
		t.Fatal("the job was never subscribed to once the run stopped feeding it")
	}
}

// The run's output lands in the pane on the goroutine that reports it — the
// clock is what draws it — and the events it does not carry come through the
// model.
// The pane a run is writing into is the only copy of those bytes: no
// subscription carries them and the log file is still being written. It is the
// one pane the cursor leaving does not drop.
func TestThePaneTheRunWritesIntoOutlivesTheCursorLeaving(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{stopped("migrate"), running("api")},
		Streams: []string{"api"},
	})
	h.emit(t, runlogs.Event{Phase: runlogs.PhaseStarting, Job: "migrate", Step: 1, Steps: 2})
	h.model.panes.write(writeChunkParams{Key: "migrate", Source: sourceSequence, Chunk: []byte("applying 3 migrations\r\n")})

	h.press(t, namedKey(tea.KeyDown))

	entry, held := h.model.panes.entry("migrate")
	if !held {
		t.Fatal("the run's own output went with the pane the cursor left")
	}
	if !strings.Contains(ansi.Strip(entry.pane.Render()), "applying 3 migrations") {
		t.Fatal("the pane was rebuilt under the run that is writing into it")
	}
}

func TestRunOutputReachesThePaneAndItsPhasesTheModel(t *testing.T) {
	h := startedHarness(t, harnessParams{
		Views: []runlogs.JobView{stopped("migrate")},
	}, func(emitter runlogs.Sink) runlogs.Outcome {
		emitter.Emit(runlogs.Event{Phase: runlogs.PhaseStarting, Job: "migrate", Step: 1, Steps: 1})
		emitter.Emit(runlogs.Event{Phase: runlogs.PhaseOutput, Job: "migrate", Chunk: []byte("applying 3 migrations\r\n")})
		emitter.Emit(runlogs.Event{Phase: runlogs.PhaseDone, Job: "migrate", Step: 1, Steps: 1})
		outcome := runlogs.Outcome{Completed: []string{"migrate"}, Steps: 1}
		emitter.Emit(runlogs.Event{Phase: runlogs.PhaseReady, Outcome: outcome})
		return outcome
	})

	if !strings.Contains(ansi.Strip(h.model.View()), "applying 3 migrations") {
		t.Fatal("the run's output never reached the pane")
	}
	if entry, held := h.model.panes.entry("migrate"); !held || entry.source != sourceSequence {
		t.Fatalf("pane source = %v, want the sequence that wrote it", entry.source)
	}
	if h.model.sequence.active {
		t.Fatal("the sequence is still reported as running after it concluded")
	}
	if got := h.model.sequence.outcomes.One(); len(got.Completed) != 1 || got.Completed[0] != "migrate" {
		t.Fatalf("outcome = %+v, want what the run reported", got)
	}
}

func abortedHarness(t *testing.T) *testHarness {
	t.Helper()
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api"), stopped("migrate"), stopped("web")},
		Streams: []string{"api"},
	})
	h.model.panes.writeLines(writeLinesParams{Key: "migrate", Lines: []string{"ERROR relation does not exist"}})
	h.model.following = true

	outcome := runlogs.Outcome{
		Started:    []string{"api"},
		NotStarted: []string{"web"},
		Failed:     "migrate",
		FailedStep: 2,
		Steps:      3,
	}
	h.emit(t,
		runlogs.Event{Phase: runlogs.PhaseFailed, Job: "migrate", Step: 2, Steps: 3, Reason: "exit status 1"},
		runlogs.Event{Phase: runlogs.PhaseAborted, Job: "migrate", Step: 2, Steps: 3, Reason: "exit status 1", Outcome: outcome},
	)
	return h
}

func TestAbortReportNamesTheFailureAndWhatIsLeft(t *testing.T) {
	h := abortedHarness(t)

	frame := ansi.Strip(h.model.View())
	for _, want := range []string{
		domain.RunViewAbortTitle,
		"failed at step 2/3: migrate — exit status 1",
		"left running: api",
		"not started: web",
		domain.RunViewAbortDismiss,
	} {
		if !strings.Contains(frame, want) {
			t.Fatalf("frame = %q, want %q", frame, want)
		}
	}
	if !strings.Contains(frame, domain.RunViewMarkCrashed+" migrate") {
		t.Fatal("the job that ended the sequence is not marked in the list")
	}
}

// The report is a band above the panes, not instead of them: what the job that
// failed printed is the first thing the reader will want under it.
func TestAbortReportKeepsThePanesOnScreen(t *testing.T) {
	h := abortedHarness(t)

	if !strings.Contains(ansi.Strip(h.model.View()), "ERROR relation does not exist") {
		t.Fatal("the failed job's output was pushed off the frame by the report")
	}
	if h.model.layout().PaneRows >= h.model.layout().Sidebar.Height {
		t.Fatal("the pane did not give the report the rows it is drawn in")
	}
}

func TestAbortReportIsDismissed(t *testing.T) {
	h := abortedHarness(t)
	rows := h.model.layout().PaneRows

	h.press(t, namedKey(tea.KeyEsc))

	if strings.Contains(ansi.Strip(h.model.View()), domain.RunViewAbortTitle) {
		t.Fatal("the report is still on screen after being dismissed")
	}
	if h.model.layout().PaneRows <= rows {
		t.Fatal("the rows the report held were not given back to the pane")
	}
	assertPaneMatchesLayout(t, h.model, h.model.selected)
	if !h.model.sequence.outcomes.Aborted() {
		t.Fatal("dismissing the report lost the outcome it was built from")
	}
}

// The band takes its rows from the body, so the emulators behind the panes have
// to be re-sized with it. A pane fed taller than it is drawn shows its oldest
// rows, and after an abort the newest ones are the whole point.
func TestAbortReportResizesThePanesItShortens(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api"), stopped("migrate")},
		Streams: []string{"api"},
	})
	for i := range 40 {
		h.streams["api"].Feed(fmt.Appendf(nil, "api line %02d\r\n", i))
	}
	h.waitForPane(t, "api", "api line 39")

	h.emit(t, runlogs.Event{
		Phase: runlogs.PhaseAborted, Job: "migrate", Step: 2, Steps: 2, Reason: "exit status 1",
		Outcome: runlogs.Outcome{Failed: "migrate", FailedStep: 2, Steps: 2, Started: []string{"api"}},
	})

	assertPaneMatchesLayout(t, h.model, "api")
	if !strings.Contains(ansi.Strip(h.model.View()), "api line 39") {
		t.Fatal("the newest output of the run that aborted fell off the frame")
	}
}

// esc is the reflex key for "never mind", and it is what closes the filter box.
// Pressing it before anything failed must not silence a report the run has not
// written yet.
func TestEscBeforeAnAbortDoesNotSilenceTheReport(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api"), stopped("migrate")},
		Streams: []string{"api"},
	})

	h.press(t, namedKey(tea.KeyEsc))
	h.emit(t, runlogs.Event{
		Phase: runlogs.PhaseAborted, Job: "migrate", Step: 2, Steps: 2, Reason: "exit status 1",
		Outcome: runlogs.Outcome{Failed: "migrate", FailedStep: 2, Steps: 2, Started: []string{"api"}},
	})

	if !strings.Contains(ansi.Strip(h.model.View()), domain.RunViewAbortTitle) {
		t.Fatal("the report never reached the frame: esc had dismissed it in advance")
	}
}

func assertPaneMatchesLayout(t *testing.T, model Model, job jobKey) {
	t.Helper()
	entry, held := model.panes.entry(job)
	if !held {
		t.Fatalf("%s has no pane", job)
	}
	layout := model.layout()
	if got := entry.pane.Size(); got.Rows != layout.PaneRows || got.Cols != layout.PaneCols {
		t.Fatalf("emulator of %s is %+v, the layout draws it at %dx%d", job, got, layout.PaneCols, layout.PaneRows)
	}
}

func TestRecapNamesWhatIsRunningAndHowToActOnIt(t *testing.T) {
	h := startedHarness(t, harnessParams{
		Views: []runlogs.JobView{running("api")},
	}, func(emitter runlogs.Sink) runlogs.Outcome {
		outcome := runlogs.Outcome{Started: []string{"api", "db"}, Completed: []string{"migrate"}, Steps: 3}
		emitter.Emit(runlogs.Event{Phase: runlogs.PhaseReady, Outcome: outcome})
		return outcome
	})

	recap := ansi.Strip(h.model.result().Recap)
	for _, want := range []string{"api, db", "migrate", domain.RunStreamAttachHint, domain.RunStreamStopHint} {
		if !strings.Contains(recap, want) {
			t.Fatalf("recap = %q, want %q", recap, want)
		}
	}
}

func TestRecapOfAnAbortNamesWhatDidNotRun(t *testing.T) {
	h := abortedHarness(t)
	h.model.started = true

	recap := ansi.Strip(h.model.result().Recap)
	for _, want := range []string{"Running:      api", "Failed:       migrate", "Not started:  web"} {
		if !strings.Contains(recap, want) {
			t.Fatalf("recap = %q, want %q", recap, want)
		}
	}
}

func TestRecapSaysWhenNothingIsLeftRunning(t *testing.T) {
	h := startedHarness(t, harnessParams{}, func(emitter runlogs.Sink) runlogs.Outcome {
		outcome := runlogs.Outcome{Completed: []string{"migrate"}, Steps: 1}
		emitter.Emit(runlogs.Event{Phase: runlogs.PhaseReady, Outcome: outcome})
		return outcome
	})

	recap := ansi.Strip(h.model.result().Recap)
	if !strings.Contains(recap, domain.RunViewRecapNoneRunning) {
		t.Fatalf("recap = %q, want it to say nothing is left running", recap)
	}
	if strings.Contains(recap, domain.RunStreamStopHint) {
		t.Fatal("the recap offers to stop jobs that are not running")
	}
}

// A view that started nothing has nothing to conclude: `wtm run logs` gives the
// terminal back as it found it.
func TestViewThatStartedNothingLeavesNoRecap(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	})

	if recap := h.model.result().Recap; recap != "" {
		t.Fatalf("recap = %q, want nothing", recap)
	}
}

// Detaching ends the reporting, not the run: the context the sequence is
// emitting on is cancelled, and nothing is asked to stop.
func TestDetachCancelsTheReportingOfARun(t *testing.T) {
	h := startedHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api")},
		Streams: []string{"api"},
	}, func(emitter runlogs.Sink) runlogs.Outcome {
		outcome := runlogs.Outcome{Started: []string{"api"}, Steps: 1}
		emitter.Emit(runlogs.Event{Phase: runlogs.PhaseReady, Outcome: outcome})
		return outcome
	})

	next, cmd := h.model.Update(key("q"))
	h.model = next.(Model)
	if _, quit := exec(t, cmd).(tea.QuitMsg); !quit {
		t.Fatal("q did not leave the view")
	}
	if h.model.runCtx.Err() == nil {
		t.Fatal("the run is still being reported to a view nobody is looking at")
	}
	if h.model.result().Recap == "" {
		t.Fatal("detaching lost the recap the run had already reported")
	}
}

// A sink emitting into a view that is gone must not hold the run's goroutine
// for ever.
func TestSinkGivesUpOnceTheViewIsGone(t *testing.T) {
	model := New(Params{
		Board: runlogstest.NewBoard(runlogstest.BoardParams{}),
		Start: func(context.Context, runlogs.Sink) (runlogs.Outcomes, error) { return runlogs.Outcomes{{}}, nil },
	})
	emitter := sink{panes: model.panes, msgs: model.msgs, done: model.runCtx.Done()}
	model.cancel()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range domain.RunViewMsgBuffer * 2 {
			emitter.Emit(runlogs.Event{Phase: runlogs.PhaseStarted, Job: "api"})
		}
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("the sink is still blocked on a channel nobody reads")
	}
}

func TestFollowingStopsWhenTheReaderTakesTheCursor(t *testing.T) {
	h := newHarness(t, harnessParams{
		Views:   []runlogs.JobView{running("api"), running("web")},
		Streams: []string{"api", "web"},
	})
	h.model.following = true

	h.press(t, namedKey(tea.KeyDown))
	h.emit(t, event(runlogs.PhaseStarting, "api", 1))

	if h.model.selected != "web" {
		t.Fatalf("selected = %q, want the cursor to stay where the reader put it", h.model.selected)
	}
}

// A run feeds panes from its own goroutine, so the clock has to be running
// before the first chunk lands and while nothing else is subscribed.
func TestRedrawClockRunsWhileTheRunDoes(t *testing.T) {
	model := New(Params{
		Board: runlogstest.NewBoard(runlogstest.BoardParams{}),
		Start: func(context.Context, runlogs.Sink) (runlogs.Outcomes, error) { return runlogs.Outcomes{{}}, nil },
	})
	t.Cleanup(model.cancel)

	if !model.ticking {
		t.Fatal("the clock was not armed for a view that starts a run")
	}
	model.sequence.active = true
	if _, cmd := updateCmd(model, frameMsg{}); cmd == nil {
		t.Fatal("the clock stopped while the run was still writing into panes")
	}

	model.sequence.active = false
	if _, cmd := updateCmd(model, frameMsg{}); cmd != nil {
		t.Fatal("the clock kept ticking once the run was over and nothing was subscribed")
	}
}

// A tick landing before the run reached its first job finds nothing to draw and
// stops the clock. The phase that opens the sequence has to arm it again, or
// the output the run writes straight into panes is never put on screen: no
// PhaseOutput ever reaches the model, so nothing else would.
func TestRedrawClockIsArmedAgainWhenTheSequenceStarts(t *testing.T) {
	model := New(Params{
		Board: runlogstest.NewBoard(runlogstest.BoardParams{Views: []runlogs.JobView{stopped("migrate")}}),
		Start: func(context.Context, runlogs.Sink) (runlogs.Outcomes, error) { return runlogs.Outcomes{{}}, nil },
	})
	t.Cleanup(model.cancel)
	model = update(model, tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	model = update(model, jobsMsg{jobs: []runlogs.JobView{stopped("migrate")}})

	model, cmd := updateCmd(model, frameMsg{})
	if cmd != nil || model.ticking {
		t.Fatal("the clock did not stop before the run reached its first job")
	}

	model = update(model, eventMsg{event: runlogs.Event{Phase: runlogs.PhaseStarting, Job: "migrate", Step: 1, Steps: 1}})

	if !model.ticking {
		t.Fatal("the sequence became active with no frame scheduled: what it writes into panes will never be drawn")
	}
}
