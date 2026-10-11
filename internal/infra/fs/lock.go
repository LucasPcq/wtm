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

// Lock says whether the work ran under its lock. A lock that cannot be taken
// (an unwritable state dir, a filesystem without flock such as NFS or some WSL
// mounts) does not cancel the work: it runs unprotected, and Held tells the
// caller so.
type Lock struct {
	Held bool
}

type WithFileLockParams struct {
	Path string
	Do   func() error
}

// WithFileLock runs Do while holding an exclusive lock on Path, so a
// read-then-write that must be atomic across processes stays one step. The lock
// is advisory and released with the descriptor, which the kernel closes even if
// the process dies mid-run. Waiting for it ends with ctx.
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

// flock retries a call a signal interrupted: the Go runtime's own preemption
// signals can, and an interrupted try is no answer about the lock.
func flock(file *os.File, how int) error {
	for {
		err := syscall.Flock(int(file.Fd()), how)
		if !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
}
