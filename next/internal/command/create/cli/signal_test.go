package cli

import (
	"os"
	"syscall"
	"testing"
	"time"

	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"

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
