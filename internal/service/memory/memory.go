// Package memory keeps the answers a repository asked the wizard never to ask again.
package memory

import (
	"context"

	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/store"
)

type RememberParams struct {
	StateDir string
	Change   rules.RememberedChange
}

func Remember(ctx context.Context, params RememberParams) error {
	return store.ConfigDisk{StateDir: params.StateDir}.Remember(ctx, params.Change)
}
