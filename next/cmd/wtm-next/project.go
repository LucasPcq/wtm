package main

import (
	"cmp"
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/domain"
)

const (
	stateDirEnv  = "WTM_STATE_DIR" // moves the state directory out of the repository's common dir
	stateDirName = "wtm"
)

// openCreate returns the create command's use case wired on the repository of dir.
func openCreate(ctx context.Context, dir string) (create.UseCase, error) {
	params, err := locate(ctx, dir)
	if err != nil {
		return create.UseCase{}, err
	}

	return wire(params).Create, nil
}

// openExtract returns the extract command's use case wired on the repository of dir.
func openExtract(ctx context.Context, dir string) (extract.UseCase, error) {
	params, err := locate(ctx, dir)
	if err != nil {
		return extract.UseCase{}, err
	}

	return wire(params).Extract, nil
}

// locate returns where the repository of dir, any of its worktrees or a folder in one, keeps its main worktree, its common dir and wtm's state.
// It refuses a directory outside any repository with [domain.CodeNotARepository].
func locate(ctx context.Context, dir string) (wireParams, error) {
	root, err := mainWorktree(ctx, dir)
	if err != nil {
		return wireParams{}, notARepository(dir, err)
	}

	commonDir, err := git.Repo{Dir: root}.CommonDir(ctx)
	if err != nil {
		return wireParams{}, notARepository(root, err)
	}

	// shortcut: WTM_STATE_DIR is process-wide (one state dir for every repository a process opens); move it into the execution context when a process serves several repositories with it set.
	return wireParams{MainPath: root, StateDir: cmp.Or(os.Getenv(stateDirEnv), filepath.Join(commonDir, stateDirName)), CommonDir: commonDir}, nil
}

// notARepository turns git's refusal to run outside a repository into the precondition failure that names dir; any other err is returned as is.
func notARepository(dir string, err error) error {
	if !errors.Is(err, git.ErrNotRepository) {
		return err
	}

	return &domain.Error{Kind: domain.KindPrecondition, Code: domain.CodeNotARepository, Message: dir + " is not a git repository", Cause: err}
}

// mainWorktree returns the path of the main worktree of dir's repository, its symbolic links resolved.
func mainWorktree(ctx context.Context, dir string) (string, error) {
	main, err := git.Repo{Dir: dir}.MainWorktree(ctx)
	if err != nil {
		return "", err
	}

	return git.RealPath(main.Path)
}
