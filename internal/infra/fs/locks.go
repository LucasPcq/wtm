package fs

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
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

type LockRequest struct {
	Path   string
	Shared bool
}

type TryLocksParams struct {
	Requests []LockRequest
}

// Locks.Held is false where flock is unavailable: nothing is held.
type Locks struct {
	Held   bool
	files  []*os.File
	remove string
}

type BusyError struct {
	Path string
}

func (e *BusyError) Error() string { return "lock held: " + e.Path }

// TryLocks takes every lock of the set or none, and never waits: a set that
// never waits cannot deadlock with another taken in the opposite order.
func TryLocks(params TryLocksParams) (Locks, error) {
	taken := Locks{Held: true}
	for _, request := range merged(params.Requests) {
		file, err := tryLock(request)
		if errors.Is(err, errUnlockable) {
			taken.Release()
			return Locks{}, nil
		}
		if err != nil {
			taken.Release()
			return Locks{}, err
		}
		taken.files = append(taken.files, file)
	}
	return taken, nil
}

// Release removes a held file before its lock goes, so nobody reads it as held
// once it is not.
func (l Locks) Release() {
	if l.remove != "" {
		_ = os.Remove(l.remove)
	}
	for _, file := range slices.Backward(l.files) {
		_ = flock(file, syscall.LOCK_UN)
		_ = file.Close()
	}
}

func merged(requests []LockRequest) []LockRequest {
	byPath := map[string]bool{}
	for _, request := range requests {
		shared, seen := byPath[request.Path]
		byPath[request.Path] = request.Shared && (!seen || shared)
	}
	out := make([]LockRequest, 0, len(byPath))
	for path, shared := range byPath {
		out = append(out, LockRequest{Path: path, Shared: shared})
	}
	slices.SortFunc(out, func(a, b LockRequest) int { return cmp.Compare(a.Path, b.Path) })
	return out
}

func tryLock(request LockRequest) (*os.File, error) {
	file, err := openLockFile(request.Path)
	if err != nil {
		return nil, errUnlockable
	}
	mode := syscall.LOCK_EX
	if request.Shared {
		mode = syscall.LOCK_SH
	}
	err = flock(file, mode|syscall.LOCK_NB)
	if err == nil {
		return file, nil
	}
	file.Close()
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return nil, &BusyError{Path: request.Path}
	}
	return nil, errUnlockable
}
