package cli

import (
	"context"
	"os"
	"os/signal"
	"syscall"
)

// interruptible returns a context the first SIGINT, SIGTERM or SIGHUP (the terminal closed) cancels, and the function that stops listening.
// The second signal kills the children still stopping, waits for the shielded sections to finish and then dies of that signal; a third one dies at once.
func interruptible(params Params) (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

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

// FailOnBrokenPipe makes a write to stdout or stderr whose reader left fail
// with EPIPE, as a write to any other file does, rather than kill the process
// with SIGPIPE: for a command that ends well when its reader leaves, wtm events
// piped to head. A signal handled rather than ignored is reset in the children
// the process starts.
func FailOnBrokenPipe() {
	signal.Notify(make(chan os.Signal, 1), syscall.SIGPIPE)
}
