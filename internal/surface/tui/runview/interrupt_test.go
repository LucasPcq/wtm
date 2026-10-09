package runview

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/surface/tui/components"
	"github.com/LucasPcq/wtm/internal/testutil/runlogstest"
)

// stubInterrupt counts the SIGINTs the view raises instead of sending them to
// the test binary.
func stubInterrupt(t *testing.T) *atomic.Int32 {
	t.Helper()
	raised := &atomic.Int32{}
	previous := components.RaiseInterrupt
	components.RaiseInterrupt = func() error {
		raised.Add(1)
		return nil
	}
	t.Cleanup(func() { components.RaiseInterrupt = previous })
	return raised
}

var ctrlC = tea.KeyMsg{Type: tea.KeyCtrlC}

// The command's process is the one running the sequence: Ctrl-C there is the
// shell's interrupt, not a way out that hands the run to someone else.
func TestCtrlCCancelsTheRunAndWaitsForItToUnwind(t *testing.T) {
	raised := stubInterrupt(t)
	start := newStepwiseStart("migrate", "seed", "web")
	model := detachModel(t, start, Detach{Sink: &recorder{}, Await: true})

	p := newProgram(t, model)
	p.send(tea.WindowSizeMsg{Width: testWidth, Height: testHeight})
	start.next(t)

	p.send(ctrlC)
	p.waitFor("the view to leave once the run unwound", func(m Model) bool { return m.interrupted })

	if raised.Load() != 1 {
		t.Errorf("raised %d interrupts, want the one that cancels the root context", raised.Load())
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if !p.model.runDone {
		t.Error("the view left before the run had unwound")
	}
	if p.model.runCtx.Err() == nil {
		t.Error("ctrl+c did not cancel the run")
	}
}

func TestTheCancellationIsOnScreenWhileTheRunUnwinds(t *testing.T) {
	stubInterrupt(t)
	start := newStepwiseStart("migrate")
	model := detachModel(t, start, Detach{Sink: &recorder{}, Await: true})
	model.width, model.height = testWidth, testHeight

	next, cmd := model.interrupt()
	m, _ := next.(Model)
	if cmd != nil {
		t.Fatal("the first ctrl+c quit before the run had unwound")
	}
	if got := m.renderStatus(testWidth); !strings.Contains(got, domain.CancellingMessage) {
		t.Errorf("status = %q, want it to say the run is being cancelled", got)
	}
}

func TestSecondCtrlCStopsWaitingForTheRun(t *testing.T) {
	raised := stubInterrupt(t)
	start := newStepwiseStart("migrate")
	model := detachModel(t, start, Detach{Sink: &recorder{}, Await: true})

	next, _ := model.interrupt()
	next, cmd := next.(Model).interrupt()
	m, _ := next.(Model)

	if !m.interrupted || cmd == nil {
		t.Fatal("the second ctrl+c did not leave")
	}
	if _, quit := cmd().(tea.QuitMsg); !quit {
		t.Error("the second ctrl+c did not quit the program")
	}
	if raised.Load() != 1 {
		t.Errorf("raised %d interrupts; a second one would reach the default handler with the terminal raw", raised.Load())
	}
}

// A dashboard outlives the view: Ctrl-C keeps handing the run over, since
// raising an interrupt would take the dashboard down with it.
func TestCtrlCStillLeavesAViewADashboardHolds(t *testing.T) {
	raised := stubInterrupt(t)
	start := newStepwiseStart("migrate", "seed")
	model := detachModel(t, start, Detach{Sink: &recorder{}})

	next, _ := model.interrupt()
	m, _ := next.(Model)

	if raised.Load() != 0 || m.cancelling || m.runCtx.Err() != nil {
		t.Error("ctrl+c in a dashboard's view interrupted the run instead of leaving it")
	}
}

func TestQuitStillDetaches(t *testing.T) {
	raised := stubInterrupt(t)
	start := newStepwiseStart("migrate", "seed")
	model := detachModel(t, start, Detach{Sink: &recorder{}, Await: true})

	next, _ := model.handleKey(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(keyQuit)})
	m, _ := next.(Model)

	if raised.Load() != 0 || m.interrupted || m.runCtx.Err() != nil {
		t.Error("q interrupted the run; it hands it to the surface")
	}
}

func TestCtrlCOnAFinishedRunJustLeaves(t *testing.T) {
	raised := stubInterrupt(t)
	start := newStepwiseStart()
	model := detachModel(t, start, Detach{Sink: &recorder{}, Await: true})
	model.runDone = true

	next, _ := model.interrupt()
	m, _ := next.(Model)

	if raised.Load() != 0 || m.interrupted {
		t.Error("ctrl+c over a finished run reported an interrupt with nothing to interrupt")
	}
}

// The terminal delivers Ctrl-C as the byte 0x03 once it is raw. Through a real
// program, that byte has to end the run and come out as the cancelled exit
// code — before, it detached and the command waited for the whole profile.
func TestCtrlCByteEndsTheRunCancelled(t *testing.T) {
	stubInterrupt(t)
	start := newStepwiseStart("migrate", "seed")
	input, feed := io.Pipe()

	type ran struct {
		result Result
		err    error
	}
	done := make(chan ran, 1)
	go func() {
		result, err := Run(t.Context(), Params{
			Board:  runlogstest.NewBoard(runlogstest.BoardParams{Views: []runlogs.JobView{stopped("migrate")}}),
			Start:  start.run,
			Detach: Detach{Sink: &recorder{}, Await: true},
			In:     input,
			Out:    &bytes.Buffer{},
		})
		done <- ran{result: result, err: err}
	}()
	go func() { _, _ = feed.Write([]byte{0x03}) }()

	select {
	case got := <-done:
		if !errors.Is(got.err, domain.ErrCancelled) || rules.ExitCode(got.err) != domain.ExitCodeCancelled {
			t.Errorf("err = %v (exit %d), want the cancelled exit code", got.err, rules.ExitCode(got.err))
		}
		if got.result.Detached {
			t.Error("ctrl+c handed the run over instead of cancelling it")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ctrl+c never ended the view; the command would wait for the whole profile")
	}
}
