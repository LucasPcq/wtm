package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

func interruptible(params Params) (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-signals
		cancel()
		second := <-signals
		params.KillCancelled()
		go func() { dieOn(<-signals) }()
		params.AwaitShields()
		dieOn(second)
	}()
	return ctx, func() {
		signal.Stop(signals)
		cancel()
	}
}

func dieOn(sig os.Signal) {
	signal.Reset(sig)
	unix, ok := sig.(syscall.Signal)
	if !ok {
		os.Exit(ExitCancelled)
	}
	_ = syscall.Kill(os.Getpid(), unix)
}
