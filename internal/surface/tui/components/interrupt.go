package components

import (
	"context"
	"errors"
	"fmt"
	"os"
	"syscall"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/LucasPcq/wtm/internal/domain"
)

// RaiseInterrupt hands a Ctrl-C a raw terminal delivered as a key back to the
// process as the SIGINT it would have been, so the root context's handler
// treats it like any other interrupt. Only this process gets it: a child is
// stopped through its context. A variable so a test can stand in for it.
var RaiseInterrupt = func() error {
	return syscall.Kill(os.Getpid(), syscall.SIGINT)
}

// Interrupt is the first Ctrl-C a raw-mode program reads: raised as SIGINT, it
// cancels the root context and the work under it unwinds as it would on a
// cooked terminal. Once that context is done the handler has been released and
// a second SIGINT would kill the process with the terminal still raw, so the
// caller's second Ctrl-C quits the program instead of coming here.
func Interrupt(ctx context.Context) {
	if ctx.Err() != nil {
		return
	}
	_ = RaiseInterrupt()
}

// InterruptedMsg says the context a program waits under is done.
type InterruptedMsg struct{}

func AwaitInterrupt(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		<-ctx.Done()
		return InterruptedMsg{}
	}
}

// ProgramError reads a program bubbletea stopped on a signal or a cancelled
// context as the cancellation it is.
func ProgramError(err error) error {
	if errors.Is(err, tea.ErrInterrupted) || errors.Is(err, tea.ErrProgramKilled) {
		return fmt.Errorf("%w: %w", domain.ErrCancelled, err)
	}
	return err
}
