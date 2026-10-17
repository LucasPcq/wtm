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

var (
	ErrUnlockable = errors.New("lock unavailable")
	ErrBusy       = errors.New("lock held")
)

func WithLock(ctx context.Context, path string, do func() error) (bool, error) {
	file, err := acquire(ctx, path)
	if errors.Is(err, ErrUnlockable) {
		return false, do()
	}
	if err != nil {
		return false, err
	}
	defer file.Close()
	defer Flock(file, syscall.LOCK_UN)
	return true, do()
}

func acquire(ctx context.Context, path string) (*os.File, error) {
	for {
		file, err := TryLock(TryLockParams{Path: path})
		if !errors.Is(err, ErrBusy) {
			return file, err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(lockRetryEvery):
		}
	}
}

type TryLockParams struct {
	Path   string
	Shared bool
}

func TryLock(params TryLockParams) (*os.File, error) {
	file, err := openLock(params.Path)
	if err != nil {
		return nil, ErrUnlockable
	}
	how := syscall.LOCK_EX
	if params.Shared {
		how = syscall.LOCK_SH
	}
	err = Flock(file, how|syscall.LOCK_NB)
	if err == nil {
		return file, nil
	}
	file.Close()
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return nil, ErrBusy
	}
	return nil, ErrUnlockable
}

func openLock(path string) (*os.File, error) {
	if path == "" {
		return nil, fmt.Errorf("lock path required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(path, os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0o644)
}

func Flock(file *os.File, how int) error {
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
