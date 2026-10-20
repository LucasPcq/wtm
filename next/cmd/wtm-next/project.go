package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/config"
	bus "github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	"github.com/LucasPcq/wtm/next/internal/adapter/registry"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

const (
	stateDirEnv  = "WTM_STATE_DIR" // moves the state directory out of the repository's common dir
	stateDirName = "wtm"

	// registerTimeout bounds the registration of a repository, so a command never waits on it.
	registerTimeout = 250 * time.Millisecond
)

// The openers of the use cases, each wired on the repository of the directory it is given.
var (
	openCreate  = opener(func(a app) create.UseCase { return a.Create })
	openExtract = opener(func(a app) extract.UseCase { return a.Extract })
	openList    = opener(func(a app) list.UseCase { return a.List })
	openStatus  = opener(func(a app) status.UseCase { return a.Status })
	openTree    = opener(func(a app) tree.UseCase { return a.Tree })
)

// opener returns the function that opens the use case useCase picks out of the app wired on the repository of a directory.
func opener[U any](useCase func(app) U) func(ctx context.Context, dir string) (U, error) {
	return func(ctx context.Context, dir string) (U, error) {
		params, err := locate(ctx, dir)
		if err != nil {
			var none U
			return none, err
		}

		register(ctx, params)
		return useCase(wire(params)), nil
	}
}

// openEvents opens the repository holding dir for the event stream, registered among those wtm knows; it refuses a dir that is not a directory with an error wrapping fs.ErrNotExist.
func openEvents(ctx context.Context, dir string) (events.Source, error) {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return events.Source{}, fmt.Errorf("%s: %w", dir, fs.ErrNotExist)
	}
	params, err := locate(ctx, dir)
	if err != nil {
		return events.Source{}, err
	}

	register(ctx, params)
	repo := git.Repo{Dir: params.MainPath}
	return events.Source{
		Repo:       domain.Repo{Root: params.MainPath, CommonDir: bus.Resolved(params.CommonDir)},
		Repository: repository.Service{Config: config.Project{StateDir: params.StateDir, GlobalPath: config.GlobalPath()}, Git: repo, MainPath: params.MainPath},
		Meta:       store.Meta{StateDir: params.StateDir},
	}, nil
}

// register lists the repository params locates among those wtm knows, for a stream that follows all of them; it never fails nor holds a command for long. A state directory moved by $WTM_STATE_DIR keeps the repository out: a sandbox or a test, not a repository of the user's.
func register(ctx context.Context, params wireParams) {
	if os.Getenv(stateDirEnv) != "" {
		return
	}

	ctx, cancel := context.WithTimeout(ctx, registerTimeout)
	defer cancel()
	known{Registry: registry.Registry{Path: registry.Path()}, Socket: bus.SocketPath()}.Register(ctx, domain.Repo{Root: params.MainPath, CommonDir: bus.Resolved(params.CommonDir)})
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
