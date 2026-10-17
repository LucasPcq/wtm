package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/proc"
)

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

// dieOn ends the process by the signal, as the default handler would have, so
// the parent sees how it died.
func dieOn(sig os.Signal) {
	signal.Reset(sig)
	unix, ok := sig.(syscall.Signal)
	if !ok {
		os.Exit(domain.ExitCodeCancelled)
	}
	_ = syscall.Kill(os.Getpid(), unix)
}

func settle() {
	proc.KillCancelled()
	proc.AwaitShields()
}
