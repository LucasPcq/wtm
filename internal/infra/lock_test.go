package infra

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
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
			_ = WithFileLock(WithFileLockParams{Path: lock, Do: func() error {
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

	if got := WithFileLock(WithFileLockParams{Path: lock, Do: func() error { return want }}); got != want {
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
	if err := WithFileLock(WithFileLockParams{
		Path: filepath.Join(unwritable, "test.lock"),
		Do:   func() error { ran = true; return nil },
	}); err != nil {
		t.Fatalf("WithFileLock: %v", err)
	}
	if !ran {
		t.Error("the work was skipped because the lock could not be taken")
	}
}

func TestWithFileLockWithoutWork(t *testing.T) {
	if err := WithFileLock(WithFileLockParams{Path: filepath.Join(t.TempDir(), "test.lock")}); err != nil {
		t.Errorf("WithFileLock with no work: %v", err)
	}
}
