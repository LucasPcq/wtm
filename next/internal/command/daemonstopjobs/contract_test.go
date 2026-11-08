package daemonstopjobs_test

import (
	"context"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstopjobs"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
)

func TestRunKeepsTheOperationContract(t *testing.T) {
	operationtest.Contract(t, func(t *testing.T, ops operation.Service) operationtest.Subject {
		d := running(web, docs)
		return operationtest.Subject{
			Run: func(ctx context.Context, opts domain.RunOptions) (operationtest.Ran, error) {
				outcome, err := usecase(d, ops).Run(ctx, daemonstopjobs.Request{}, opts)
				return operationtest.Ran{Op: outcome.Op, Fingerprint: outcome.Fingerprint}, err
			},
			Stale: func(*testing.T) { d.status.Jobs = append(d.status.Jobs, worker) },
			Fail:  func(*testing.T) { d.refuse = map[string]error{"web /src/acme": errStack} },
		}
	})
}
