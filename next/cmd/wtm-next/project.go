package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	bus "github.com/LucasPcq/wtm/next/internal/adapter/events"
	"github.com/LucasPcq/wtm/next/internal/adapter/git"
	reposjson "github.com/LucasPcq/wtm/next/internal/adapter/registry"
	"github.com/LucasPcq/wtm/next/internal/adapter/store"
	"github.com/LucasPcq/wtm/next/internal/command/checkout"
	"github.com/LucasPcq/wtm/next/internal/command/clean"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/command/exec"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/command/fastforward"
	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/command/prune"
	"github.com/LucasPcq/wtm/next/internal/command/reparent"
	"github.com/LucasPcq/wtm/next/internal/command/resolve"
	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain"
)

const (
	stateDirEnv  = "WTM_STATE_DIR" // moves the state directory out of the repository's common dir
	stateDirName = "wtm"
)

// The openers of the use cases, each wired on the repository of the directory it is given.
var (
	openCreate      = opener(func(a app) create.UseCase { return a.Create })
	openExtract     = opener(func(a app) extract.UseCase { return a.Extract })
	openCheckout    = opener(func(a app) checkout.UseCase { return a.Checkout })
	openClean       = opener(func(a app) clean.UseCase { return a.Clean })
	openPrune       = opener(func(a app) prune.UseCase { return a.Prune })
	openFastForward = opener(func(a app) fastforward.UseCase { return a.FastForward })
	openExec        = opener(func(a app) exec.UseCase { return a.Exec })
	openReparent    = opener(func(a app) reparent.UseCase { return a.Reparent })
	openList        = opener(func(a app) list.UseCase { return a.List })
	openResolve     = opener(func(a app) resolve.UseCase { return a.Resolve })
	openStatus      = opener(func(a app) status.UseCase { return a.Status })
	openTree        = opener(func(a app) tree.UseCase { return a.Tree })
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

// openEvents opens the repository holding dir for the event stream, without registering it: the stream registers the repository it watches once its config is read, never one a global stream joins; it refuses a dir that is not a directory with an error wrapping fs.ErrNotExist.
func openEvents(ctx context.Context, dir string) (events.Source, error) {
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return events.Source{}, fmt.Errorf("%s: %w", dir, fs.ErrNotExist)
	}
	params, err := locate(ctx, dir)
	if err != nil {
		return events.Source{}, err
	}

	return events.Source{Repo: params.repo(), Repository: params.facts(), Meta: store.Meta{StateDir: params.StateDir}, Git: git.Repo{Dir: params.MainPath}}, nil
}

// register lists the repository params locates among those wtm knows, for a stream that follows all of them, once initialized; it never fails nor holds a command for long.
func register(ctx context.Context, params wireParams) {
	knownRepos(reposjson.Path(), bus.SocketPath()).Register(ctx, params.repo())
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

	return wireParams{MainPath: root, StateDir: stateDir(commonDir), CommonDir: commonDir}, nil
}

// stateDir is where the repository of commonDir keeps wtm's state: in its common dir, unless $WTM_STATE_DIR moves it.
func stateDir(commonDir string) string {
	return cmp.Or(movedState(), filepath.Join(commonDir, stateDirName))
}

// movedState is where $WTM_STATE_DIR moves wtm's state, "" when it does not: a sandbox or a test, for which the registry is left alone. It is the one reader of $WTM_STATE_DIR.
// shortcut: WTM_STATE_DIR is process-wide (one state dir for every repository a process opens); move it into the execution context when a process serves several repositories with it set.
func movedState() string {
	return os.Getenv(stateDirEnv)
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
