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

// held is what a run holds: its holder file and its locks, or nothing at all
// when flock is unavailable (Held false).
type held struct {
	holder fs.Locks
	locks  fs.Locks
}

func (h held) protected() bool { return h.holder.Held && h.locks.Held }

func (h held) release() {
	h.locks.Release()
	h.holder.Release()
}

// lock writes the operation's holder file, then takes its locks, all or none
// and without waiting: a key another operation holds is a 409 naming it.
func (d *Dispatcher) lock(operation kernel.Operation) (held, error) {
	if d.params.StateDir == "" {
		return held{}, nil
	}
	data, err := json.Marshal(operation)
	if err != nil {
		return held{}, kernel.Internal(kernel.Problem{Code: kernel.CodeInternal, Cause: err})
	}
	holder, err := fs.HoldFile(fs.HoldFileParams{Path: d.holderPath(operation.ID), Data: data})
	if err != nil {
		// An unwritable state dir does not stop the command, as a missing flock
		// does not: it runs unprotected and the run warns locks.unavailable.
		return held{}, nil
	}
	requests := make([]fs.LockRequest, 0, len(operation.Locks))
	for _, key := range operation.Locks {
		requests = append(requests, fs.LockRequest{Path: d.lockPath(key), Shared: key.Mode == kernel.LockShared})
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
	Asked kernel.Operation
}

func (d *Dispatcher) lockHeld(params lockHeldParams) error {
	key := params.Path
	if index := slices.IndexFunc(params.Asked.Locks, d.locksAt(params.Path)); index >= 0 {
		key = keyName(params.Asked.Locks[index])
	}
	problem := kernel.Problem{Code: CodeLockHeld, Params: kernel.Params{
		ParamKey: key, ParamOp: unknownHolder, ParamCommand: unknownHolder, ParamPID: unknownHolder,
	}}
	operations, _ := d.operations()
	for _, operation := range operations {
		if operation.ID == params.Asked.ID || !slices.ContainsFunc(operation.Locks, d.locksAt(params.Path)) {
			continue
		}
		problem.Params = kernel.Params{
			ParamKey:       key,
			ParamOp:        string(operation.ID),
			ParamCommand:   operation.Command,
			ParamPID:       strconv.Itoa(operation.PID),
			ParamSurface:   string(operation.Surface),
			ParamStartedAt: operation.StartedAt.Format(time.RFC3339),
		}
		break
	}
	return kernel.Conflict(problem)
}

func (d *Dispatcher) locksAt(path string) func(kernel.LockKey) bool {
	return func(key kernel.LockKey) bool { return d.lockPath(key) == path }
}

// keyName is how a lock reads to a user: repo, or wt:<branch>.
func keyName(key kernel.LockKey) string {
	if key.Scope == kernel.LockWorktree {
		return string(key.Scope) + keySeparator + key.Name
	}
	return string(key.Scope)
}

// operations reads the holder files still held, oldest first: a file nobody
// holds any more belongs to a run that ended or died.
func (d *Dispatcher) operations() ([]kernel.Operation, error) {
	if d.params.StateDir == "" {
		return nil, nil
	}
	files, err := fs.HeldFiles(filepath.Join(d.locksDir(), operationsDir))
	if err != nil {
		return nil, err
	}
	operations := make([]kernel.Operation, 0, len(files))
	for _, data := range files {
		var operation kernel.Operation
		if err := json.Unmarshal(data, &operation); err != nil {
			continue
		}
		operations = append(operations, operation)
	}
	slices.SortFunc(operations, func(a, b kernel.Operation) int { return a.StartedAt.Compare(b.StartedAt) })
	return operations, nil
}

func (d *Dispatcher) locksDir() string {
	return filepath.Join(d.params.StateDir, locksDirName)
}

func (d *Dispatcher) holderPath(id kernel.OpID) string {
	return filepath.Join(d.locksDir(), operationsDir, string(id)+holderFileExt)
}

func (d *Dispatcher) lockPath(key kernel.LockKey) string {
	if key.Scope == kernel.LockWorktree {
		return filepath.Join(d.locksDir(), worktreeLocksDir, rules.EncodeBranchSegment(key.Name)+lockFileExt)
	}
	return filepath.Join(d.locksDir(), repoLockName+lockFileExt)
}

// sortedKeys is the command's locks in the order they are recorded.
func sortedKeys(keys []kernel.LockKey) []kernel.LockKey {
	sorted := slices.Clone(keys)
	slices.SortFunc(sorted, func(a, b kernel.LockKey) int {
		return cmp.Or(cmp.Compare(a.Scope, b.Scope), cmp.Compare(a.Name, b.Name))
	})
	return sorted
}

// Operations are the operations in progress, read from the holder files still
// held: the port query/operations is built on.
func (d *Dispatcher) Operations(context.Context) ([]kernel.Operation, error) {
	return d.operations()
}
