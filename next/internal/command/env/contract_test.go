package env_test

import (
	"context"
	"errors"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/storetest"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
)

func TestRunKeepsTheOperationContract(t *testing.T) {
	operationtest.Contract(t, func(t *testing.T, ops operation.Service) operationtest.Subject {
		s := acmeSeed()
		w := newWorld(s)
		w.usecase.Operations = ops
		return operationtest.Subject{
			Run: func(ctx context.Context, opts domain.RunOptions) (operationtest.Ran, error) {
				outcome, err := w.usecase.Run(ctx, naming(billing), opts)
				return operationtest.Ran{Op: outcome.Op, Fingerprint: outcome.Fingerprint}, err
			},
			Stale: func(t *testing.T) {
				// a file edited since the preview
				s.Files["/repo/.trees/"+billing+"/"+webEnv] = "PORT=3011\nAPI_URL=http://localhost:4020\n"
				w.usecase.EnvFiles = storetest.NewFake(storetest.FakeParams{Meta: s.Meta, Files: s.Files})
			},
			Fail: func(*testing.T) {
				w.usecase.EnvFiles = storetest.NewFake(storetest.FakeParams{Meta: s.Meta, Files: s.Files, Fail: map[string]error{"Edit": errors.New("disk full")}})
			},
		}
	})
}
