package fs_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
