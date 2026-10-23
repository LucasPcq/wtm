package registry

import (
	"context"
	"testing"

	"github.com/LucasPcq/wtm/next/internal/adapter/registrytest"
	"github.com/LucasPcq/wtm/next/internal/domain"
)

// port is what both the registry and its fake implement.
type port interface {
	Add(ctx context.Context, repo domain.Repo) (bool, error)
	Prune(ctx context.Context, keep func(domain.Repo) bool) (kept, dropped []domain.Repo, err error)
}

type implementation struct {
	name string
	port port
}

// implementations returns the registry in a new directory and the fake, both empty.
func implementations(t *testing.T) []implementation {
	t.Helper()
	return []implementation{
		{name: "file", port: registryIn(t)},
		{name: "fake", port: registrytest.NewFake(registrytest.FakeParams{})},
	}
}

var (
	stays  = domain.Repo{Root: "/code/stays", CommonDir: "/code/stays/.git"}
	leaves = domain.Repo{Root: "/code/leaves", CommonDir: "/code/leaves/.git"}
)
