package fs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const lockRetryEvery = 10 * time.Millisecond

type Lock struct {
	Held bool
}

type WithFileLockParams struct {
	Path string
	Do   func() error
}

// WithFileLock's lock is released with the descriptor, which the kernel closes
// even if the process dies mid-run.
func WithFileLock(ctx context.Context, params WithFileLockParams) (Lock, error) {
	if params.Do == nil {
		return Lock{Held: true}, nil
	}

	file, err := acquire(ctx, params.Path)
	if errors.Is(err, errUnlockable) {
		return Lock{}, params.Do()
	}
	if err != nil {
		return Lock{}, err
	}
	defer file.Close()
	defer flock(file, syscall.LOCK_UN)

	return Lock{Held: true}, params.Do()
}

var errUnlockable = errors.New("lock unavailable")

func acquire(ctx context.Context, path string) (*os.File, error) {
	file, err := openLockFile(path)
	if err != nil {
		return nil, errUnlockable
	}
	for {
		err := flock(file, syscall.LOCK_EX|syscall.LOCK_NB)
		if err == nil {
			return file, nil
		}
		if !errors.Is(err, syscall.EWOULDBLOCK) {
			file.Close()
			return nil, errUnlockable
		}
		select {
		case <-ctx.Done():
			file.Close()
			return nil, ctx.Err()
		case <-time.After(lockRetryEvery):
		}
	}
}

func openLockFile(path string) (*os.File, error) {
	if path == "" {
		return nil, fmt.Errorf("lock path required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
}

func flock(file *os.File, how int) error {
	for {
		err := syscall.Flock(int(file.Fd()), how)
		// The Go runtime's own preemption signals interrupt a call, and an interrupted
		// try is no answer about the lock.
		if !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
}
