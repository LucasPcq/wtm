package cli

import (
	"bufio"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"testing"

	"github.com/stretchr/testify/require"
)

// Legacy died of SIGPIPE, exit 141, before its EPIPE handler ran: wtm events | head -n1 failed.
func TestAWriteToAStdoutWhoseReaderLeftFailsRatherThanKillTheProcess(t *testing.T) {
	if os.Getenv(envBrokenPipeHelper) != "" {
		FailOnBrokenPipe()
		for {
			if _, err := os.Stdout.Write([]byte("line\n")); err != nil {
				if errors.Is(err, syscall.EPIPE) {
					os.Exit(0)
				}
				os.Exit(3)
			}
		}
	}

	helper := exec.Command(os.Args[0], "-test.run=^TestAWriteToAStdoutWhoseReaderLeftFailsRatherThanKillTheProcess$")
	helper.Env = append(os.Environ(), envBrokenPipeHelper+"=1")
	stdout, err := helper.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, helper.Start())
	require.NoError(t, stdout.Close())

	require.NoError(t, helper.Wait())
}

// Closing the terminal sends SIGHUP: unhandled, it killed wtm and left every child, each in its own group, running.
func TestAHangupCancelsTheRunAsAnInterruptDoes(t *testing.T) {
	if os.Getenv(envHangupHelper) != "" {
		ctx, stop := interruptible(Params{KillCancelled: func() {}, AwaitShields: func() {}})
		defer stop()
		_, _ = os.Stdout.Write([]byte("ready\n"))
		<-ctx.Done()
		os.Exit(0)
	}

	helper := exec.Command(os.Args[0], "-test.run=^TestAHangupCancelsTheRunAsAnInterruptDoes$")
	helper.Env = append(os.Environ(), envHangupHelper+"=1")
	stdout, err := helper.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, helper.Start())
	_, err = bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)

	require.NoError(t, helper.Process.Signal(syscall.SIGHUP))

	require.NoError(t, helper.Wait())
}
