package process

import (
	"strings"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/run"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestATaskPrintsBothStreamsOnOnePipeAndExitsWithItsCode(t *testing.T) {
	child, out := spawned(t, "echo out; echo err >&2; test -t 1 || echo piped; exit 3", false)

	assert.Equal(t, 3, child.Wait())
	assert.Equal(t, "out\nerr\npiped\n", out.String())
}

func TestAServiceRunsUnderATerminalOfItsOwnSized(t *testing.T) {
	child, out := spawned(t, "test -t 1 && stty size", true)

	assert.Equal(t, 0, child.Wait())
	assert.Equal(t, "40 120", strings.TrimSpace(out.String()))
}

func TestTheChildRunsWithItsEnvironmentAndNotTheVariablesUnset(t *testing.T) {
	t.Setenv("WTM_WORKTREE", "the daemon's")
	t.Setenv("WTM_KEPT", "inherited")
	out := &printed{}
	child, err := Spawn(run.Command{Line: `echo "${WTM_WORKTREE:-none} $WTM_KEPT $PORT $(pwd -P)"`, Dir: "/", Env: map[string]string{"PORT": "3100"}, Unset: []string{"WTM_WORKTREE"}, Output: out.write})
	require.NoError(t, err)

	assert.Equal(t, 0, child.Wait())
	assert.Equal(t, "none inherited 3100 /\n", out.String())
}

func TestStopEndsTheWholeGroup(t *testing.T) {
	child, _ := spawned(t, "sleep 30 & sleep 30", false)
	time.Sleep(100 * time.Millisecond)

	child.Stop(time.Second)

	assert.Equal(t, -1, child.Wait())
	assert.False(t, groupAlive(child.PID()), "a member of the group outlived the stop")
}

func TestStopKillsAChildDeafToSIGTERMOnceTheGracePassed(t *testing.T) {
	child, out := spawned(t, "trap '' TERM; echo ready; while :; do sleep 1; done", false)
	require.Eventually(t, func() bool { return out.String() == "ready\n" }, 5*time.Second, 10*time.Millisecond)

	started := time.Now()
	child.Stop(200 * time.Millisecond)

	assert.Less(t, time.Since(started), 3*time.Second)
	assert.Equal(t, -1, child.Wait())
}

func TestWaitReturnsWhatAChildLeftInTheBackgroundAside(t *testing.T) {
	child, _ := spawned(t, "sleep 30 & echo launched", false)

	started := time.Now()
	assert.Equal(t, 0, child.Wait())
	assert.Less(t, time.Since(started), 3*time.Second)
	assert.True(t, groupAlive(child.PID()), "the stack a launcher leaves runs on")
}
