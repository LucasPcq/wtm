package fs

import (
	"cmp"
	"errors"
	"os"
	"slices"
	"syscall"
)

// LockRequest is one lock of a set taken together: shared lets other shared
// holders in, exclusive lets no one.
type LockRequest struct {
	Path   string
	Shared bool
}

type TryLocksParams struct {
	Requests []LockRequest
}

// Locks is a set of locks held together until Release. Held is false where
// flock is unavailable: nothing is held, and the caller runs unprotected.
type Locks struct {
	Held   bool
	files  []*os.File
	remove string
}

// BusyError names the lock of a set that another holder kept it from taking.
type BusyError struct {
	Path string
}

func (e *BusyError) Error() string { return "lock held: " + e.Path }

// TryLocks takes every lock of the set or none, without waiting: the paths in
// sorted order, each tried once, everything taken released on the first
// refusal. A set that never waits cannot deadlock with another taken in the
// opposite order.
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

// Release lets go of every lock of the set, in the reverse of the order they
// were taken; a held file is removed before its lock goes, so nobody reads it
// as held once it is not.
func (l Locks) Release() {
	if l.remove != "" {
		_ = os.Remove(l.remove)
	}
	for _, file := range slices.Backward(l.files) {
		_ = flock(file, syscall.LOCK_UN)
		_ = file.Close()
	}
}

// merged sorts the set by path and keeps one request per path, exclusive when
// any of them is.
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
