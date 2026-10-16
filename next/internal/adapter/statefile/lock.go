package statefile

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const (
	lockRetryEvery = 10 * time.Millisecond
	scratchLockDir = "wtm-locks"
)

var errUnlockable = errors.New("lock unavailable")

type lockParams struct {
	Path string
	Do   func() error
}

func withLock(ctx context.Context, params lockParams) (bool, error) {
	file, err := acquire(ctx, params.Path)
	if errors.Is(err, errUnlockable) {
		return false, params.Do()
	}
	if err != nil {
		return false, err
	}
	defer file.Close()
	defer flock(file, syscall.LOCK_UN)
	return true, params.Do()
}

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
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0o644)
}

func flock(file *os.File, how int) error {
	for {
		// The runtime's preemption signals interrupt the call, and an interrupted try says nothing about the lock.
		if err := syscall.Flock(int(file.Fd()), how); !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
}

func ScratchLockPath(path string) string {
	sum := sha256.Sum256([]byte(resolvedFile(path)))
	return filepath.Join(os.TempDir(), scratchLockDir, hex.EncodeToString(sum[:16])+lockSuffix)
}

func resolvedFile(path string) string {
	absolute, err := filepath.Abs(path)
	if err != nil {
		absolute = path
	}
	if resolved, err := filepath.EvalSymlinks(absolute); err == nil {
		return resolved
	}
	dir := filepath.Dir(absolute)
	if resolved, err := filepath.EvalSymlinks(dir); err == nil {
		dir = resolved
	}
	return filepath.Join(dir, filepath.Base(absolute))
}
