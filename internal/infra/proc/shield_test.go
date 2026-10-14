package proc

import (
	"context"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A shielded git is out of reach of the terminal's Ctrl-C, which goes to the
// foreground process group: it runs in a group of its own.
func TestAShieldedChildRunsInItsOwnProcessGroup(t *testing.T) {
	shielded, release := Shield(t.Context())
	defer release()
	cmd := Command(shielded, "sleep", "5")
	require.NoError(t, cmd.Start())
	defer func() { _ = cmd.Process.Kill(); _ = cmd.Wait() }()

	pgid, err := syscall.Getpgid(cmd.Process.Pid)

	require.NoError(t, err)
	assert.NotEqual(t, syscall.Getpgrp(), pgid, "the shielded child shares wtm's process group, where the terminal's Ctrl-C lands")
}

func TestAShieldIgnoresTheCancellationItWasMadeFrom(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	shielded, release := Shield(ctx)
	defer release()

	cancel()

	assert.NoError(t, shielded.Err(), "the shielded context was cancelled with its parent")
	assert.Nil(t, shielded.Done(), "the shielded context was cancelled with its parent")
}

func TestAwaitShieldsHoldsUntilEveryOneIsReleased(t *testing.T) {
	_, release := Shield(t.Context())
	released := make(chan struct{})
	time.AfterFunc(100*time.Millisecond, func() { close(released); release() })

	AwaitShields()

	select {
	case <-released:
	default:
		require.FailNow(t, "AwaitShields returned before the shield was released")
	}
}
