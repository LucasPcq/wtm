package dispatch

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"

	"github.com/LucasPcq/wtm/internal/domain/operation"
	"github.com/LucasPcq/wtm/internal/infra/fs"
	"github.com/LucasPcq/wtm/internal/kernel"
)

type operationParams[Req any] struct {
	Run  RunParams[Req]
	Keys []kernel.LockKey
}

func (h *Handle[Req, F, P, D]) operation(params operationParams[Req]) operation.Operation {
	return operation.Operation{
		ID:            string(h.dispatcher.params.NewOpID()),
		PID:           os.Getpid(),
		Command:       h.command.Name,
		Surface:       string(params.Run.Options.Surface),
		CorrelationID: params.Run.Options.CorrelationID,
		StartedAt:     h.dispatcher.params.Now().UTC(),
		Locks:         recorded(params.Keys),
	}
}

func recorded(keys []kernel.LockKey) []operation.Lock {
	if keys == nil {
		return nil
	}
	locks := make([]operation.Lock, 0, len(keys))
	for _, key := range keys {
		locks = append(locks, operation.Lock{Scope: string(key.Scope), Name: key.Name, Mode: string(key.Mode)})
	}
	return locks
}

func (d *Dispatcher) Operations(context.Context) ([]operation.Operation, error) {
	return d.inProgress()
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
