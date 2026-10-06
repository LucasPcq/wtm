package infra

import (
	"context"
	"syscall"
	"testing"
	"time"
)

// A shielded git is out of reach of the terminal's Ctrl-C, which goes to the
// foreground process group: it runs in a group of its own.
func TestAShieldedChildRunsInItsOwnProcessGroup(t *testing.T) {
	shielded, release := Shield(t.Context())
	defer release()
	cmd := Command(shielded, "sleep", "5")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	if pgid == syscall.Getpgrp() {
		t.Error("the shielded child shares wtm's process group, where the terminal's Ctrl-C lands")
	}
}

func TestAShieldIgnoresTheCancellationItWasMadeFrom(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	shielded, release := Shield(ctx)
	defer release()
	cancel()
	if shielded.Err() != nil || shielded.Done() != nil {
		t.Error("the shielded context was cancelled with its parent")
	}
}

func TestAwaitShieldsHoldsUntilEveryOneIsReleased(t *testing.T) {
	_, release := Shield(t.Context())
	released := make(chan struct{})
	time.AfterFunc(100*time.Millisecond, func() { close(released); release() })

	AwaitShields()

	select {
	case <-released:
	default:
		t.Fatal("AwaitShields returned before the shield was released")
	}
}
