package daemonstop

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// Daemon is wtm's daemon as a stop reaches it: how it stands, never starting it, and its shutdown, which returns once it is gone.
type Daemon interface {
	Status(ctx context.Context) (run.Daemon, error)
	Shutdown(ctx context.Context) error
}
