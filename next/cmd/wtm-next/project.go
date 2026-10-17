package main

import (
	"cmp"
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	createcli "github.com/LucasPcq/wtm/next/internal/command/create/cli"
	extractcli "github.com/LucasPcq/wtm/next/internal/command/extract/cli"
	"github.com/LucasPcq/wtm/next/internal/domain"
)

const (
	projectDirEnv = "WTM_PROJECT_DIR" // names the repository instead of the working directory's
	stateDirEnv   = "WTM_STATE_DIR"   // moves the state directory out of the repository's common dir
	stateDirName  = "wtm"
)

// A project is the repository a command runs on, as [open] found it from the working directory.
type project struct {
	Wire wireParams
	Cwd  string // the working directory with its symbolic links resolved
}

// openCreate opens the project and returns the create command's use case wired on it.
func openCreate(ctx context.Context) (createcli.Project, error) {
	opened, err := open(ctx)
	if err != nil {
		return createcli.Project{}, err
	}

	return createcli.Project{UseCase: wire(opened.Wire).Create, Cwd: opened.Cwd}, nil
}

// openExtract opens the project and returns the extract command's use case wired on it.
func openExtract(ctx context.Context) (extractcli.Project, error) {
	opened, err := open(ctx)
	if err != nil {
		return extractcli.Project{}, err
	}

	return extractcli.Project{UseCase: wire(opened.Wire).Extract, Cwd: opened.Cwd}, nil
}

// open finds the project of the working directory.
// It refuses an invalid $WTM_CORRELATION_ID as a usage error, and a directory outside any repository with [domain.CodeNotARepository].
func open(ctx context.Context) (project, error) {
	if err := domain.CorrelationProblem(os.Getenv(correlationIDEnv)); err != nil {
		return project{}, domain.InvalidMessage("$" + correlationIDEnv + " " + err.Error() + ": invalid correlation id")
	}

	cwd, err := os.Getwd()
	if err != nil {
		return project{}, err
	}

	params, err := locate(ctx, cwd)
	if err != nil {
		return project{}, err
	}

	resolved, err := git.RealPath(cwd)
	if err != nil {
		return project{}, err
	}

	return project{Wire: params, Cwd: resolved}, nil
}

// locate returns where the repository of cwd keeps its main worktree, its common dir and wtm's state.
func locate(ctx context.Context, cwd string) (wireParams, error) {
	root, err := projectRoot(ctx, cwd)
	if err != nil {
		return wireParams{}, notARepository(cwd, err)
	}

	commonDir, err := git.Repo{Dir: root}.CommonDir(ctx)
	if err != nil {
		return wireParams{}, notARepository(root, err)
	}

	return wireParams{MainPath: root, StateDir: cmp.Or(os.Getenv(stateDirEnv), filepath.Join(commonDir, stateDirName)), CommonDir: commonDir}, nil
}

// notARepository turns git's refusal to run outside a repository into the precondition failure that names dir; any other err is returned as is.
func notARepository(dir string, err error) error {
	if !errors.Is(err, git.ErrNotRepository) {
		return err
	}

	return &domain.Error{Kind: domain.KindPrecondition, Code: domain.CodeNotARepository, Message: dir + " is not a git repository", Cause: err}
}

// projectRoot returns the path of the main worktree of cwd's repository, or the one $WTM_PROJECT_DIR names.
func projectRoot(ctx context.Context, cwd string) (string, error) {
	if named := os.Getenv(projectDirEnv); named != "" {
		return git.RealPath(named)
	}

	main, err := git.Repo{Dir: cwd}.MainWorktree(ctx)
	if err != nil {
		return "", err
	}

	return git.RealPath(main.Path)
}
