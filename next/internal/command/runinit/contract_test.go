package runinit_test

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/configtest"
	"github.com/LucasPcq/wtm/next/internal/command/runinit"
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
				outcome, err := w.usecase.Run(ctx, runinit.Request{}, opts)
				return operationtest.Ran{Op: outcome.Op, Fingerprint: outcome.Fingerprint}, err
			},
			Stale: func(t *testing.T) {
				writeFile(t, filepath.Join(w.main, "apps/docs/package.json"), `{"name": "docs", "scripts": {"dev": "astro dev"}}`)
			},
			Fail: func(*testing.T) {
				w.usecase.RunConfig = configtest.NewFake(configtest.FakeParams{Fail: map[string]error{"SaveRun": errors.New("read-only file system")}})
			},
		}
	})
}
