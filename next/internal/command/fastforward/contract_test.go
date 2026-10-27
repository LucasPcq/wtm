package fastforward_test

import (
	"context"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
)

func TestRunKeepsTheOperationContract(t *testing.T) {
	operationtest.Contract(t, func(t *testing.T, ops operation.Service) operationtest.Subject {
		w := newWorld(t)
		w.usecase.Operations = ops
		return operationtest.Subject{
			Run: func(ctx context.Context, opts domain.RunOptions) (operationtest.Ran, error) {
				outcome, err := w.usecase.Run(ctx, naming(behind), opts)
				return operationtest.Ran{Op: outcome.Op, Fingerprint: outcome.Fingerprint}, err
			},
			Stale: func(t *testing.T) {
				w.pushed(t, behind)
			},
			Fail: func(*testing.T) {
				w.usecase.Provision.Git = refusingMerge{Repo: w.repo, path: w.at(behind)}
			},
		}
	})
}
