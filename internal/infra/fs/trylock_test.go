package fs_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/fs"
)

func TestASetIsTakenWholeOrNotAtAll(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.lock"), filepath.Join(dir, "b.lock")
	other := tryLocks(t, fs.LockRequest{Path: b})
	defer other.Release()

	_, err := fs.TryLocks(fs.TryLocksParams{Requests: []fs.LockRequest{{Path: a}, {Path: b}}})

	var busy *fs.BusyError
	require.ErrorAs(t, err, &busy)
	assert.Equal(t, b, busy.Path)
	alone := tryLocks(t, fs.LockRequest{Path: a})
	alone.Release()
}

func TestSharedHoldersLetEachOtherInAndKeepAnExclusiveOut(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo.lock")
	first := tryLocks(t, fs.LockRequest{Path: repo, Shared: true})
	defer first.Release()
	second := tryLocks(t, fs.LockRequest{Path: repo, Shared: true})
	defer second.Release()

	_, err := fs.TryLocks(fs.TryLocksParams{Requests: []fs.LockRequest{{Path: repo}}})

	var busy *fs.BusyError
	assert.ErrorAs(t, err, &busy)
}

func TestAPathAskedTwiceIsTakenOnceAtTheStrongerMode(t *testing.T) {
	repo := filepath.Join(t.TempDir(), "repo.lock")
	held := tryLocks(t, fs.LockRequest{Path: repo, Shared: true}, fs.LockRequest{Path: repo})
	defer held.Release()

	_, err := fs.TryLocks(fs.TryLocksParams{Requests: []fs.LockRequest{{Path: repo, Shared: true}}})

	var busy *fs.BusyError
	assert.ErrorAs(t, err, &busy, "the set took the shared mode and let another shared holder in")
}

func TestReleasedLocksCanBeTakenAgain(t *testing.T) {
	path := filepath.Join(t.TempDir(), "wt", "feat.lock")
	tryLocks(t, fs.LockRequest{Path: path}).Release()
	tryLocks(t, fs.LockRequest{Path: path}).Release()
}

func TestASetThatCannotBeLockedRunsUnprotected(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(notADir, nil, 0o644))

	locks, err := fs.TryLocks(fs.TryLocksParams{Requests: []fs.LockRequest{{Path: filepath.Join(notADir, "repo.lock")}}})

	require.NoError(t, err)
	assert.False(t, locks.Held)
	locks.Release()
}

// — Across processes: opposite orders never deadlock, never overlap —

func TestSetsTakenInOppositeOrdersNeitherDeadlockNorOverlap(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a.lock"), filepath.Join(dir, "b.lock")

	var wg sync.WaitGroup
	for _, order := range [][]string{{a, b}, {b, a}} {
		wg.Go(func() {
			cmd := exec.Command(os.Args[0], "-test.run=^TestLockHelperProcess$")
			cmd.Env = append(os.Environ(), envLockPaths+"="+strings.Join(order, string(os.PathListSeparator)), envLockInside+"="+filepath.Join(dir, "inside"))
			out, err := cmd.CombinedOutput()
			assert.NoError(t, err, string(out))
		})
	}
	wg.Wait()
}
