package runstop_test

import (
	"context"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
)

func TestRunKeepsTheOperationContract(t *testing.T) {
	operationtest.Contract(t, func(t *testing.T, ops operation.Service) operationtest.Subject {
		w := newWorld(t)
		w.usecase.Operations = ops
		w.up("api", featA, "dev")
		return operationtest.Subject{
			Run: func(ctx context.Context, opts domain.RunOptions) (operationtest.Ran, error) {
				outcome, err := w.usecase.Run(ctx, w.stopping("api", featA), opts)
				return operationtest.Ran{Op: outcome.Op, Fingerprint: outcome.Fingerprint}, err
			},
			// api crashed since: the plan clears it rather than stops it
			Stale: func(*testing.T) {
				w.daemon.listed[0].Up, w.daemon.listed[0].State = false, run.Crashed
			},
			Fail: func(*testing.T) {
				w.daemon.refuse = map[string]error{key(w.at(featA), "api"): errStack}
			},
		}
	})
}
