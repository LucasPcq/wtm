package fs

import (
	"cmp"
	"errors"
	"os"
	"slices"
	"syscall"
)

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
