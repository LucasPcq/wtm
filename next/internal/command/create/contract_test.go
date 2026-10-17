package create_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
)

func TestRunKeepsTheOperationContract(t *testing.T) {
	operationtest.Contract(t, func(t *testing.T, ops operation.Service) operationtest.Subject {
		w := newWorld(t, seed{})
		w.usecase.Operations = ops
		return operationtest.Subject{
			Run: func(ctx context.Context, opts domain.RunOptions) (string, error) {
				outcome, err := w.usecase.Run(ctx, create.Request{Branches: []string{branchName}}, opts)
				return outcome.Fingerprint, err
			},
			Stale: func(t *testing.T) {
				writeFile(t, filepath.Join(w.stateDir, "config.toml"), strings.Replace(projectToml, `strategy = "main"`, `strategy = "example"`, 1))
			},
			Fail: func(t *testing.T) {
				writeFile(t, filepath.Join(w.stateDir, "config.toml"), withHook("exit 3"))
			},
		}
	})
}
