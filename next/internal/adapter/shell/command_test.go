package shell

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
)

func TestACancelledChildIsInterruptedFirst(t *testing.T) {
	run := runCancelled(t, `trap 'echo int; exit 3' INT; while :; do sleep 0.05; done`)

	require.Contains(t, run.Output, "int")
	assert.Less(t, run.Elapsed, interruptGrace)
}

func TestAChildThatIgnoresTheInterruptIsTerminated(t *testing.T) {
	run := runCancelled(t, `trap '' INT; trap 'echo term; exit 4' TERM; while :; do sleep 0.05; done`)

	assert.Contains(t, run.Output, "term")
}

func TestACancelledChildTakesWhatItStartedWithIt(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cmd := Command(ctx, "sh", "-c", `sleep 30 & echo $!; wait`)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	grandchild := readPID(t, stdout)
	t.Cleanup(func() { _ = syscall.Kill(grandchild, syscall.SIGKILL) })

	cancel()
	_ = cmd.Wait()

	assert.Eventually(t, func() bool { return syscall.Kill(grandchild, 0) != nil }, 2*interruptGrace, 20*time.Millisecond)
}

func TestEveryChildRunsInAGroupOfItsOwnOutOfReachOfTheTerminal(t *testing.T) {
	cmd := Command(t.Context(), "sleep", "5")
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })

	pgid, err := syscall.Getpgid(cmd.Process.Pid)

	require.NoError(t, err)
	assert.Equal(t, cmd.Process.Pid, pgid)
	assert.NotEqual(t, syscall.Getpgrp(), pgid)
}

func TestAChildWithALiveContextRunsToTheEnd(t *testing.T) {
	out, err := Command(t.Context(), "sh", "-c", "echo done").Output()

	require.NoError(t, err)
	assert.Equal(t, "done", strings.TrimSpace(string(out)))
}

func TestADaemonThatLeftTheGroupIsLeftAlone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	script := fmt.Sprintf(`%s=1 %q -test.run='^TestADaemonTheChildLeavesBehind$' & sleep 30 & wait`, envDaemonHelper, os.Args[0])
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

	assert.NoError(t, syscall.Kill(daemon, 0))
}

func TestADaemonTheChildLeavesBehind(t *testing.T) {
	if os.Getenv(envDaemonHelper) == "" {
		t.Skip("started by TestADaemonThatLeftTheGroupIsLeftAlone")
	}
	if _, err := syscall.Setsid(); err != nil {
		os.Exit(2)
	}
	fmt.Println(os.Getpid())
	time.Sleep(30 * time.Second)
	os.Exit(0)
}

func TestKillCancelledStopsAChildThatIgnoresEveryPoliteSignal(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cmd := Command(ctx, "sh", "-c", `trap '' INT TERM; echo ready; while :; do sleep 0.05; done`)
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, cmd.Start())
	waitForLine(t, stdout)
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()

	cancel()
	time.Sleep(50 * time.Millisecond)
	KillCancelled()

	select {
	case <-done:
	case <-time.After(interruptGrace / 2):
		require.FailNow(t, "the child outlived KillCancelled")
	}
}
