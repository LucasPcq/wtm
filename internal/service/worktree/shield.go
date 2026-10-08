package worktree

import (
	"context"

	"github.com/LucasPcq/wtm/internal/infra"
)

// Shield runs a step that must not stop half-way out of reach of a
// cancellation; release ends it. See infra.Shield.
func Shield(ctx context.Context) (context.Context, func()) {
	return infra.Shield(ctx)
}
