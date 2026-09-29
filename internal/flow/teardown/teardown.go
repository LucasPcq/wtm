// Package teardown is the sequence clean and prune run on each worktree they
// remove. Its order is the point: the jobs are stopped and checked gone, the
// on_clean hooks run, git removes the worktree, and only then is its data
// dropped and its hold on the shared services released — so a failure anywhere
// before the removal leaves the data where it was.
package teardown

import (
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/owed"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
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
	Presenter flow.Presenter
	Clean     domain.CleanParams
	Path      string
	Cause     error
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
