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

// The errors of [TryLock]: ErrUnlockable when the lock file cannot be opened
// or locked at all (a read-only or lock-less filesystem), ErrBusy when another
// process holds the lock.
var (
	ErrUnlockable = errors.New("lock unavailable")
	ErrBusy       = errors.New("lock held")
)

// WithLock runs do while holding the exclusive flock of the file at path,
// waiting for it until ctx is done, and returns do's error. It reports whether
// the lock was held: where the lock cannot be taken at all, do runs unlocked
// rather than not at all. A cancelled wait returns ctx's error without running
// do.
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

// acquire tries the exclusive lock at path until it is free or ctx is done.
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

// TryLockParams names the lock file [TryLock] takes, and whether to take it
// shared rather than exclusive.
type TryLockParams struct {
	Path   string
	Shared bool
}

// TryLock takes the flock of the file at params.Path without waiting,
// creating the file and its directory if needed, and returns the open file
// that holds it; the caller unlocks it with [Flock] and closes it. It returns
// ErrBusy when another process holds the lock and ErrUnlockable when it cannot
// be taken at all. A lock file that is a symbolic link is not followed.
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

// Flock applies the flock operation how (syscall.LOCK_EX, LOCK_SH, LOCK_UN,
// with LOCK_NB) to file and returns its error.
func Flock(file *os.File, how int) error {
	for {
		// The runtime's preemption signals interrupt the call, and an interrupted try says nothing about the lock.
		if err := syscall.Flock(int(file.Fd()), how); !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
}

// ScratchLockPath returns the path of a lock file for the file at path, under
// the system's temporary directory rather than beside it, so locking a file
// leaves nothing in its directory. Two paths to the same file, through a
// symbolic link included, get the same lock.
func ScratchLockPath(path string) string {
	sum := sha256.Sum256([]byte(resolvedFile(path)))
	return filepath.Join(os.TempDir(), scratchLockDir, hex.EncodeToString(sum[:16])+lockSuffix)
}

// resolvedFile returns path absolute with its symbolic links resolved; a file
// that does not exist yet has only its directory resolved.
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
