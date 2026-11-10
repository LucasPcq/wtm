package daemonstopjobs

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"
)

// Daemon is wtm's daemon as stop-jobs reaches it: how it stands, never starting it, and a job stopped.
type Daemon interface {
	Status(ctx context.Context) (run.Daemon, error)
	runjobs.Stopper
}

// RepoRoot returns the main checkout of the repository the directory at path is in, as the daemon's jobs name it; it fails when path is in no repository.
type RepoRoot func(ctx context.Context, path string) (string, error)
