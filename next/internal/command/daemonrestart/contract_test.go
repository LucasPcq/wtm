package daemonrestart_test

import (
	"context"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/daemonrestart"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
)

func TestRunKeepsTheOperationContract(t *testing.T) {
	operationtest.Contract(t, func(t *testing.T, ops operation.Service) operationtest.Subject {
		d := running("0.30.9", web, stack)
		return operationtest.Subject{
			Run: func(ctx context.Context, opts domain.RunOptions) (operationtest.Ran, error) {
				outcome, err := usecase(d, ops).Run(ctx, daemonrestart.Request{}, opts)
				return operationtest.Ran{Op: outcome.Op, Fingerprint: outcome.Fingerprint}, err
			},
			Stale: func(*testing.T) {
				d.status.Jobs = append(d.status.Jobs, api)
				d.records = append(d.records, recordOf(api))
			},
			Fail: func(*testing.T) { d.refuse = map[string]error{"web " + main: errPort} },
		}
	})
}
