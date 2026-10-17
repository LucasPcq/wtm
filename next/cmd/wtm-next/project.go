package main

import (
	"cmp"
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/surface/cli"
	"github.com/LucasPcq/wtm/next/internal/wtm"
)

const (
	projectDirEnv = "WTM_PROJECT_DIR"
	stateDirEnv   = "WTM_STATE_DIR"
	stateDirName  = "wtm"
)

func open(ctx context.Context) (cli.Project, error) {
	if err := wtm.CorrelationProblem(os.Getenv(correlationIDEnv)); err != nil {
		return cli.Project{}, wtm.InvalidMessage("$" + correlationIDEnv + " " + err.Error() + ": invalid correlation id")
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
	return cli.Project{App: newApp(params), Cwd: resolved}, nil
}

func locate(ctx context.Context, cwd string) (appParams, error) {
	root, err := projectRoot(ctx, cwd)
	if err != nil {
		return appParams{}, notARepository(cwd, err)
	}
	commonDir, err := git.Repo{Dir: root}.CommonDir(ctx)
	if err != nil {
		return appParams{}, notARepository(root, err)
	}
	return appParams{MainPath: root, StateDir: cmp.Or(os.Getenv(stateDirEnv), filepath.Join(commonDir, stateDirName)), CommonDir: commonDir}, nil
}

func notARepository(dir string, err error) error {
	if !errors.Is(err, git.ErrNotRepository) {
		return err
	}
	return &wtm.Error{Kind: wtm.KindPrecondition, Code: wtm.CodeNotARepository, Message: dir + " is not a git repository", Cause: err}
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
