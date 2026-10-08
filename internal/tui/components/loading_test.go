package components

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// stubInterrupt stands in for the SIGINT the first Ctrl-C raises: it does what
// the root handler does with it, cancel the root context.
func stubInterrupt(t *testing.T, cancel context.CancelFunc) *atomic.Int32 {
	t.Helper()
	raised := &atomic.Int32{}
	previous := RaiseInterrupt
	RaiseInterrupt = func() error {
		raised.Add(1)
		cancel()
		return nil
	}
	t.Cleanup(func() { RaiseInterrupt = previous })
	return raised
}

func updateLoading(t *testing.T, m loadingModel, msg tea.Msg) (loadingModel, tea.Cmd) {
	t.Helper()
	next, cmd := m.Update(msg)
	lm, ok := next.(loadingModel)
	if !ok {
		t.Fatalf("Update returned %T, want loadingModel", next)
	}
	return lm, cmd
}

func isQuit(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, quit := cmd().(tea.QuitMsg)
	return quit
}

func newLoading(ctx context.Context) loadingModel {
	return loadingModel{ctx: ctx, spinner: newMutedSpinner(), message: "Loading worktrees…"}
}

func TestFirstCtrlCCancelsTheWorkAndKeepsWaitingForIt(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	raised := stubInterrupt(t, cancel)

	m, cmd := updateLoading(t, newLoading(ctx), key(tea.KeyCtrlC))

	if raised.Load() != 1 || ctx.Err() == nil {
		t.Fatal("the first ctrl+c did not cancel the root context")
	}
	if isQuit(cmd) {
		t.Fatal("the first ctrl+c quit before the work unwound")
	}
	if !strings.Contains(m.View(), domain.CancellingMessage) {
		t.Errorf("view = %q, want it to say the work is being cancelled", m.View())
	}

	m, cmd = updateLoading(t, m, loadingDoneMsg{err: context.Canceled})
	if !isQuit(cmd) || m.abandoned || !errors.Is(m.err, context.Canceled) {
		t.Errorf("the work ended and the loader did not return what it said: abandoned=%v err=%v", m.abandoned, m.err)
	}
}

func TestSecondCtrlCStopsWaiting(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	raised := stubInterrupt(t, cancel)

	m, _ := updateLoading(t, newLoading(ctx), key(tea.KeyCtrlC))
	m, cmd := updateLoading(t, m, key(tea.KeyCtrlC))

	if !isQuit(cmd) || !m.abandoned {
		t.Fatal("the second ctrl+c did not stop waiting")
	}
	if raised.Load() != 1 {
		t.Errorf("raised %d interrupts; a second one would reach the default handler with the terminal raw", raised.Load())
	}
}

// A context cancelled elsewhere — SIGTERM, a signal from another shell, an
// earlier wait's Ctrl-C — is already an interrupt: this one only says so, and
// raising another would be the second, fatal, SIGINT.
func TestAnInterruptedContextIsShownAndNotRaisedAgain(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	raised := stubInterrupt(t, func() {})
	cancel()

	m, _ := updateLoading(t, newLoading(ctx), InterruptedMsg{})
	if !strings.Contains(m.View(), domain.CancellingMessage) {
		t.Errorf("view = %q, want the cancellation shown", m.View())
	}

	m, cmd := updateLoading(t, m, key(tea.KeyCtrlC))
	if !isQuit(cmd) || !m.abandoned || raised.Load() != 0 {
		t.Errorf("ctrl+c under an interrupted run: quit=%v abandoned=%v raised=%d", isQuit(cmd), m.abandoned, raised.Load())
	}
}

func TestOtherKeysLeaveTheWorkAlone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	raised := stubInterrupt(t, cancel)

	m, cmd := updateLoading(t, newLoading(ctx), runeKey('q'))

	if cmd != nil || m.cancelling || raised.Load() != 0 {
		t.Error("a key other than ctrl+c touched the work")
	}
}

// The terminal delivers Ctrl-C as the byte 0x03 once it is raw; this drives a
// real program through that byte to the work's own context.
func TestCtrlCByteReachesTheWorkThroughTheProgram(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	stubInterrupt(t, cancel)

	m := newLoading(ctx)
	m.work = func() tea.Msg {
		select {
		case <-ctx.Done():
			return loadingDoneMsg{err: ctx.Err()}
		case <-time.After(5 * time.Second):
			return loadingDoneMsg{}
		}
	}

	input, feed := io.Pipe()
	program := tea.NewProgram(m, tea.WithInput(input), tea.WithOutput(&bytes.Buffer{}), tea.WithoutSignalHandler())
	go func() { _, _ = feed.Write([]byte{0x03}) }()

	final, err := program.Run()
	if err != nil {
		t.Fatalf("program: %v", err)
	}
	lm, ok := final.(loadingModel)
	if !ok || !errors.Is(lm.err, context.Canceled) {
		t.Fatalf("final = %#v, want the work to have ended on its cancelled context", final)
	}
}

func TestAWaitAbandonedOnTheSecondCtrlCExitsCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	stubInterrupt(t, cancel)

	m, _ := updateLoading(t, newLoading(ctx), key(tea.KeyCtrlC))
	m, _ = updateLoading(t, m, key(tea.KeyCtrlC))

	err := loadingOutcome(m)
	if !errors.Is(err, domain.ErrCancelled) || rules.ExitCode(err) != domain.ExitCodeCancelled {
		t.Errorf("err = %v (exit %d), want the cancelled exit code", err, rules.ExitCode(err))
	}
}

func TestABubbleteaInterruptIsACancellation(t *testing.T) {
	for _, err := range []error{tea.ErrInterrupted, tea.ErrProgramKilled} {
		if got := rules.ExitCode(ProgramError(err)); got != domain.ExitCodeCancelled {
			t.Errorf("%v exits %d, want %d", err, got, domain.ExitCodeCancelled)
		}
	}
}
