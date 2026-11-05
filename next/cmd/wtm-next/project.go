package main

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

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
	"github.com/LucasPcq/wtm/next/internal/command/rundown"
	"github.com/LucasPcq/wtm/next/internal/command/runlogs"
	"github.com/LucasPcq/wtm/next/internal/command/runstart"
	"github.com/LucasPcq/wtm/next/internal/command/runstop"
	"github.com/LucasPcq/wtm/next/internal/command/runup"
	"github.com/LucasPcq/wtm/next/internal/command/status"
	synccommand "github.com/LucasPcq/wtm/next/internal/command/sync"
	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"
	"github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/runview"
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
	openSync        = opener(func(a app) synccommand.UseCase { return a.Sync })
	openExec        = opener(func(a app) exec.UseCase { return a.Exec })
	openReparent    = opener(func(a app) reparent.UseCase { return a.Reparent })
	openList        = opener(func(a app) list.UseCase { return a.List })
	openResolve     = opener(func(a app) resolve.UseCase { return a.Resolve })
	openStatus      = opener(func(a app) status.UseCase { return a.Status })
	openTree        = opener(func(a app) tree.UseCase { return a.Tree })
	openRunUp       = opener(func(a app) runup.UseCase { return a.RunUp })
	openRunLogs     = opener(func(a app) runlogs.UseCase { return a.RunLogs })
	openRunDown     = opener(func(a app) rundown.UseCase { return a.RunDown })
	openRunStart    = opener(func(a app) runstart.UseCase { return a.RunStart })
	openRunStop     = opener(func(a app) runstop.UseCase { return a.RunStop })
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

// stopJob is the live view's s: run stop of the job in its worktree, unattended, saying when there was nothing to stop, failing with why it did not stop.
func stopJob(ctx context.Context, act runview.Act) (runview.Acted, error) {
	usecase, err := openRunStop(ctx, act.Dir)
	if err != nil {
		return runview.Acted{}, err
	}
	outcome, err := usecase.Run(ctx, runstop.Request{Worktrees: []string{act.Worktree}, Job: act.Job}.From(domain.Caller{Cwd: act.Dir}), domain.RunOptions{})
	if err := failed(outcome.Result, err); err != nil || outcome.Result == nil {
		return runview.Acted{}, err
	}
	// Not up there, or a shared service its worktree does not hold: the stop left everything as it was.
	unchanged := !slices.ContainsFunc(outcome.Result.Items, runjobs.StoppedOrReleased)
	return runview.Acted{Unchanged: unchanged}, nil
}

// restartJob is the live view's r: run start --restart of the job in its worktree, unattended, leaving the other worktrees' jobs running; a restart cut short once it stopped the job says so; it fails with why it did not start again.
func restartJob(ctx context.Context, act runview.Act) (runview.Acted, error) {
	usecase, err := openRunStart(ctx, act.Dir)
	if err != nil {
		return runview.Acted{}, err
	}
	outcome, err := usecase.Run(ctx, runstart.Request{Worktree: act.Worktree, Job: act.Job, Restart: true, Parallel: true}.From(domain.Caller{Cwd: act.Dir}), domain.RunOptions{})
	if outcome.Result != nil && slices.ContainsFunc(outcome.Result.FollowUps, func(f domain.FollowUp) bool { return f.Code == runstart.CodeStart }) {
		return runview.Acted{CutShort: true}, nil
	}
	return runview.Acted{}, failed(outcome.Result, err)
}

// failed is why an action of the live view did not do what it asked: context.Canceled when it was cut short before it did anything; a request refused, its problems said; another refusal, its words and the command line that does it anyway (a start forced, its daemon restarted); else the failure of its item.
func failed[D any](result *domain.Result[D], err error) error {
	if err == nil && result != nil {
		if failure := result.Failure(); failure != nil {
			err = failure
		}
	}
	refused, ok := errors.AsType[*domain.Error](err)
	switch {
	case !ok:
		return err
	case refused.Kind == domain.KindCancelled:
		return context.Canceled
	}

	var said []string
	if refused.Kind != domain.KindInvalid {
		said = append(said, refused.Message)
	}
	for _, field := range refused.Fields {
		said = append(said, field.Message(field.Path))
	}
	if refused.FollowUp != nil {
		said = append(said, strings.Join(commandLine(*refused.FollowUp), " ")+" "+refused.FollowUp.Message)
	}
	if len(said) == 0 {
		return refused
	}
	return errors.New(strings.Join(said, "; "))
}

// showJob spells, for the live view, the read that shows how act's job stands: status of its worktree.
func showJob(act runview.Act) string {
	return cli.Program + " " + status.Name + " " + act.Worktree
}

// startAgain spells, for the live view, the command line that starts act's job again in its worktree.
func startAgain(act runview.Act) string {
	return strings.Join(commandLine(domain.FollowUp{Command: runstart.Name, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{act.Worktree}}, {Path: runstart.PathJob, Values: []string{act.Job}}}}), " ")
}
