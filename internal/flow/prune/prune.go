// Package prune runs the `wtm prune` flow.
package prune

import (
	"errors"
	"fmt"
	"os"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/service/runjobs"
	"github.com/LucasPcq/wtm/internal/service/shell"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Request struct {
	// Merged, Closed and Gone are the reason filters. At least one is always set:
	// the surface widens to all three when none was asked for.
	Merged  bool
	Closed  bool
	Gone    bool
	NoFetch bool
	// Force is the safety axis: it lifts the dirty/unpushed/open-PR refusals.
	Force            bool
	ReparentChildren bool
	// DryRun previews the plan and mutates nothing. It is a business input, not
	// an output mode: it changes what the run does, not how it reads.
	DryRun     bool
	BaseBranch string
}

type Outcome struct {
	Result domain.PruneResult
	// Plan is what a dry run computed, for a surface that renders a preview
	// differently from a result.
	Plan domain.PrunePlan
	// Empty reports that nothing matched, or that nothing survived the selection.
	Empty   bool
	Aborted bool
}

type Presenter interface {
	flow.Presenter
	Pruned(Outcome) error
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

// Operation declares how a surface must schedule a prune: it removes several
// worktrees and asks first, so it holds the surface for its whole run. It names
// no target — holding everything, it needs no per-worktree lock.
func Operation() flow.Operation {
	return flow.Operation{Kind: domain.OpKindPrune, Mode: flow.ModeBlocking}
}

func Run(params Params) (Outcome, error) {
	f := &pruneFlow{
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
	}
	return f.run()
}

type pruneFlow struct {
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter

	plan domain.PrunePlan
}

func (f *pruneFlow) run() (Outcome, error) {
	if err := f.scan(); err != nil {
		return Outcome{}, err
	}
	if len(f.plan.Selected) == 0 {
		return f.conclude(Outcome{Empty: true})
	}
	if f.request.DryRun {
		return f.conclude(Outcome{
			Plan: f.plan,
			Result: domain.PruneResult{
				Pruned:     f.plan.Selected,
				Reparented: f.plan.Reparents,
				Skipped:    f.plan.Skipped,
				DryRun:     true,
			},
		})
	}

	answers, err := f.prompter.Ask(f.session())
	if errors.Is(err, domain.ErrUserAborted) {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}
	if err != nil {
		return Outcome{}, err
	}

	f.plan = rules.FinalizePrunePlan(rules.FinalizePrunePlanParams{
		Plan:       f.plan,
		Chosen:     answers.Values(KeySelection),
		BaseBranch: f.request.BaseBranch,
		Force:      f.request.Force || answers.Value(KeyConfirm) == confirmForce,
	})
	if len(f.plan.Selected) == 0 {
		return f.conclude(Outcome{Empty: true})
	}

	return f.remove(answers.Value(KeyReparent) == reparentYes)
}

// scan classifies with force whenever someone can still deselect, so unsafe
// worktrees surface as candidates left unchecked rather than as silent skips.
func (f *pruneFlow) scan() error {
	needPRs := f.request.Merged || f.request.Closed || !f.request.Force

	var connection domain.GHConnection
	err := f.presenter.Stage(flow.StageParams{
		Message: f.scanMessage(needPRs),
		Work: func() error {
			var prs []domain.PRInfo
			if needPRs {
				prs, connection = github.ListPRsWithConnection(f.ctx.ProjectDir)
			}
			var planErr error
			f.plan, planErr = worktree.PlanPrune(f.params(), prs)
			return planErr
		},
	})
	if err != nil {
		return err
	}

	// prune reads "done" from GitHub, so say so when the CLI is unavailable:
	// merged/closed detection is then inert and only --gone applies.
	if needPRs {
		if title, lines, show := rules.PruneGHNotice(connection); show {
			f.presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: title, Lines: lines})
		}
	}
	return nil
}

func (f *pruneFlow) scanMessage(needPRs bool) string {
	if needPRs || (f.request.Gone && !f.request.NoFetch) {
		return domain.PruneFetchAndScanning
	}
	return domain.PruneScanning
}

func (f *pruneFlow) remove(reparentChildren bool) (Outcome, error) {
	orphaned := f.plan.Reparents
	if reparentChildren {
		orphaned = nil
	} else {
		f.plan.Reparents = nil
	}

	// Decided before the removal, while the paths still resolve their symlinks.
	insidePruned := f.insidePruned()

	for _, candidate := range f.plan.Selected {
		f.stopServices(candidate.Branch)
	}
	if err := f.runHooks(); err != nil {
		return Outcome{}, err
	}

	var result domain.PruneResult
	err := f.presenter.Stage(flow.StageParams{
		Message: domain.PruneRemoving,
		Work: func() error {
			var pruneErr error
			result, pruneErr = worktree.Prune(f.params(), f.plan)
			return pruneErr
		},
	})
	if err != nil {
		return Outcome{}, err
	}
	result.Orphaned = orphaned

	for _, pruned := range result.Pruned {
		f.purgeJobLogs(pruned.Branch)
	}

	if insidePruned {
		shell.RequestCd(f.ctx.ProjectDir)
	}
	return f.conclude(Outcome{Result: result})
}

// runHooks hooks every selected worktree before the first removal, which is
// observable behaviour: a hook failing at rank N aborts with nothing deleted, yet
// 1..N-1 already had their teardown — hence the idempotence on_clean requires.
func (f *pruneFlow) runHooks() error {
	hooks := f.ctx.Config.Project.Hooks.OnClean
	if len(hooks) == 0 {
		return nil
	}
	return f.presenter.HookPhase(flow.HookPhaseParams{
		Title:   domain.HooksTitleOnClean,
		LogPath: rules.HooksLogPath(rules.HooksLogPathParams{StateDir: f.ctx.StateDir, Phase: domain.HookOnClean}),
		Run: func(sink flow.HookSink) error {
			for _, candidate := range f.plan.Selected {
				if candidate.Path == "" {
					continue
				}
				if err := worktree.RunCleanHooks(domain.CleanHooksParams{
					ProjectDir:   f.ctx.ProjectDir,
					StateDir:     f.ctx.StateDir,
					WorktreePath: candidate.Path,
					Branch:       candidate.Branch,
					Hooks:        hooks,
					Output:       sink.Output,
					OnHook:       sink.OnHook,
				}); err != nil {
					return err
				}
			}
			return nil
		},
	})
}

// purgeJobLogs drops a removed worktree's persisted job logs. Best effort:
// leftover log files are not worth failing a removal that already happened.
func (f *pruneFlow) purgeJobLogs(branch string) {
	_ = process.PurgeWorktreeLogs(rules.WorktreeLogDir(rules.WorktreeLogDirParams{
		StateDir: f.ctx.StateDir,
		Branch:   branch,
	}))
}

func (f *pruneFlow) stopServices(branchName string) {
	wt, err := worktree.FindByBranch(worktree.FindByBranchParams{
		ProjectDir: f.ctx.ProjectDir,
		Branch:     branchName,
	})
	if err != nil {
		return
	}

	socket := process.SocketPath()
	if !process.IsDaemonRunning(socket) {
		// Nothing listening does not mean nothing running: a detached stack
		// outlives its daemon. The index is what says whether waking one is
		// worth a fork.
		if !process.HasIndexedJobs(wt.Path) {
			return
		}
		if err := process.EnsureDaemon(process.DaemonParams{SocketPath: socket}); err != nil {
			return
		}
	}
	if process.StopWorktreeJobs(process.NewClient(socket), wt.Path) {
		f.presenter.Status(flow.Notice{
			Kind: flow.NoticeSuccess,
			Text: fmt.Sprintf(domain.CleanStoppedServicesFmt, branchName),
		})
	}
}

// insidePruned must run before the removal: the paths have to still exist to
// canonicalize their symlinks.
func (f *pruneFlow) insidePruned() bool {
	cwd, err := os.Getwd()
	if err != nil || cwd == "" {
		return false
	}
	resolvedCwd := flow.ResolveSymlinks(cwd)
	for _, candidate := range f.plan.Selected {
		if candidate.Path == "" {
			continue
		}
		if rules.IsPathWithin(flow.ResolveSymlinks(candidate.Path), resolvedCwd) {
			return true
		}
	}
	return false
}

func (f *pruneFlow) conclude(outcome Outcome) (Outcome, error) {
	f.settleOwedNamespaces()
	return outcome, f.presenter.Pruned(outcome)
}

// settleOwedNamespaces gives back what a clean could not, because the shared
// service holding it was down at the time. A dry run settles nothing: it
// previews, and giving a namespace back is a mutation like any other.
func (f *pruneFlow) settleOwedNamespaces() {
	if f.request.DryRun {
		return
	}
	owed := runjobs.LoadPendingRemovals(f.ctx.StateDir)
	if len(owed) == 0 {
		return
	}
	cfg, err := runconfig.Load(f.ctx.StateDir)
	if err != nil {
		return
	}

	up := rules.SharedJobsUp(rules.SharedJobsUpParams{Jobs: runjobs.Load(), Config: cfg})
	var settled []domain.NamespaceRef
	for _, ref := range owed {
		if !up[ref.Job] {
			continue
		}
		result := runjobs.RemoveWorktreeNamespaces(runjobs.RemoveNamespacesParams{
			Config:  rules.JobsNamed(cfg, ref.Job),
			Env:     rules.NamespaceEnv(rules.NamespaceEnvParams{Worktree: ref.Worktree, Ordinal: ref.Ordinal}),
			WorkDir: f.ctx.ProjectDir,
			Up:      up,
		})
		if len(result.Released) == 0 {
			continue
		}
		settled = append(settled, ref)
		f.presenter.Status(flow.Notice{
			Kind: flow.NoticeSuccess,
			Text: fmt.Sprintf(domain.PruneSettledNamespaceFmt, ref.Job, ref.Worktree),
		})
	}
	_ = runjobs.SettleRemovals(runjobs.SettleRemovalsParams{StateDir: f.ctx.StateDir, Refs: settled})
}

func (f *pruneFlow) params() domain.PruneParams {
	return domain.PruneParams{
		ProjectDir: f.ctx.ProjectDir,
		StateDir:   f.ctx.StateDir,
		Config:     f.ctx.Config,
		BaseBranch: f.request.BaseBranch,
		Merged:     f.request.Merged,
		Closed:     f.request.Closed,
		Gone:       f.request.Gone,
		NoFetch:    f.request.NoFetch,
		Force: rules.PruneClassifyForce(rules.PruneClassifyForceParams{
			Force:       f.request.Force,
			Interactive: f.prompter.Interactive(),
			DryRun:      f.request.DryRun,
		}),
		DryRun: f.request.DryRun,
	}
}
