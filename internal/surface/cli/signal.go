package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/proc"
)

// interruptible is the root context. The first interrupt cancels it; the
// second ends the process — once no shielded git step is left half done, and
// after killing whatever a cancellation is still waiting on.
func interruptible() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		cancel()
		second := <-signals
		proc.KillCancelled()
		go func() { dieOn(<-signals) }()
		proc.AwaitShields()
		dieOn(second)
	}()
	stop := func() {
		signal.Stop(signals)
		cancel()
	}
	return ctx, stop
}

// dieOn ends the process the way the default handler would have: by the signal.
func dieOn(sig os.Signal) {
	signal.Reset(sig)
	unix, ok := sig.(syscall.Signal)
	if !ok {
		os.Exit(domain.ExitCodeCancelled)
	}
	_ = syscall.Kill(os.Getpid(), unix)
}

// settle is the last thing a run does: nothing it started may outlive it, and
// nothing shielded may be cut off half-way by the exit.
func settle() {
	proc.KillCancelled()
	proc.AwaitShields()
}
