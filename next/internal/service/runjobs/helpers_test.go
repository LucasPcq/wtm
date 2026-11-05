package runjobs_test

import (
	"context"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"
)

// stops reads a stop's facts over d, no directory gone, every path standing for itself.
func stops(d *daemon) runjobs.Stops {
	return runjobs.Stops{
		Daemon:  d,
		Gone:    func(context.Context, worktree.Worktree) (bool, error) { return false, nil },
		Located: func(_ context.Context, name string, _ domain.Caller) (string, error) { return name, nil },
		Locate: func(context.Context, []worktree.Worktree, []string, domain.Caller) (map[string]string, error) {
			return map[string]string{}, nil
		},
	}
}

// starts starts jobs over d, every port answering, no wait for a crash.
func starts(d *daemon) runjobs.Service {
	return runjobs.Service{
		Meta:      noMeta{},
		EnvFiles:  noEnv{},
		Daemon:    d,
		Listening: func(context.Context, []int, time.Duration) map[int]bool { return map[int]bool{} },
		StateDir:  mainPath + "/.git/wtm",
		CommonDir: mainPath + "/.git",
		Gone:      func(context.Context, worktree.Worktree) (bool, error) { return false, nil },
		Located:   func(_ context.Context, name string, _ domain.Caller) (string, error) { return name, nil },
		Number:    func(context.Context, string) (int, error) { return 1, nil },
		Owing:     func() []env.OwedDrop { return nil },
	}
}
