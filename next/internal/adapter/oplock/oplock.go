// Package oplock takes the operation locks wtm takes, on the same files: never waiting, all of a set or none, the holder recorded for whoever is refused.
package oplock

import (
	"cmp"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"syscall"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"
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

type Locks struct {
	StateDir string
}

type Held struct {
	locks     Locks
	operation domain.Operation
	holder    holder
	files     []*os.File
	protected bool
}

func (l Locks) Take(op domain.Operation) (*Held, error) {
	taken := &Held{locks: l, operation: op}
	if l.StateDir == "" {
		return taken, nil
	}
	recorded, err := hold(l.holderPath(op.ID), op)
	if err != nil {
		return taken, nil
	}
	taken.holder = recorded
	files, protected, err := l.try(op.Locks, op)
	if err != nil {
		taken.Release()
		return nil, err
	}
	taken.files, taken.protected = files, protected
	return taken, nil
}

func (h *Held) Protected() bool {
	return h.protected
}

func (h *Held) Grow(locks []domain.Lock) (bool, error) {
	missing := slices.DeleteFunc(slices.Clone(locks), func(lock domain.Lock) bool {
		return slices.ContainsFunc(h.operation.Locks, func(held domain.Lock) bool { return held.Key() == lock.Key() })
	})
	if len(missing) == 0 || !h.protected {
		return false, nil
	}
	files, protected, err := h.locks.try(missing, h.operation)
	if err != nil {
		return false, err
	}
	h.files, h.protected = append(h.files, files...), protected
	h.operation.Locks = append(h.operation.Locks, missing...)
	slices.SortFunc(h.operation.Locks, func(a, b domain.Lock) int { return cmp.Compare(a.Key(), b.Key()) })
	h.holder = h.holder.rehold(h.operation)
	return true, nil
}

func (h *Held) Release() {
	closeAll(h.files)
	h.files = nil
	h.holder.release()
}

func (l Locks) try(locks []domain.Lock, asked domain.Operation) ([]*os.File, bool, error) {
	var files []*os.File
	for _, lock := range locks {
		file, err := statefile.TryLock(statefile.TryLockParams{Path: l.lockPath(lock), Shared: lock.Mode == domain.LockShared})
		if err != nil {
			closeAll(files)
		}
		if errors.Is(err, statefile.ErrBusy) {
			return nil, false, l.held(lock, asked)
		}
		if err != nil {
			return nil, false, nil
		}
		files = append(files, file)
	}
	return files, true, nil
}

func closeAll(files []*os.File) {
	for _, file := range files {
		_ = statefile.Flock(file, syscall.LOCK_UN)
		_ = file.Close()
	}
}

func (l Locks) held(lock domain.Lock, asked domain.Operation) error {
	key := lock.Key()
	params := domain.Params{"key": key, "op": unknown, "command": unknown, "pid": unknown}
	for _, holder := range running(filepath.Join(l.StateDir, locksDir, operationsDir)) {
		if holder.ID == asked.ID || !slices.ContainsFunc(holder.Locks, func(lock domain.Lock) bool { return lock.Key() == key }) {
			continue
		}
		params = domain.Params{
			"key":        key,
			"op":         holder.ID,
			"command":    holder.Command,
			"pid":        strconv.Itoa(holder.PID),
			"surface":    holder.Surface,
			"started_at": holder.StartedAt.Format(time.RFC3339),
		}
		break
	}
	return &domain.Error{
		Kind:    domain.KindConflict,
		Code:    "lock.held",
		Message: key + " is held by " + params["command"] + " (pid " + params["pid"] + ")",
		Params:  params,
	}
}

func (l Locks) holderPath(id string) string {
	return filepath.Join(l.StateDir, locksDir, operationsDir, id+holderExt)
}

func (l Locks) lockPath(lock domain.Lock) string {
	if lock.Scope == domain.ScopeWorktree {
		return filepath.Join(l.StateDir, locksDir, worktreesDir, worktree.StateSegment(lock.Name)+lockExt)
	}
	return filepath.Join(l.StateDir, locksDir, repoLock+lockExt)
}
