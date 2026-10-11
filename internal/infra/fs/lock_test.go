package fs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestWithFileLockSerializesConcurrentRuns(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "test.lock")

	var mu sync.Mutex
	inside, peak := 0, 0
	var wg sync.WaitGroup

	for range 16 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = WithFileLock(t.Context(), WithFileLockParams{Path: lock, Do: func() error {
				mu.Lock()
				inside++
				if inside > peak {
					peak = inside
				}
				mu.Unlock()

				// Long enough that an unprotected section would overlap.
				buf := make([]byte, 4096)
				for i := range buf {
					buf[i] = byte(i)
				}

				mu.Lock()
				inside--
				mu.Unlock()
				return nil
			}})
		}()
	}
	wg.Wait()

	if peak > 1 {
		t.Errorf("%d runs were inside the lock at once, want 1", peak)
	}
}

func TestWithFileLockPropagatesError(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "test.lock")
	want := os.ErrInvalid

	if _, got := WithFileLock(t.Context(), WithFileLockParams{Path: lock, Do: func() error { return want }}); got != want {
		t.Errorf("error = %v, want %v", got, want)
	}
}

// A lock that cannot be taken must not cancel the work it was guarding.
func TestWithFileLockRunsWhenItCannotLock(t *testing.T) {
	unwritable := filepath.Join(t.TempDir(), "nope")
	if err := os.WriteFile(unwritable, nil, 0o644); err != nil {
		t.Fatalf("setup: %v", err)
	}

	ran := false
	lock, err := WithFileLock(t.Context(), WithFileLockParams{
		Path: filepath.Join(unwritable, "test.lock"),
		Do:   func() error { ran = true; return nil },
	})
	if err != nil {
		t.Fatalf("WithFileLock: %v", err)
	}
	if !ran {
		t.Error("the work was skipped because the lock could not be taken")
	}
	if lock.Held {
		t.Error("Held = true for a lock that could not be taken")
	}
}

func TestWithFileLockWithoutWork(t *testing.T) {
	if _, err := WithFileLock(t.Context(), WithFileLockParams{Path: filepath.Join(t.TempDir(), "test.lock")}); err != nil {
		t.Errorf("WithFileLock with no work: %v", err)
	}
}

func TestWithFileLockStopsWaitingWhenCancelled(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "test.lock")
	held := make(chan struct{})
	release := make(chan struct{})
	go func() {
		_, _ = WithFileLock(t.Context(), WithFileLockParams{Path: lock, Do: func() error {
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
	_, err := WithFileLock(ctx, WithFileLockParams{Path: lock, Do: func() error { ran = true; return nil }})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want %v", err, context.DeadlineExceeded)
	}
	if ran {
		t.Error("the work ran without the lock it was waiting for")
	}
}
