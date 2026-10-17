package main

import (
	"cmp"
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/surface/cli"
)

const (
	projectDirEnv = "WTM_PROJECT_DIR"
	stateDirEnv   = "WTM_STATE_DIR"
	stateDirName  = "wtm"
)

func open(ctx context.Context) (cli.Project, error) {
	if err := domain.CorrelationProblem(os.Getenv(correlationIDEnv)); err != nil {
		return cli.Project{}, domain.InvalidMessage("$" + correlationIDEnv + " " + err.Error() + ": invalid correlation id")
	}
	cwd, err := os.Getwd()
	if err != nil {
		return cli.Project{}, err
	}
	params, err := locate(ctx, cwd)
	if err != nil {
		return cli.Project{}, err
	}
	resolved, err := git.RealPath(cwd)
	if err != nil {
		return cli.Project{}, err
	}
	return cli.Project{Create: wire(params), Cwd: resolved}, nil
}

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

func notARepository(dir string, err error) error {
	if !errors.Is(err, git.ErrNotRepository) {
		return err
	}
	return &domain.Error{Kind: domain.KindPrecondition, Code: domain.CodeNotARepository, Message: dir + " is not a git repository", Cause: err}
}

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
