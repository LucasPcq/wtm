package dispatch

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"time"

	"github.com/LucasPcq/wtm/internal/domain/operation"
	"github.com/LucasPcq/wtm/internal/infra/fs"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules"
)

const (
	locksDirName     = "locks"
	worktreeLocksDir = "wt"
	operationsDir    = "ops"
	repoLockName     = "repo"
	lockFileExt      = ".lock"
	holderFileExt    = ".json"
	unknownHolder    = "unknown"
	keySeparator     = ":"
)

var errUnnamedLock = errors.New("a worktree lock names no worktree")

// held is the holder file and the locks an operation took, which grow once
// the defaults of its request name more.
type held struct {
	dispatcher *Dispatcher
	operation  operation.Operation
	keys       []kernel.LockKey
	holder     fs.Locks
	locks      []fs.Locks
}

type lockParams struct {
	Operation operation.Operation
	Keys      []kernel.LockKey
}

func (h *held) protected() bool {
	return h.holder.Held && !slices.ContainsFunc(h.locks, func(locks fs.Locks) bool { return !locks.Held })
}

func (h *held) release() {
	for _, locks := range slices.Backward(h.locks) {
		locks.Release()
	}
	h.holder.Release()
}

func (d *Dispatcher) lock(params lockParams) (*held, error) {
	operation := params.Operation
	taken := &held{dispatcher: d, operation: operation, keys: params.Keys}
	if err := named(params.Keys); err != nil {
		return nil, err
	}
	if d.params.StateDir == "" {
		return taken, nil
	}
	data, err := json.Marshal(operation)
	if err != nil {
		return nil, kernel.Internal(kernel.Problem{Code: kernel.CodeInternal, Cause: err})
	}
	holder, err := fs.HoldFile(fs.HoldFileParams{Path: d.holderPath(operation.ID), Data: data})
	if err != nil {
		// An unwritable state dir is treated as a missing flock: the run goes on
		// unprotected and warns.
		return taken, nil
	}
	taken.holder = holder
	locks, err := d.tryLocks(lockKeysParams{Keys: operation.Locks, Asked: operation})
	if err != nil {
		taken.release()
		return nil, err
	}
	taken.locks = []fs.Locks{locks}
	return taken, nil
}

// grow takes the keys the operation does not hold yet, all or none and without
// waiting, and says whether it took any: the facts read before may be stale.
func (h *held) grow(keys []kernel.LockKey) (bool, error) {
	if err := named(keys); err != nil {
		return false, err
	}
	missing := slices.DeleteFunc(slices.Clone(keys), func(key kernel.LockKey) bool {
		return slices.ContainsFunc(h.keys, sameLock(key))
	})
	if len(missing) == 0 || !h.protected() {
		return false, nil
	}
	all := sortedKeys(append(slices.Clone(h.keys), missing...))
	asked := h.operation
	asked.Locks = recorded(all)
	locks, err := h.dispatcher.tryLocks(lockKeysParams{Keys: recorded(missing), Asked: asked})
	if err != nil {
		return false, err
	}
	h.locks = append(h.locks, locks)
	h.keys, h.operation = all, asked
	h.rewrite()
	return true, nil
}

// rewrite lists every lock taken in the holder file; a holder that cannot be
// rewritten keeps the keys it listed, and only a 409 names its holder less well.
func (h *held) rewrite() {
	data, err := json.Marshal(h.operation)
	if err != nil {
		return
	}
	if holder, err := fs.Rehold(fs.ReholdParams{Held: h.holder, Data: data}); err == nil {
		h.holder = holder
	}
}

type lockKeysParams struct {
	Keys  []operation.Lock
	Asked operation.Operation
}

func (d *Dispatcher) tryLocks(params lockKeysParams) (fs.Locks, error) {
	requests := make([]fs.LockRequest, 0, len(params.Keys))
	for _, key := range params.Keys {
		requests = append(requests, fs.LockRequest{Path: d.lockPath(key), Shared: key.Mode == string(kernel.LockShared)})
	}
	locks, err := fs.TryLocks(fs.TryLocksParams{Requests: requests})
	var busy *fs.BusyError
	if errors.As(err, &busy) {
		return fs.Locks{}, d.lockHeld(lockHeldParams{Path: busy.Path, Asked: params.Asked})
	}
	if err != nil {
		return fs.Locks{}, kernel.Internal(kernel.Problem{Code: kernel.CodeInternal, Cause: err})
	}
	return locks, nil
}

// named refuses a worktree key with no name: it would lock no worktree.
func named(keys []kernel.LockKey) error {
	if !slices.ContainsFunc(keys, func(key kernel.LockKey) bool { return key.Scope == kernel.LockWorktree && key.Name == "" }) {
		return nil
	}
	return kernel.Internal(kernel.Problem{Code: kernel.CodeInternal, Cause: errUnnamedLock})
}

func sameLock(key kernel.LockKey) func(kernel.LockKey) bool {
	return func(other kernel.LockKey) bool { return other.Scope == key.Scope && other.Name == key.Name }
}

type lockHeldParams struct {
	Path  string
	Asked operation.Operation
}

func (d *Dispatcher) lockHeld(params lockHeldParams) error {
	key := params.Path
	if index := slices.IndexFunc(params.Asked.Locks, d.locksAt(params.Path)); index >= 0 {
		key = keyName(params.Asked.Locks[index])
	}
	problem := kernel.Problem{Code: kernel.CodeLockHeld, Params: kernel.Params{
		kernel.ParamKey: key, kernel.ParamOp: unknownHolder, kernel.ParamCommand: unknownHolder, kernel.ParamPID: unknownHolder,
	}}
	running, _ := d.inProgress()
	for _, operation := range running {
		if operation.ID == params.Asked.ID || !slices.ContainsFunc(operation.Locks, d.locksAt(params.Path)) {
			continue
		}
		problem.Params = kernel.Params{
			kernel.ParamKey:       key,
			kernel.ParamOp:        operation.ID,
			kernel.ParamCommand:   operation.Command,
			kernel.ParamPID:       strconv.Itoa(operation.PID),
			kernel.ParamSurface:   operation.Surface,
			kernel.ParamStartedAt: operation.StartedAt.Format(time.RFC3339),
		}
		break
	}
	return kernel.Conflict(problem)
}

func (d *Dispatcher) locksAt(path string) func(operation.Lock) bool {
	return func(key operation.Lock) bool { return d.lockPath(key) == path }
}

func keyName(key operation.Lock) string {
	if key.Scope == string(kernel.LockWorktree) {
		return key.Scope + keySeparator + key.Name
	}
	return key.Scope
}

func (d *Dispatcher) inProgress() ([]operation.Operation, error) {
	if d.params.StateDir == "" {
		return nil, nil
	}
	files, err := fs.HeldFiles(filepath.Join(d.locksDir(), operationsDir))
	if err != nil {
		return nil, err
	}
	found := make([]operation.Operation, 0, len(files))
	for _, data := range files {
		var operation operation.Operation
		if err := json.Unmarshal(data, &operation); err != nil {
			continue
		}
		found = append(found, operation)
	}
	slices.SortFunc(found, func(a, b operation.Operation) int { return a.StartedAt.Compare(b.StartedAt) })
	return found, nil
}

func (d *Dispatcher) locksDir() string {
	return filepath.Join(d.params.StateDir, locksDirName)
}

func (d *Dispatcher) holderPath(id string) string {
	return filepath.Join(d.locksDir(), operationsDir, id+holderFileExt)
}

func (d *Dispatcher) lockPath(key operation.Lock) string {
	if key.Scope == string(kernel.LockWorktree) {
		return filepath.Join(d.locksDir(), worktreeLocksDir, rules.EncodeBranchSegment(key.Name)+lockFileExt)
	}
	return filepath.Join(d.locksDir(), repoLockName+lockFileExt)
}

func sortedKeys(keys []kernel.LockKey) []kernel.LockKey {
	sorted := slices.Clone(keys)
	slices.SortFunc(sorted, func(a, b kernel.LockKey) int {
		return cmp.Or(cmp.Compare(a.Scope, b.Scope), cmp.Compare(a.Name, b.Name))
	})
	return sorted
}

func (d *Dispatcher) Operations(context.Context) ([]operation.Operation, error) {
	return d.inProgress()
}
