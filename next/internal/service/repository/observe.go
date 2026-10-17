// Package repository reads the facts of a repository a command decides from: its config, its run.toml and its snapshot.
package repository

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ObserveParams are the ports Observe reads through, and the path of the main checkout.
type ObserveParams struct {
	Config   Config
	Git      Snapshotter
	MainPath string
}

// Observe reads the repository a command decides from: the project's config
// and global settings, the repository's snapshot and its run.toml. A run.toml
// that fails to load does not fail it: its error travels in RunErr to where
// run.toml is used. A repository with no wtm config fails with the error of
// Config.Load, wrapping project.ErrConfigNotFound.
func Observe(ctx context.Context, params ObserveParams) (worktree.Repository, error) {
	config, global, err := params.Config.Load()
	if err != nil {
		return worktree.Repository{}, err
	}

	snapshot, err := params.Git.Snapshot(ctx)
	if err != nil {
		return worktree.Repository{}, err
	}

	repo := worktree.Repository{MainPath: params.MainPath, Config: config, Global: global, Snapshot: snapshot}
	repo.Run, repo.RunErr = params.Config.LoadRun()
	return repo, nil
}
