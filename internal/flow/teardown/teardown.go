// Package teardown is the sequence clean and prune run on the worktrees they
// remove, one or several. Its order is the point: the jobs are stopped and checked gone, the
// on_clean hooks run, git removes the worktree, and only then is its data
// dropped and its hold on the shared services released — so a failure anywhere
// before the removal leaves the data where it was.
package teardown

import (
	"errors"
	"fmt"
	"os"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/ordinal"
	"github.com/LucasPcq/wtm/internal/flow/publish"
	"github.com/LucasPcq/wtm/internal/flow/run/owed"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/shell"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Target struct {
	Branch string
	Path   string
}

type StopParams struct {
	Context   flow.Context
	Presenter flow.Presenter
	Target    Target
	// Force lets a worktree whose jobs would not stop be removed anyway.
	Force bool
}

// Stop stops the worktree's own jobs and checks they are gone. A job still up
// refuses the removal: an API connected to its database is what a drop cannot
// go through, and a process left in a deleted directory is nobody's to stop.
func Stop(params StopParams) error {
	if params.Target.Path == "" {
		return nil
	}
	var stopped []string
	err := params.Presenter.Stage(flow.StageParams{
		Message: fmt.Sprintf(domain.CleanStoppingServicesFmt, params.Target.Branch),
		Work: func() error {
			var stopErr error
			stopped, stopErr = process.StopWorktreeJobs(process.WorktreeJobsParams{
				SocketPath: process.SocketPath(),
				WorkDir:    params.Target.Path,
			})
			return stopErr
		},
	})
	if err != nil && !params.Force {
		return fmt.Errorf(domain.CleanStopRefusedFmt, params.Target.Branch, err, params.Target.Branch)
	}
	if err != nil {
		params.Presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: fmt.Sprintf(domain.CleanStopForcedFmt, params.Target.Branch, err)})
		return nil
	}
	if len(stopped) > 0 {
		params.Presenter.Status(flow.Notice{Kind: flow.NoticeSuccess, Text: fmt.Sprintf(domain.CleanStoppedServicesFmt, params.Target.Branch)})
	}
	return nil
}

type HooksParams struct {
	Context   flow.Context
	Presenter flow.Presenter
	Target    Target
	Title     string
}

// Hooks runs on_clean as its own phase, so the hooks do not fight the removal's
// progress for the terminal.
func Hooks(params HooksParams) error {
	hooks := params.Context.Config.Project.Hooks.OnClean
	if len(hooks) == 0 || params.Target.Path == "" {
		return nil
	}
	ordinal.BeforeHooks(params.Context, params.Target.Branch)
	return params.Presenter.HookPhase(flow.HookPhaseParams{
		Title: params.Title,
		LogPath: rules.HooksLogPath(rules.HooksLogPathParams{
			StateDir: params.Context.StateDir,
			Phase:    domain.HookOnClean,
			Branch:   params.Target.Branch,
		}),
		Run: func(sink flow.HookSink) error {
			return worktree.RunCleanHooks(domain.CleanHooksParams{
				ProjectDir:   params.Context.ProjectDir,
				StateDir:     params.Context.StateDir,
				WorktreePath: params.Target.Path,
				Branch:       params.Target.Branch,
				Hooks:        hooks,
				Output:       sink.Output,
				OnHook:       sink.OnHook,
			})
		},
	})
}

type SalvageParams struct {
	Context   flow.Context
	Presenter flow.Presenter
	Clean     domain.CleanParams
	Path      string
	Cause     error
	// Last is the worktree as it was before the removal began, nil when it
	// could not be read: once git has dropped it there is nothing left to read,
	// and whoever settles the removal publishes it.
	Last *domain.WorktreeIdentity
}

// Salvage settles a removal git reported as failed. Nothing removed — a locked
// worktree — hands the cause back, data untouched. Removed with files left
// behind — root-owned files a container wrote — is a removal all the same: git
// has already dropped its entry, so the branch and the state go too and the
// leftover directory is named, rather than stranding a worktree git no longer
// knows beside a branch and a namespace nothing would ever reclaim.
func Salvage(params SalvageParams) error {
	if worktree.StillTracked(worktree.FindByBranchParams{ProjectDir: params.Clean.ProjectDir, Branch: params.Clean.Branch}) {
		return params.Cause
	}
	if err := worktree.FinishRemoval(params.Clean); err != nil {
		return err
	}
	PublishRemoved(params)
	params.Presenter.Status(flow.Notice{
		Kind: flow.NoticeWarning,
		Text: fmt.Sprintf(domain.CleanLeftOnDiskFmt, params.Clean.Branch, params.Path, params.Cause, params.Path),
	})
	return nil
}

type ReclaimParams struct {
	Context flow.Context
	Target  Target
	Dropper *owed.Dropper
}

// Reclaim is what follows a removal: the data the worktree held, and its job
// logs. The logs are best effort — leftover files are not worth failing a
// removal that already happened.
func Reclaim(params ReclaimParams) []domain.NamespaceOutcome {
	outcomes := params.Dropper.Drop(params.Target.Branch)
	_ = process.PurgeWorktreeLogs(rules.WorktreeLogDir(rules.WorktreeLogDirParams{
		StateDir: params.Context.StateDir,
		Branch:   params.Target.Branch,
	}))
	return outcomes
}

type ReleaseParams struct {
	Presenter flow.Presenter
	Target    Target
}

// Release lets go of the worktree's claims on the shared services, last: the
// last claim released stops a service, which then takes no namespace back.
func Release(params ReleaseParams) {
	if params.Target.Path == "" {
		return
	}
	if err := process.ReleaseWorktreeJobs(process.WorktreeJobsParams{
		SocketPath: process.SocketPath(),
		WorkDir:    params.Target.Path,
	}); err != nil {
		params.Presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: fmt.Sprintf(domain.CleanReleaseFailedFmt, params.Target.Branch, err)})
	}
}

type Removal struct {
	Target     Target
	Absent     bool
	Namespaces []domain.NamespaceOutcome
	Err        error
}

type BatchParams struct {
	Context   flow.Context
	Presenter flow.Presenter
	Targets   []Target
	// Force lets a worktree whose jobs would not stop be removed anyway.
	Force bool
	// ForceRemoval is git's own --force. prune sets it whatever Force says: a
	// squash-merged branch is never merged locally, and its safety was settled
	// when it was classified.
	ForceRemoval bool
	BaseBranch   string
	StartDown    bool
	KeepData     bool
	// StopOnFailure leaves every target after a failed one untouched, data included.
	StopOnFailure bool
	// NameHookPhases titles every hook phase after its worktree. A batch always
	// does; prune does for a single one too, since its candidates were never named
	// on the command line.
	NameHookPhases bool
	// Recover settles a removal git reported as failed; nil settles it as Salvage does.
	Recover func(SalvageParams) error
	OnStart func(flow.Progress)
	OnDone  func(Removal)
}

// Batch tears the targets down one after the other. What they hold is read
// before the first removal, while every environment still exists, and their
// claims are released after the last, together: the claim one removal lets go
// of may be what keeps a service up for the next one's drop.
func Batch(params BatchParams) []Removal {
	inside := insideTarget(params.Targets)
	dropper := owed.NewDropper(owed.DropperParams{
		Context:   params.Context,
		Presenter: params.Presenter,
		Snapshot:  owed.Read(owed.ReadParams{Context: params.Context, Branches: branchesOf(params.Targets)}),
		StartDown: params.StartDown,
		KeepData:  params.KeepData,
	})

	removals := make([]Removal, 0, len(params.Targets))
	for index, target := range params.Targets {
		if params.OnStart != nil {
			params.OnStart(flow.Progress{Branch: target.Branch, Position: index + 1, Total: len(params.Targets)})
		}
		removal := removeOne(removeOneParams{Batch: params, Target: target, Dropper: dropper})
		removals = append(removals, removal)
		if params.OnDone != nil {
			params.OnDone(removal)
		}
		if removal.Err != nil && params.StopOnFailure {
			break
		}
	}

	for _, removal := range removals {
		if removal.Err == nil {
			Release(ReleaseParams{Presenter: params.Presenter, Target: removal.Target})
		}
	}
	dropper.Close()

	if removedBranch(removedBranchParams{Removals: removals, Branch: inside}) {
		shell.RequestCd(params.Context.ProjectDir)
	}
	return removals
}

type removeOneParams struct {
	Batch   BatchParams
	Target  Target
	Dropper *owed.Dropper
}

func removeOne(params removeOneParams) Removal {
	batch, target := params.Batch, params.Target
	removal := Removal{Target: target}
	if err := Stop(StopParams{Context: batch.Context, Presenter: batch.Presenter, Target: target, Force: batch.Force}); err != nil {
		removal.Err = err
		return removal
	}
	if err := Hooks(HooksParams{
		Context:   batch.Context,
		Presenter: batch.Presenter,
		Target:    target,
		Title:     hooksTitle(hooksTitleParams{Target: target, Named: batch.NameHookPhases || len(batch.Targets) > 1}),
	}); err != nil {
		removal.Err = err
		return removal
	}

	clean := domain.CleanParams{
		ProjectDir: batch.Context.ProjectDir,
		StateDir:   batch.Context.StateDir,
		Branch:     target.Branch,
		Force:      batch.ForceRemoval,
		BaseBranch: batch.BaseBranch,
		Config:     batch.Context.Config,
		SkipHooks:  true,
	}
	salvage := SalvageParams{Context: batch.Context, Presenter: batch.Presenter, Clean: clean, Path: target.Path}
	if last, captured := publish.Capture(batch.Context, target.Branch); captured {
		salvage.Last = &last
	}
	err := batch.Presenter.Stage(flow.StageParams{
		Message: fmt.Sprintf(domain.CleanLoadingFmt, target.Branch),
		Work:    func() error { return worktree.Clean(clean) },
	})
	// A branch git refused to delete fails the run after the worktree went:
	// the worktree is gone all the same, and that is what consumers track.
	if err == nil || (!errors.Is(err, domain.ErrWorktreeRemoveFailed) && !worktree.StillTracked(worktree.FindByBranchParams{ProjectDir: clean.ProjectDir, Branch: clean.Branch})) {
		PublishRemoved(salvage)
	}
	if errors.Is(err, domain.ErrWorktreeRemoveFailed) {
		salvage.Cause = err
		err = recoverer(batch)(salvage)
	}
	// Already gone is a removal that happened earlier, which keeps a re-run idempotent.
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		removal.Absent, err = true, nil
	}
	if err != nil {
		removal.Err = err
		return removal
	}
	removal.Namespaces = Reclaim(ReclaimParams{Context: batch.Context, Target: target, Dropper: params.Dropper})
	return removal
}

// PublishRemoved reports a removal that went through, whichever of the three
// removals settled it.
func PublishRemoved(params SalvageParams) {
	if params.Last == nil {
		return
	}
	publish.Removed(params.Context, *params.Last)
}

func recoverer(batch BatchParams) func(SalvageParams) error {
	if batch.Recover != nil {
		return batch.Recover
	}
	return Salvage
}

type hooksTitleParams struct {
	Target Target
	Named  bool
}

func hooksTitle(params hooksTitleParams) string {
	if params.Named {
		return fmt.Sprintf(domain.HooksTitleOnCleanFmt, params.Target.Branch)
	}
	return domain.HooksTitleOnClean
}

func branchesOf(targets []Target) []string {
	branches := make([]string, 0, len(targets))
	for _, target := range targets {
		branches = append(branches, target.Branch)
	}
	return branches
}

// insideTarget is decided before any removal, while the paths still resolve
// their symlinks.
func insideTarget(targets []Target) string {
	cwd, err := os.Getwd()
	if err != nil || cwd == "" {
		return ""
	}
	resolved := flow.ResolveSymlinks(cwd)
	for _, target := range targets {
		if target.Path != "" && rules.IsPathWithin(flow.ResolveSymlinks(target.Path), resolved) {
			return target.Branch
		}
	}
	return ""
}

type removedBranchParams struct {
	Removals []Removal
	Branch   string
}

func removedBranch(params removedBranchParams) bool {
	for _, removal := range params.Removals {
		if params.Branch != "" && removal.Err == nil && removal.Target.Branch == params.Branch {
			return true
		}
	}
	return false
}
