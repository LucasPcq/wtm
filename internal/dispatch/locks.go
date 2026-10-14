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

type held struct {
	holder fs.Locks
	locks  fs.Locks
}

func (h held) protected() bool { return h.holder.Held && h.locks.Held }

func (h held) release() {
	h.locks.Release()
	h.holder.Release()
}

func (d *Dispatcher) lock(operation operation.Operation) (held, error) {
	if d.params.StateDir == "" {
		return held{}, nil
	}
	data, err := json.Marshal(operation)
	if err != nil {
		return held{}, kernel.Internal(kernel.Problem{Code: kernel.CodeInternal, Cause: err})
	}
	holder, err := fs.HoldFile(fs.HoldFileParams{Path: d.holderPath(operation.ID), Data: data})
	if err != nil {
		// An unwritable state dir is treated as a missing flock: the run goes on
		// unprotected and warns.
		return held{}, nil
	}
	requests := make([]fs.LockRequest, 0, len(operation.Locks))
	for _, key := range operation.Locks {
		requests = append(requests, fs.LockRequest{Path: d.lockPath(key), Shared: key.Mode == string(kernel.LockShared)})
	}
	locks, err := fs.TryLocks(fs.TryLocksParams{Requests: requests})
	var busy *fs.BusyError
	if errors.As(err, &busy) {
		holder.Release()
		return held{}, d.lockHeld(lockHeldParams{Path: busy.Path, Asked: operation})
	}
	if err != nil {
		holder.Release()
		return held{}, kernel.Internal(kernel.Problem{Code: kernel.CodeInternal, Cause: err})
	}
	return held{holder: holder, locks: locks}, nil
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
