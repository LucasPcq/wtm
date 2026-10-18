// Package repository reads the facts of a repository a command decides from: its config, its run.toml and its snapshot.
package repository

import (
	"context"
	"errors"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// The suffixes of the codes a command refuses a repository's config with: <command>.config_not_found when it has no wtm config, <command>.config_invalid when its config.toml does not parse or validate.
const (
	ConfigNotFound = ".config_not_found"
	ConfigInvalid  = ".config_invalid"
)

// Service reads the repository whose main checkout is at MainPath, through its ports.
type Service struct {
	Config   Config
	Git      Snapshotter
	MainPath string
}

// Observe reads the repository the command named command decides from: the
// project's config and global settings, the repository's snapshot and its
// run.toml. A run.toml that fails to load does not fail it: its error travels
// in RunErr to where run.toml is used. A repository with no wtm config is
// refused with the command's code, command + ConfigNotFound, and one whose
// config.toml is invalid with command + ConfigInvalid.
func (s Service) Observe(ctx context.Context, command string) (worktree.Repository, error) {
	config, global, err := s.Config.Load()
	if errors.Is(err, project.ErrConfigNotFound) {
		return worktree.Repository{}, &domain.Error{Kind: domain.KindPrecondition, Code: command + ConfigNotFound, Message: "no wtm config found — run `wtm init` first", Cause: err}
	}
	if invalid, ok := errors.AsType[*project.InvalidConfigError](err); ok {
		return worktree.Repository{}, &domain.Error{Kind: domain.KindPrecondition, Code: command + ConfigInvalid, Message: "invalid config.toml: " + invalid.Reason.Error(), Cause: err}
	}
	if err != nil {
		return worktree.Repository{}, err
	}

	snapshot, err := s.Git.Snapshot(ctx)
	if err != nil {
		return worktree.Repository{}, err
	}

	repo := worktree.Repository{MainPath: s.MainPath, Config: config, Global: global, Snapshot: snapshot}
	repo.Run, repo.RunErr = s.Config.LoadRun()
	return repo, nil
}
