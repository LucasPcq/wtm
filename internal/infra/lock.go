package infra

import (
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

type WithFileLockParams struct {
	Path string
	Do   func() error
}

// WithFileLock runs Do while holding an exclusive lock on Path, so a
// read-then-write that must be atomic across processes stays one step. The lock
// is advisory and released with the descriptor, which the kernel closes even if
// the process dies mid-run.
//
// A lock that cannot be taken (an unwritable state dir, a filesystem without
// flock) does not cancel the work: Do runs unprotected rather than failing a
// command over a guard.
func WithFileLock(params WithFileLockParams) error {
	if params.Do == nil {
		return nil
	}

	file, err := openLockFile(params.Path)
	if err != nil {
		return params.Do()
	}
	defer file.Close()

	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return params.Do()
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)

	return params.Do()
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
