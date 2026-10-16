package statefile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestALockSerializesConcurrentRuns(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "test.lock")
	var mu sync.Mutex
	inside, peak := 0, 0

	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			_, err := WithLock(t.Context(), WithLockParams{Path: lock, Do: func() error {
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

func TestALockReturnsWhatTheWorkReturns(t *testing.T) {
	failed := errors.New("work failed")

	lock, err := WithLock(t.Context(), WithLockParams{Path: filepath.Join(t.TempDir(), "test.lock"), Do: func() error { return failed }})

	assert.ErrorIs(t, err, failed)
	assert.True(t, lock.Held)
}

func TestALockThatCannotBeTakenRunsTheWorkUnprotected(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "file")
	writeFixture(t, notADir, "")
	ran := false

	lock, err := WithLock(t.Context(), WithLockParams{Path: filepath.Join(notADir, "test.lock"), Do: func() error { ran = true; return nil }})

	require.NoError(t, err)
	assert.True(t, ran, "the work was skipped because the lock could not be taken")
	assert.False(t, lock.Held)
}

func TestALockInADirectoryThatRefusesItDegradesToUnlocked(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "readonly")
	require.NoError(t, os.Mkdir(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	ran := false

	lock, err := WithLock(t.Context(), WithLockParams{Path: filepath.Join(dir, "test.lock"), Do: func() error { ran = true; return nil }})

	require.NoError(t, err)
	assert.True(t, ran)
	assert.False(t, lock.Held)
}

func TestALockWithoutWorkDoesNothing(t *testing.T) {
	lock, err := WithLock(t.Context(), WithLockParams{Path: filepath.Join(t.TempDir(), "test.lock")})

	assert.NoError(t, err)
	assert.True(t, lock.Held)
}

func TestWaitingForALockEndsWithTheContext(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "test.lock")
	held, release := make(chan struct{}), make(chan struct{})
	go func() {
		_, _ = WithLock(t.Context(), WithLockParams{Path: lock, Do: func() error {
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

	_, err := WithLock(ctx, WithLockParams{Path: lock, Do: func() error { ran = true; return nil }})

	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.False(t, ran, "the work ran without the lock it was waiting for")
}

func TestAFreeLockIsTakenEvenOnceTheContextIsDone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	ran := false

	lock, err := WithLock(ctx, WithLockParams{Path: filepath.Join(t.TempDir(), "test.lock"), Do: func() error { ran = true; return nil }})

	require.NoError(t, err)
	assert.True(t, ran)
	assert.True(t, lock.Held)
}

func TestAScratchLockIsOneLockPerFileOutsideItsDirectory(t *testing.T) {
	dir := t.TempDir()
	env := filepath.Join(dir, ".env")

	lock := ScratchLockPath(env)

	assert.Equal(t, lock, ScratchLockPath(env), "two processes must meet on the same lock")
	assert.NotEqual(t, lock, ScratchLockPath(filepath.Join(dir, "compose.yml")))
	assert.True(t, strings.HasPrefix(lock, os.TempDir()))
	assert.NotEqual(t, dir, filepath.Dir(lock))
}

func TestAScratchLockIsTheSameThroughASymlinkedDirectory(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	require.NoError(t, os.Mkdir(real, 0o755))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(real, link))

	assert.Equal(t, ScratchLockPath(filepath.Join(real, ".env")), ScratchLockPath(filepath.Join(link, ".env")), "absent file")
	writeFixture(t, filepath.Join(real, ".env"), "")
	assert.Equal(t, ScratchLockPath(filepath.Join(real, ".env")), ScratchLockPath(filepath.Join(link, ".env")), "present file")
}
