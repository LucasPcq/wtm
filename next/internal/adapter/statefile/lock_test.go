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
			_, err := WithLock(t.Context(), lock, func() error {
				mu.Lock()
				inside++
				peak = max(peak, inside)
				mu.Unlock()
				time.Sleep(time.Millisecond)
				mu.Lock()
				inside--
				mu.Unlock()
				return nil
			})
			assert.NoError(t, err)
		})
	}
	wg.Wait()

	assert.Equal(t, 1, peak, "runs overlapped inside the lock")
}

func TestALockReturnsWhatTheWorkReturns(t *testing.T) {
	failed := errors.New("work failed")

	held, err := WithLock(t.Context(), filepath.Join(t.TempDir(), "test.lock"), func() error { return failed })

	assert.ErrorIs(t, err, failed)
	assert.True(t, held)
}

func TestALockThatCannotBeTakenRunsTheWorkUnprotected(t *testing.T) {
	notADir := filepath.Join(t.TempDir(), "file")
	writeFixture(t, notADir, "")
	ran := false

	held, err := WithLock(t.Context(), filepath.Join(notADir, "test.lock"), func() error { ran = true; return nil })

	require.NoError(t, err)
	assert.True(t, ran, "the work was skipped because the lock could not be taken")
	assert.False(t, held)
}

func TestALockInADirectoryThatRefusesItDegradesToUnlocked(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "readonly")
	require.NoError(t, os.Mkdir(dir, 0o555))
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	ran := false

	held, err := WithLock(t.Context(), filepath.Join(dir, "test.lock"), func() error { ran = true; return nil })

	require.NoError(t, err)
	assert.True(t, ran)
	assert.False(t, held)
}

func TestWaitingForALockEndsWithTheContext(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "test.lock")
	held, release := make(chan struct{}), make(chan struct{})
	go func() {
		_, _ = WithLock(t.Context(), lock, func() error {
			close(held)
			<-release
			return nil
		})
	}()
	<-held
	defer close(release)
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	ran := false

	_, err := WithLock(ctx, lock, func() error { ran = true; return nil })

	assert.ErrorIs(t, err, context.DeadlineExceeded)
	assert.False(t, ran, "the work ran without the lock it was waiting for")
}

func TestAFreeLockIsTakenEvenOnceTheContextIsDone(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	ran := false

	held, err := WithLock(ctx, filepath.Join(t.TempDir(), "test.lock"), func() error { ran = true; return nil })

	require.NoError(t, err)
	assert.True(t, ran)
	assert.True(t, held)
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
	target := filepath.Join(dir, "real")
	require.NoError(t, os.Mkdir(target, 0o755))
	link := filepath.Join(dir, "link")
	require.NoError(t, os.Symlink(target, link))

	assert.Equal(t, ScratchLockPath(filepath.Join(target, ".env")), ScratchLockPath(filepath.Join(link, ".env")), "absent file")
	writeFixture(t, filepath.Join(target, ".env"), "")
	assert.Equal(t, ScratchLockPath(filepath.Join(target, ".env")), ScratchLockPath(filepath.Join(link, ".env")), "present file")
}

func TestALockFileThatIsASymlinkIsNotFollowed(t *testing.T) {
	dir := t.TempDir()
	victim := filepath.Join(dir, "victim")
	require.NoError(t, os.WriteFile(victim, []byte("untouched"), 0o600))
	lockPath := filepath.Join(dir, "x.lock")
	require.NoError(t, os.Symlink(victim, lockPath))

	held, err := WithLock(t.Context(), lockPath, func() error { return nil })

	require.NoError(t, err)
	assert.False(t, held)
}
