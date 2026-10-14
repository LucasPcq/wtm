package proc

import (
	"context"
	"fmt"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/domain"
)

// A git or a hook interrupted the way a terminal would interrupt it gets to
// clean up after itself: its lock file, its half-written ref.
func TestCommandInterruptsFirst(t *testing.T) {
	run := runCancelled(t, `trap 'echo int; exit 3' INT; while :; do sleep 0.05; done`)

	require.Contains(t, run.Output, "int", "the child never saw SIGINT")
	require.Less(t, run.Elapsed, domain.SubprocessInterruptGrace, "the cancel took %v, want it well under the grace", run.Elapsed)
}

func TestCommandTerminatesAChildThatIgnoresTheInterrupt(t *testing.T) {
	run := runCancelled(t, `trap '' INT; trap 'echo term; exit 4' TERM; while :; do sleep 0.05; done`)

	require.Contains(t, run.Output, "term", "the child never saw SIGTERM")
}

// A child that runs in its own group takes its whole tree with it: killing the
// shell alone would leave what it started holding the run's pipes.
func TestGroupCommandSignalsTheWholeGroup(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cmd := GroupCommand(ctx, "sh", "-c", `sleep 30 & echo $!; wait`)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	grandchild := readPID(t, stdout)

	cancel()
	_ = cmd.Wait()

	deadline := time.Now().Add(domain.SubprocessInterruptGrace)
	for syscall.Kill(grandchild, 0) == nil {
		if time.Now().After(deadline) {
			_ = syscall.Kill(grandchild, syscall.SIGKILL)
			require.FailNow(t, "the grandchild outlived the cancelled group")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestCommandWithALiveContextRunsToTheEnd(t *testing.T) {
	out, err := Command(t.Context(), "sh", "-c", "echo done").Output()

	require.NoError(t, err)
	assert.Equal(t, "done", strings.TrimSpace(string(out)))
}

// git does not stop the ssh it started when it is stopped itself: the
// cancellation reaches what the child started too.
func TestCommandStopsWhatTheChildStarted(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cmd := Command(ctx, "sh", "-c", `sleep 30 & echo $!; wait`)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	grandchild := readPID(t, stdout)
	t.Cleanup(func() { _ = syscall.Kill(grandchild, syscall.SIGKILL) })

	cancel()
	_ = cmd.Wait()

	// A shell's background job ignores SIGINT: it goes on the SIGTERM a grace later.
	deadline := time.Now().Add(2 * domain.SubprocessInterruptGrace)
	for syscall.Kill(grandchild, 0) == nil {
		if time.Now().After(deadline) {
			require.FailNow(t, "the grandchild outlived the cancelled child")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// A cancellation stops what the child started, never a daemon it left behind
// on purpose: killing a credential cache or an ssh master on Ctrl-C would cost
// the user their setup.
func TestCommandLeavesADaemonTheChildStartedAlone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	script := fmt.Sprintf(`%s=1 %q -test.run='^TestDaemonHelper$' & sleep 30 & wait`, daemonHelperEnv, os.Args[0])
	cmd := Command(ctx, "sh", "-c", script)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	daemon := readPID(t, stdout)
	t.Cleanup(func() { _ = syscall.Kill(daemon, syscall.SIGKILL) })

	cancel()
	_ = cmd.Wait()
	KillCancelled()
	time.Sleep(200 * time.Millisecond)

	assert.NoError(t, syscall.Kill(daemon, 0), "the daemon the child left behind was killed with it")
}
