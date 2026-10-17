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
	if errors.Is(err, git.ErrNotRepository) {
		return appParams{}, &wtm.Error{Kind: wtm.KindPrecondition, Code: "git.not_a_repository", Message: cwd + " is not a git repository", Cause: err}
	}
	if err != nil {
		return appParams{}, err
	}
	commonDir, err := git.Repo{Dir: root}.GitCommonDir(ctx)
	if err != nil {
		return appParams{}, err
	}
	return appParams{MainPath: root, StateDir: cmp.Or(os.Getenv(stateDirEnv), filepath.Join(commonDir, stateDirName)), CommonDir: commonDir}, nil
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
