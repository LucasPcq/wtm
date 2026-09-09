// Package clean runs the `wtm clean` flow.
package clean

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/service/runjobs"
	"github.com/LucasPcq/wtm/internal/service/shell"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Request struct {
	Branch string
	// Force is the safety axis: it lifts the dirty/unpushed/open-PR refusals.
	Force            bool
	ReparentChildren bool
	BaseBranch       string
	// AllowPrivileged lets a failed removal offer the `sudo rm -rf` fallback. The
	// prompt it opens belongs to sudo and takes the terminal, so only a surface
	// that can hand it over sets this.
	AllowPrivileged bool
	// KeepData withholds the namespaces this worktree carved out of shared
	// services. The default is to give them back: clean is the destructive
	// command, and removing a worktree without its data would leave an orphan
	// database behind on every iteration.
	KeepData bool
}

type Outcome struct {
	Branch           string
	Path             string
	AlreadyAbsent    bool
	Reparented       []domain.ReparentResult
	OrphanedChildren []domain.ReparentResult
	Aborted          bool
}

type Presenter interface {
	flow.Presenter
	Cleaned(Outcome) error
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

// Operation declares how a surface must schedule a clean: it destroys its target,
// so it holds the surface until it is done rather than running behind its back.
func Operation() flow.Operation {
	return flow.Operation{Kind: domain.OpKindClean, Mode: flow.ModeBlocking, TargetKey: KeyWorktree}
}

func Run(params Params) (Outcome, error) {
	f := &cleanFlow{
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
		checks:    make(map[string]checkResult),
	}
	return f.run()
}

// checkResult caches one safety check: it queries the PR state over the network.
type checkResult struct {
	check domain.CleanCheckResult
	err   error
}

type cleanFlow struct {
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter
	checks    map[string]checkResult
}

func (f *cleanFlow) run() (Outcome, error) {
	// Only an interactive run pre-flights, so an absent or parent worktree is
	// reported without a single question. The prompt-free path lets the removal
	// report both idempotently, which is what its machine payload says.
	if f.request.Branch != "" && f.prompter.Interactive() {
		_, err := f.checkStaged(f.request.Branch)
		if handled, outcome, hErr := f.handleCheckError(f.request.Branch, err); handled {
			return outcome, hErr
		}
	}

	answers, err := f.prompter.Ask(f.session())
	if errors.Is(err, domain.ErrUserAborted) {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}
	if err != nil {
		return Outcome{}, err
	}

	branchName := answers.Value(KeyWorktree)
	force := f.request.Force || answers.Value(KeyDelete) == deleteForce

	plan := f.reparentPlan(branchName)
	return f.remove(removeParams{
		Params:        f.cleanParams(branchName, force),
		ReparentPlan:  plan,
		ApplyReparent: len(plan.Children) > 0 && answers.Value(KeyReparent) == reparentYes,
	})
}

func (f *cleanFlow) handleCheckError(branchName string, err error) (bool, Outcome, error) {
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		outcome := Outcome{Branch: branchName, AlreadyAbsent: true}
		return true, outcome, f.presenter.Cleaned(outcome)
	}
	if errors.Is(err, domain.ErrCannotCleanParent) {
		f.presenter.Notice(flow.Notice{Kind: flow.NoticeWarning, Text: domain.CleanCannotCleanParent})
		return true, Outcome{Branch: branchName}, nil
	}
	if err != nil {
		return true, Outcome{}, err
	}
	return false, Outcome{}, nil
}

type removeParams struct {
	Params        domain.CleanParams
	ReparentPlan  domain.CleanReparentPlan
	ApplyReparent bool
}

func (f *cleanFlow) remove(p removeParams) (Outcome, error) {
	params := p.Params
	worktreePath := ""
	if wt, err := worktree.FindByBranch(worktree.FindByBranchParams{
		ProjectDir: params.ProjectDir,
		Branch:     params.Branch,
	}); err == nil {
		worktreePath = wt.Path
	}

	// Decided before the removal, while the paths still resolve.
	cwd, _ := os.Getwd()
	insideRemoved := worktreePath != "" && cwd != "" &&
		rules.IsPathWithin(flow.ResolveSymlinks(worktreePath), flow.ResolveSymlinks(cwd))

	f.removeNamespaces(params.Branch)
	f.stopServices(params.Branch)

	// Hooks run as their own phase before the removal, so they don't fight the
	// removal progress for the terminal; the service then skips them.
	params.SkipHooks = true
	if hookErr := f.runHooks(worktreePath, params.Branch); hookErr != nil {
		return Outcome{}, hookErr
	}

	err := f.presenter.Stage(flow.StageParams{
		Message: fmt.Sprintf(domain.CleanLoadingFmt, params.Branch),
		Work:    func() error { return worktree.Clean(params) },
	})
	if errors.Is(err, domain.ErrWorktreeNotFound) {
		outcome := Outcome{Branch: params.Branch, AlreadyAbsent: true}
		return outcome, f.presenter.Cleaned(outcome)
	}
	if errors.Is(err, domain.ErrCannotCleanParent) {
		f.presenter.Notice(flow.Notice{Kind: flow.NoticeWarning, Text: domain.CleanCannotCleanParent})
		return Outcome{Branch: params.Branch}, nil
	}
	if errors.Is(err, domain.ErrWorktreeRemoveFailed) {
		recovered, rErr := f.recoverRemoveFailure(recoverParams{
			Params: params,
			Path:   worktreePath,
			Cause:  err,
		})
		if rErr != nil {
			return Outcome{}, rErr
		}
		if !recovered {
			return Outcome{}, err
		}
		err = nil
	}
	if err != nil {
		return Outcome{}, err
	}

	f.purgeJobLogs(params.Branch)

	if insideRemoved {
		shell.RequestCd(params.ProjectDir)
	}

	reparented, reparentErr := f.applyReparent(p.ReparentPlan, p.ApplyReparent)
	if reparentErr != nil {
		return Outcome{}, reparentErr
	}

	outcome := Outcome{
		Branch:           params.Branch,
		Path:             worktreePath,
		Reparented:       reparented,
		OrphanedChildren: orphanedChildren(p.ReparentPlan, p.ApplyReparent),
	}
	return outcome, f.presenter.Cleaned(outcome)
}

// purgeJobLogs drops the removed worktree's persisted job logs. Best effort:
// leftover log files are not worth failing a removal that already happened.
func (f *cleanFlow) purgeJobLogs(branch string) {
	_ = process.PurgeWorktreeLogs(rules.WorktreeLogDir(rules.WorktreeLogDirParams{
		StateDir: f.ctx.StateDir,
		Branch:   branch,
	}))
}

// removeNamespaces gives back what this worktree carved out of the shared
// services, before stopServices releases its claims: a claim released may be
// the last one, and a namespace cannot be given back to a service that is down.
func (f *cleanFlow) removeNamespaces(branchName string) {
	if f.request.KeepData {
		return
	}
	cfg, err := runconfig.Load(f.ctx.StateDir)
	if err != nil || len(cfg.Jobs) == 0 {
		return
	}
	wt, err := worktree.FindByBranch(worktree.FindByBranchParams{
		ProjectDir: f.ctx.ProjectDir,
		Branch:     branchName,
	})
	if err != nil {
		return
	}
	env, err := worktree.JobEnv(worktree.JobEnvParams{
		ProjectDir: f.ctx.ProjectDir,
		StateDir:   f.ctx.StateDir,
		Dir:        wt.Path,
	})
	if err != nil {
		return
	}

	// Only what this worktree actually carved out. A worktree created and thrown
	// away without ever starting the stack owes nothing, and running its detach
	// would be a DROP DATABASE on a database that never existed.
	held := worktree.NamespacesOf(worktree.ParentBranchParams{StateDir: f.ctx.StateDir, Branch: branchName})
	if len(held) == 0 {
		return
	}

	result := runjobs.RemoveWorktreeNamespaces(runjobs.RemoveNamespacesParams{
		Config:  rules.JobsHeld(cfg, held),
		Env:     env,
		WorkDir: wt.Path,
		Up:      rules.SharedJobsUp(rules.SharedJobsUpParams{Jobs: runjobs.Load(), Config: cfg}),
	})
	f.reportNamespaces(result)

	if err := runjobs.QueueRemovals(runjobs.QueueRemovalsParams{StateDir: f.ctx.StateDir, Refs: result.Deferred}); err != nil {
		f.presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: err.Error()})
	}
}

func (f *cleanFlow) reportNamespaces(result runjobs.RemoveNamespacesResult) {
	for _, ref := range result.Released {
		f.presenter.Status(flow.Notice{
			Kind: flow.NoticeSuccess,
			Text: fmt.Sprintf(domain.CleanRemovedNamespaceFmt, ref.Worktree, ref.Job),
		})
	}
	for _, ref := range result.Deferred {
		f.presenter.Status(flow.Notice{
			Kind: flow.NoticeWarning,
			Text: fmt.Sprintf(domain.CleanDeferredNamespaceFmt, ref.Job, ref.Worktree),
		})
	}
	for _, err := range result.Errs {
		f.presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: err.Error()})
	}
}

func (f *cleanFlow) stopServices(branchName string) {
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

func (f *cleanFlow) runHooks(worktreePath, branchName string) error {
	hooks := f.ctx.Config.Project.Hooks.OnClean
	if len(hooks) == 0 || worktreePath == "" {
		return nil
	}
	return f.presenter.HookPhase(flow.HookPhaseParams{
		Title: domain.HooksTitleOnClean,
		Run: func(sink io.Writer) error {
			return worktree.RunCleanHooks(domain.CleanHooksParams{
				ProjectDir:   f.ctx.ProjectDir,
				StateDir:     f.ctx.StateDir,
				WorktreePath: worktreePath,
				Branch:       branchName,
				Hooks:        hooks,
				Output:       sink,
			})
		},
	})
}

type recoverParams struct {
	Params domain.CleanParams
	Path   string
	Cause  error
}

// recoverRemoveFailure offers the privileged removal when `git worktree remove`
// failed on files the current user cannot delete (typically root-owned files left by
// a container). recovered=true resumes the normal post-removal flow.
func (f *cleanFlow) recoverRemoveFailure(p recoverParams) (bool, error) {
	if !f.request.AllowPrivileged || !f.prompter.Interactive() || p.Path == "" {
		return false, nil
	}

	f.presenter.Status(flow.Notice{
		Kind: flow.NoticeWarning,
		Text: fmt.Sprintf(domain.CleanRemovalFailedFmt, p.Cause),
	})

	confirmed, err := f.prompter.Confirm(flow.ConfirmParams{
		Title:      fmt.Sprintf(domain.CleanSudoConfirmFmt, p.Path),
		DefaultYes: false,
	})
	if err != nil || !confirmed {
		return false, nil
	}

	if forceErr := worktree.ForceClean(domain.ForceCleanParams{
		ProjectDir: p.Params.ProjectDir,
		StateDir:   p.Params.StateDir,
		Path:       p.Path,
		Branch:     p.Params.Branch,
		Force:      p.Params.Force,
	}); forceErr != nil {
		return false, forceErr
	}
	return true, nil
}

func (f *cleanFlow) applyReparent(plan domain.CleanReparentPlan, apply bool) ([]domain.ReparentResult, error) {
	if !apply || len(plan.Children) == 0 {
		return nil, nil
	}
	return worktree.ApplyReparentChildren(worktree.ApplyReparentChildrenParams{
		Plan:     plan,
		StateDir: f.ctx.StateDir,
	})
}

func orphanedChildren(plan domain.CleanReparentPlan, apply bool) []domain.ReparentResult {
	if apply || len(plan.Children) == 0 {
		return nil
	}
	return plan.Children
}

func (f *cleanFlow) cleanParams(branchName string, force bool) domain.CleanParams {
	return domain.CleanParams{
		ProjectDir: f.ctx.ProjectDir,
		StateDir:   f.ctx.StateDir,
		Branch:     branchName,
		Force:      force,
		BaseBranch: f.request.BaseBranch,
		Config:     f.ctx.Config,
	}
}

// reparentPlan lists the children a removal would orphan. Force plays no part in it.
func (f *cleanFlow) reparentPlan(branchName string) domain.CleanReparentPlan {
	return worktree.PlanCleanReparent(f.cleanParams(branchName, false))
}

func (f *cleanFlow) checkStaged(branchName string) (domain.CleanCheckResult, error) {
	if cached, ok := f.checks[branchName]; ok {
		return cached.check, cached.err
	}
	var result checkResult
	if err := f.presenter.Stage(flow.StageParams{
		Message: domain.CleanCheckLoading,
		Work: func() error {
			result.check, result.err = worktree.Check(f.cleanParams(branchName, false))
			return nil
		},
	}); err != nil {
		return domain.CleanCheckResult{}, err
	}
	f.checks[branchName] = result
	return result.check, result.err
}

// checkCached skips the progress indicator: the host loading a step already shows
// one of its own.
func (f *cleanFlow) checkCached(branchName string) (domain.CleanCheckResult, error) {
	if cached, ok := f.checks[branchName]; ok {
		return cached.check, cached.err
	}
	var result checkResult
	result.check, result.err = worktree.Check(f.cleanParams(branchName, false))
	f.checks[branchName] = result
	return result.check, result.err
}
