package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// interruptible returns a context the first SIGINT or SIGTERM cancels, and the function that stops listening.
// The second signal kills the children still stopping, waits for the shielded sections to finish and then dies of that signal; a third one dies at once.
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

// dieOn ends the process by sig with the default disposition, so the shell sees the process killed by that signal rather than an exit code.
func dieOn(sig os.Signal) {
	signal.Reset(sig)
	unix, ok := sig.(syscall.Signal)
	if !ok {
		os.Exit(ExitCancelled)
	}
	_ = syscall.Kill(os.Getpid(), unix)
}
