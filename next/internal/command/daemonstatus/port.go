package daemonstatus

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// Daemon reads wtm's daemon as it stands, never starting it: with none answering, the stacks and claims its index holds.
type Daemon func(ctx context.Context) (run.Daemon, error)
