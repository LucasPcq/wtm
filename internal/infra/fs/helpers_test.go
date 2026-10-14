package fs_test

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/fs"
)

const (
	envCounterPath   = "WTM_TEST_COUNTER_PATH"
	envCounterRounds = "WTM_TEST_COUNTER_ROUNDS"
	envLockPaths     = "WTM_TEST_LOCK_PATHS"
	envLockInside    = "WTM_TEST_LOCK_INSIDE"
	envHoldPath      = "WTM_TEST_HOLD_PATH"
	lockRounds       = 200
)

func increment(t testing.TB, path string) fs.Lock {
	t.Helper()
	lock, err := fs.UpdateFile(t.Context(), fs.UpdateFileParams{
		Path: path,
		Mutate: func(current []byte) (fs.Change, error) {
			count := 0
			if current != nil {
				parsed, err := strconv.Atoi(string(current))
				if err != nil {
					return nil, err
				}
				count = parsed
			}
			return fs.Write{Data: []byte(strconv.Itoa(count + 1))}, nil
		},
	})
	require.NoError(t, err)
	return lock
}

func counter(t *testing.T, path string) int {
	t.Helper()
	count, err := strconv.Atoi(read(t, path))
	require.NoError(t, err)
	return count
}

func set(t *testing.T, path, content string, perm os.FileMode) {
	t.Helper()
	_, err := fs.UpdateFile(t.Context(), fs.UpdateFileParams{Path: path, Perm: perm, Mutate: func([]byte) (fs.Change, error) {
		return fs.Write{Data: []byte(content)}, nil
	}})
	require.NoError(t, err)
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func modeOf(t *testing.T, path string) os.FileMode {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	return info.Mode().Perm()
}

func entries(t *testing.T, dir string) []string {
	t.Helper()
	listed, err := os.ReadDir(dir)
	require.NoError(t, err)
	names := make([]string, 0, len(listed))
	for _, entry := range listed {
		names = append(names, entry.Name())
	}
	return names
}

func writeFixture(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// TestHelperProcess is the body of the processes the cross-process test
// starts: it increments the counter it is pointed at, and is skipped otherwise.
func TestHelperProcess(t *testing.T) {
	path := os.Getenv(envCounterPath)
	if path == "" {
		t.Skip("run by TestUpdateFileLosesNoUpdateAcrossProcesses")
	}
	rounds, err := strconv.Atoi(os.Getenv(envCounterRounds))
	require.NoError(t, err)
	for range rounds {
		increment(t, path)
	}
}

func tryLocks(t testing.TB, requests ...fs.LockRequest) fs.Locks {
	t.Helper()
	locks, err := fs.TryLocks(fs.TryLocksParams{Requests: requests})
	require.NoError(t, err)
	require.True(t, locks.Held)
	return locks
}

func heldFiles(t *testing.T, dir string) [][]byte {
	t.Helper()
	held, err := fs.HeldFiles(dir)
	require.NoError(t, err)
	return held
}

func waitForLine(t *testing.T, out io.Reader) {
	t.Helper()
	ready := make(chan error, 1)
	go func() {
		_, err := bufio.NewReader(out).ReadString('\n')
		ready <- err
	}()
	select {
	case err := <-ready:
		require.NoError(t, err)
	case <-time.After(10 * time.Second):
		t.Fatal("the child never said it was ready")
	}
}

// TestLockHelperProcess takes the set it is pointed at, in the order given,
// again and again, and fails if another holder of the set is inside with it.
func TestLockHelperProcess(t *testing.T) {
	paths := os.Getenv(envLockPaths)
	if paths == "" {
		t.Skip("run by TestSetsTakenInOppositeOrdersNeitherDeadlockNorOverlap")
	}
	var requests []fs.LockRequest
	for _, path := range strings.Split(paths, string(os.PathListSeparator)) {
		requests = append(requests, fs.LockRequest{Path: path})
	}
	inside := os.Getenv(envLockInside)
	for range lockRounds {
		locks, err := fs.TryLocks(fs.TryLocksParams{Requests: requests})
		var busy *fs.BusyError
		if errors.As(err, &busy) {
			continue
		}
		require.NoError(t, err)
		marker, err := os.OpenFile(inside, os.O_CREATE|os.O_EXCL, 0o644)
		require.NoError(t, err, "another process holds the set too")
		require.NoError(t, marker.Close())
		require.NoError(t, os.Remove(inside))
		locks.Release()
	}
}

// TestHoldHelperProcess holds the file it is pointed at until it is killed,
// waiting on its stdin: a process blocked on nothing but a channel would be
// taken for a deadlock and stopped by the runtime.
func TestHoldHelperProcess(t *testing.T) {
	path := os.Getenv(envHoldPath)
	if path == "" {
		t.Skip("run by TestAProcessThatDiesHoldsNothing")
	}
	_, err := fs.HoldFile(fs.HoldFileParams{Path: path, Data: []byte(`{"op":"child"}`)})
	require.NoError(t, err)
	fmt.Println("held")
	_, _ = io.Copy(io.Discard, os.Stdin)
}
