package shell

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAShieldIgnoresTheCancellationItWasMadeFrom(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	shielded, release := Shield(ctx)
	t.Cleanup(release)

	cancel()

	assert.NoError(t, shielded.Err())
	assert.Nil(t, shielded.Done())
}

func TestAShieldedChildRunsToTheEndThroughACancel(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	shielded, release := Shield(ctx)
	t.Cleanup(release)
	cmd := Command(shielded, "sh", "-c", "sleep 0.2; echo finished")

	cancel()
	out, err := cmd.Output()

	require.NoError(t, err)
	assert.Equal(t, "finished\n", string(out))
}

func TestAwaitShieldsHoldsUntilEveryShieldIsReleased(t *testing.T) {
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
