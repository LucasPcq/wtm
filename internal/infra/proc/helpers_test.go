package proc

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type cancelledRun struct {
	Output  string
	Elapsed time.Duration
}

func runCancelled(t *testing.T, script string) cancelledRun {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	var out bytes.Buffer
	cmd := Command(ctx, "sh", "-c", script)
	cmd.Stdout = &out
	require.NoError(t, cmd.Start())
	time.Sleep(200 * time.Millisecond)
	begin := time.Now()
	cancel()
	_ = cmd.Wait()
	return cancelledRun{Output: out.String(), Elapsed: time.Since(begin)}
}

func readPID(t *testing.T, stdout io.Reader) int {
	t.Helper()
	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(line))
	require.NoError(t, err, "the child said %q", line)
	return pid
}

// TestDaemonHelper is the daemon a git child may leave behind: a credential
// cache, an fsmonitor, an ssh ControlPersist master. It leaves its parent's
// session the way they all do.
func TestDaemonHelper(t *testing.T) {
	if os.Getenv(daemonHelperEnv) == "" {
		t.Skip("helper process")
	}
	if _, err := syscall.Setsid(); err != nil {
		os.Exit(2)
	}
	fmt.Println(os.Getpid())
	time.Sleep(30 * time.Second)
	os.Exit(0)
}
