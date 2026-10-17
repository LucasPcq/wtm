package cli

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"

	"github.com/LucasPcq/wtm/next/internal/domain/env"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const helperEnv = "WTM_NEXT_CLI_HELPER"

func TestTheFirstInterruptSaysWhatItFinishesAndExitsCancelled(t *testing.T) {
	if os.Getenv(helperEnv) != "" {
		os.Exit(uicli.Main(newWorld(seed{OnCreate: []project.Hook{{Cmd: "make setup"}}, RunHooks: armedUntilCancelled}).params([]string{"create", branchName})))
	}

	helper := startHelper(t, "TestTheFirstInterruptSaysWhatItFinishesAndExitsCancelled")
	helper.expect(t, "armed")
	require.NoError(t, syscall.Kill(helper.cmd.Process.Pid, syscall.SIGINT))

	status := helper.wait(t)
	assert.Equal(t, uicli.ExitCancelled, status.ExitStatus())
	assert.Contains(t, helper.stderr.String(), "Cancelling — finishing on_create of feat/x; Ctrl+C again to stop now")
}

func TestTheSecondInterruptStopsAtOnce(t *testing.T) {
	if os.Getenv(helperEnv) != "" {
		os.Exit(uicli.Main(newWorld(seed{OnCreate: []project.Hook{{Cmd: "make setup"}}, RunHooks: armedForever}).params([]string{"create", branchName})))
	}

	helper := startHelper(t, "TestTheSecondInterruptStopsAtOnce")
	helper.expect(t, "armed")
	require.NoError(t, syscall.Kill(helper.cmd.Process.Pid, syscall.SIGINT))
	time.Sleep(100 * time.Millisecond)
	require.NoError(t, syscall.Kill(helper.cmd.Process.Pid, syscall.SIGINT))

	status := helper.wait(t)
	assert.True(t, status.Signaled(), "the run must die by the signal, as a shell reads 130")
	assert.Equal(t, syscall.SIGINT, status.Signal())
}

func armedUntilCancelled(ctx context.Context, _ env.HookChain) error {
	fmt.Println("armed")
	<-ctx.Done()
	return ctx.Err()
}

func armedForever(context.Context, env.HookChain) error {
	fmt.Println("armed")
	select {}
}

type helper struct {
	cmd    *exec.Cmd
	lines  *bufio.Scanner
	stderr *bytes.Buffer
}

func startHelper(t *testing.T, test string) helper {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^"+test+"$")
	cmd.Env = append(os.Environ(), helperEnv+"=1")
	stdout, err := cmd.StdoutPipe()
	require.NoError(t, err)
	stderr := &bytes.Buffer{}
	cmd.Stderr = stderr
	require.NoError(t, cmd.Start())
	t.Cleanup(func() { _ = cmd.Process.Kill() })
	return helper{cmd: cmd, lines: bufio.NewScanner(stdout), stderr: stderr}
}

func (h helper) expect(t *testing.T, want string) {
	t.Helper()
	for h.lines.Scan() {
		if h.lines.Text() == want {
			return
		}
	}
	t.Fatalf("the helper never said %q", want)
}

func (h helper) wait(t *testing.T) syscall.WaitStatus {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		for h.lines.Scan() {
		}
		done <- h.cmd.Wait()
	}()
	select {
	case err := <-done:
		exitErr, ok := errors.AsType[*exec.ExitError](err)
		require.True(t, ok, "the helper ended with %v", err)
		status, ok := exitErr.Sys().(syscall.WaitStatus)
		require.True(t, ok)
		return status
	case <-time.After(10 * time.Second):
		t.Fatal("the helper did not stop")
	}
	return 0
}
