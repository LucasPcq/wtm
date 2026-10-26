package prune_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
)

func TestRunKeepsTheOperationContract(t *testing.T) {
	operationtest.Contract(t, func(t *testing.T, ops operation.Service) operationtest.Subject {
		w := newWorld(t, seed{})
		w.usecase.Operations = ops
		return operationtest.Subject{
			Run: func(ctx context.Context, opts domain.RunOptions) (operationtest.Ran, error) {
				outcome, err := w.usecase.Run(ctx, pruning(merged), opts)
				return operationtest.Ran{Op: outcome.Op, Fingerprint: outcome.Fingerprint}, err
			},
			Stale: func(t *testing.T) {
				writeFile(t, filepath.Join(w.stateDir, "config.toml"), withHook("true"))
			},
			Fail: func(t *testing.T) {
				writeFile(t, filepath.Join(w.stateDir, "config.toml"), withHook("exit 3"))
			},
		}
	})
}
