package exec_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
)

func TestRunKeepsTheOperationContract(t *testing.T) {
	operationtest.Contract(t, func(t *testing.T, ops operation.Service) operationtest.Subject {
		w := newWorld(t)
		w.usecase.Operations = ops
		command := "test ! -e " + filepath.Join(w.stateDir, "fail")
		return operationtest.Subject{
			Run: func(ctx context.Context, opts domain.RunOptions) (operationtest.Ran, error) {
				outcome, err := w.usecase.Run(ctx, running(command, featA), opts)
				return operationtest.Ran{Op: outcome.Op, Fingerprint: outcome.Fingerprint}, err
			},
			Stale: func(t *testing.T) {
				w.record(t, featA, worktree.Meta{Ordinal: 3})
			},
			Fail: func(t *testing.T) {
				writeFile(t, filepath.Join(w.stateDir, "fail"), "")
			},
		}
	})
}
