package worktree

import (
	"context"

	"github.com/LucasPcq/wtm/internal/infra/proc"
)

// Shield runs a step that must not stop half-way out of reach of a
// cancellation; release ends it. See proc.Shield.
func Shield(ctx context.Context) (context.Context, func()) {
	return proc.Shield(ctx)
}
