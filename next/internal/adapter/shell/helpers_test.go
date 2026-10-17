package shell

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// cancelledRun is what a child printed before and after its cancellation, and
// how long it took to return once cancelled.
type cancelledRun struct {
	Output  string
	Elapsed time.Duration
}

// runCancelled starts script under sh, cancels it once it had time to set its
// traps, and returns what it printed and how long it took to stop.
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

// readPID reads the first line a child writes on stdout as a process id.
func readPID(t *testing.T, stdout io.Reader) int {
	t.Helper()
	line, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
	pid, err := strconv.Atoi(strings.TrimSpace(line))
	require.NoError(t, err, "the child said %q", line)
	return pid
}

// waitForLine blocks until a child writes its first line on stdout.
func waitForLine(t *testing.T, stdout io.Reader) {
	t.Helper()
	_, err := bufio.NewReader(stdout).ReadString('\n')
	require.NoError(t, err)
}

// collected gathers the lines a hook chain emits, from any goroutine.
type collected struct {
	mu    sync.Mutex
	lines []string
}

func (c *collected) add(line string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.lines = append(c.lines, line)
}

func (c *collected) all() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return slices.Clone(c.lines)
}

// resolved returns path with its symbolic links resolved, as a child sees its
// working directory (macOS's /tmp is /private/tmp).
func resolved(t *testing.T, path string) string {
	t.Helper()
	target, err := filepath.EvalSymlinks(path)
	require.NoError(t, err)
	return target
}
