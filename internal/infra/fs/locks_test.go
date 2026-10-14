package fs_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/LucasPcq/wtm/internal/infra/fs"
)

func TestWithFileLockSerializesConcurrentRuns(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "test.lock")
	var mu sync.Mutex
	inside, peak := 0, 0

	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			_, err := fs.WithFileLock(t.Context(), fs.WithFileLockParams{Path: lock, Do: func() error {
				mu.Lock()
				inside++
				peak = max(peak, inside)
				mu.Unlock()
				time.Sleep(time.Millisecond)
				mu.Lock()
				inside--
				mu.Unlock()
				return nil
			}})
			assert.NoError(t, err)
		})
	}
	wg.Wait()

	assert.Equal(t, 1, peak, "runs overlapped inside the lock")
}

func TestWithFileLockReturnsWhatTheWorkReturns(t *testing.T) {
	failed := errors.New("work failed")
	lock, err := fs.WithFileLock(t.Context(), fs.WithFileLockParams{
		Path: filepath.Join(t.TempDir(), "test.lock"),
		Do:   func() error { return failed },
	})
	assert.ErrorIs(t, err, failed)
	assert.True(t, lock.Held)
}

func TestALockThatCannotBeTakenRunsTheWorkUnprotected(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "file")
	require.NoError(t, os.WriteFile(notADir, nil, 0o644))

	ran := false
	lock, err := fs.WithFileLock(t.Context(), fs.WithFileLockParams{
		Path: filepath.Join(notADir, "test.lock"),
		Do:   func() error { ran = true; return nil },
	})

	require.NoError(t, err)
	assert.True(t, ran, "the work was skipped because the lock could not be taken")
	assert.False(t, lock.Held)
}

func TestWithFileLockWithoutWorkDoesNothing(t *testing.T) {
	_, err := fs.WithFileLock(t.Context(), fs.WithFileLockParams{Path: filepath.Join(t.TempDir(), "test.lock")})
	assert.NoError(t, err)
}

func TestWaitingForALockEndsWithTheContext(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "test.lock")
	held, release := make(chan struct{}), make(chan struct{})
	go func() {
		_, _ = fs.WithFileLock(t.Context(), fs.WithFileLockParams{Path: lock, Do: func() error {
			close(held)
			<-release
			return nil
		}})
	}()
	<-held
	defer close(release)

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	ran := false
	_, err := fs.WithFileLock(ctx, fs.WithFileLockParams{Path: lock, Do: func() error { ran = true; return nil }})

	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.False(t, ran, "the work ran without the lock it was waiting for")
}

func TestAFreeLockIsTakenEvenOnceTheContextIsDone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	ran := false
	lock, err := fs.WithFileLock(ctx, fs.WithFileLockParams{
		Path: filepath.Join(t.TempDir(), "test.lock"),
		Do:   func() error { ran = true; return nil },
	})
	require.NoError(t, err)
	assert.True(t, ran)
	assert.True(t, lock.Held)
}

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
