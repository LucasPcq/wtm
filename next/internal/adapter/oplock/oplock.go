// Package oplock takes the operation locks wtm takes, on the same files: never waiting, all of a set or none, the holder recorded for whoever is refused.
package oplock

import (
	"cmp"
	"errors"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"syscall"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/operation"
)

const (
	locksDir      = "locks"
	worktreesDir  = "wt"
	operationsDir = "ops"
	repoLock      = "repo"
	lockExt       = ".lock"
	holderExt     = ".json"
	unknown       = "unknown"
)

var errUnlockable = errors.New("lock unavailable")

type Locks struct {
	StateDir string
}

type Held struct {
	locks     Locks
	operation operation.Operation
	holder    holder
	files     []*os.File
	protected bool
}

func (l Locks) Take(op operation.Operation) (*Held, error) {
	taken := &Held{locks: l, operation: op}
	if l.StateDir == "" {
		return taken, nil
	}
	recorded, err := hold(l.holderPath(op.ID), op)
	if err != nil {
		return taken, nil
	}
	taken.holder = recorded
	files, protected, err := l.try(tried{Locks: op.Locks, Asked: op})
	if err != nil {
		taken.Release()
		return nil, err
	}
	taken.files, taken.protected = files, protected && recorded.held
	return taken, nil
}

func (h *Held) Protected() bool {
	return h.protected
}

func (h *Held) Grow(op operation.Operation) (bool, error) {
	missing := slices.DeleteFunc(slices.Clone(op.Locks), func(lock operation.Lock) bool {
		return slices.ContainsFunc(h.operation.Locks, func(held operation.Lock) bool { return held.Key() == lock.Key() })
	})
	if len(missing) == 0 || !h.protected {
		return false, nil
	}
	files, protected, err := h.locks.try(tried{Locks: missing, Asked: op})
	if err != nil {
		return false, err
	}
	h.files, h.protected = append(h.files, files...), protected
	h.operation = op
	h.holder = h.holder.rehold(op)
	return true, nil
}

func (h *Held) Release() {
	for _, file := range slices.Backward(h.files) {
		_ = statefile.Flock(file, syscall.LOCK_UN)
		_ = file.Close()
	}
	h.files = nil
	h.holder.release()
}

type tried struct {
	Locks []operation.Lock
	Asked operation.Operation
}

func (l Locks) try(t tried) ([]*os.File, bool, error) {
	var files []*os.File
	for _, lock := range merged(t.Locks) {
		file, err := tryLock(tryParams{Path: l.lockPath(lock), Shared: lock.Mode == operation.Shared})
		if errors.Is(err, errUnlockable) {
			closeAll(files)
			return nil, false, nil
		}
		if err != nil {
			closeAll(files)
			return nil, false, l.held(busy{Lock: lock, Asked: t.Asked})
		}
		files = append(files, file)
	}
	return files, true, nil
}

func merged(locks []operation.Lock) []operation.Lock {
	byKey := map[string]operation.Lock{}
	for _, lock := range locks {
		if seen, found := byKey[lock.Key()]; found && seen.Mode == operation.Exclusive {
			continue
		}
		byKey[lock.Key()] = lock
	}
	out := slices.Collect(maps.Values(byKey))
	slices.SortFunc(out, func(a, b operation.Lock) int { return cmp.Compare(a.Key(), b.Key()) })
	return out
}

type tryParams struct {
	Path   string
	Shared bool
}

var errBusy = errors.New("lock held")

func tryLock(params tryParams) (*os.File, error) {
	file, err := statefile.OpenLock(params.Path)
	if err != nil {
		return nil, errUnlockable
	}
	mode := syscall.LOCK_EX
	if params.Shared {
		mode = syscall.LOCK_SH
	}
	err = statefile.Flock(file, mode|syscall.LOCK_NB)
	if err == nil {
		return file, nil
	}
	file.Close()
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return nil, errBusy
	}
	return nil, errUnlockable
}

func closeAll(files []*os.File) {
	for _, file := range files {
		_ = statefile.Flock(file, syscall.LOCK_UN)
		_ = file.Close()
	}
}

type busy struct {
	Lock  operation.Lock
	Asked operation.Operation
}

func (l Locks) held(b busy) error {
	key := b.Lock.Key()
	params := wtm.Params{"key": key, "op": unknown, "command": unknown, "pid": unknown}
	for _, holder := range running(filepath.Join(l.StateDir, locksDir, operationsDir)) {
		if holder.ID == b.Asked.ID || !slices.ContainsFunc(holder.Locks, func(lock operation.Lock) bool { return lock.Key() == key }) {
			continue
		}
		params = wtm.Params{
			"key":        key,
			"op":         holder.ID,
			"command":    holder.Command,
			"pid":        strconv.Itoa(holder.PID),
			"surface":    holder.Surface,
			"started_at": holder.StartedAt.Format(time.RFC3339),
		}
		break
	}
	return &wtm.Error{
		Kind:    wtm.KindConflict,
		Code:    "lock.held",
		Message: key + " is held by " + params["command"] + " (pid " + params["pid"] + ")",
		Params:  params,
	}
}

func (l Locks) holderPath(id string) string {
	return filepath.Join(l.StateDir, locksDir, operationsDir, id+holderExt)
}

func (l Locks) lockPath(lock operation.Lock) string {
	if lock.Scope == operation.ScopeWorktree {
		return filepath.Join(l.StateDir, locksDir, worktreesDir, branch.StateSegment(lock.Name)+lockExt)
	}
	return filepath.Join(l.StateDir, locksDir, repoLock+lockExt)
}
