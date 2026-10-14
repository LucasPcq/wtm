package dispatch_test

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/testutil/dispatchtest"
)

// A read of stdin keeps the helper waiting where a blocked channel alone would
// be taken for a deadlock, and the process killed.
func TestHelperProcess(t *testing.T) {
	stateDir := os.Getenv(envStateDir)
	if stateDir == "" {
		t.Skip("run by TestLocksAreTakenWholeAcrossProcesses")
	}
	r := &repo{}
	entered, release := r.held()
	go func() {
		<-entered
		fmt.Println(readyLine)
		_, _ = io.Copy(io.Discard, os.Stdin)
		release()
	}()
	ran := run(t.Context(), runParams{
		Bench:   dispatchtest.New(t, dispatchtest.BenchParams{StateDir: stateDir, OpPrefix: "child-"}),
		Repo:    r,
		Request: request{Branches: strings.Split(os.Getenv(envBranches), ",")},
	})
	require.NoError(t, ran.Err)
}

func TestLocksAreTakenWholeAcrossProcesses(t *testing.T) {
	b := bench(t)
	child := exec.Command(os.Args[0], "-test.run=^TestHelperProcess$")
	child.Env = append(os.Environ(), envStateDir+"="+b.StateDir, envBranches+"=feat/b")
	stdin, err := child.StdinPipe()
	require.NoError(t, err)
	stdout, err := child.StdoutPipe()
	require.NoError(t, err)
	require.NoError(t, child.Start())
	t.Cleanup(func() { _ = stdin.Close(); _ = child.Process.Kill(); _ = child.Wait() })
	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	require.Equal(t, readyLine+"\n", line)

	refused := run(t.Context(), runParams{Bench: b, Repo: &repo{}, Request: request{Branches: []string{"feat/a", "feat/b"}}})
	failure := problem(t, refused.Err)
	assert.Equal(t, kernel.CodeLockHeld, failure.Base().Code)
	assert.Equal(t, strconv.Itoa(child.Process.Pid), failure.Base().Params[kernel.ParamPID])

	alone := run(t.Context(), runParams{Bench: b, Repo: &repo{}, Request: request{Branches: []string{"feat/a"}}})
	assert.NoError(t, alone.Err, "the refused run kept feat/a")

	require.NoError(t, child.Process.Kill())
	_ = child.Wait()
	after := run(t.Context(), runParams{Bench: b, Repo: &repo{}, Request: request{Branches: []string{"feat/b"}}})
	assert.NoError(t, after.Err, "a dead process still held feat/b")
}
