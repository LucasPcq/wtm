// Package env runs the `wtm env` flow.
package env

import (
	"errors"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Request struct {
	// Worktree is the branch the surface already named; empty leaves it to the
	// picker.
	Worktree   string
	Mode       domain.EnvMode
	From       string
	OnConflict domain.EnvConflictDecision
	Prune      bool
	// Check reports the drift and writes nothing: the run returns before asking.
	Check bool
	// Isolation settles the worktree on it, recorded only once its .env is in
	// line; empty keeps the recorded one unless the wizard picks another.
	Isolation domain.Isolation
}

type Outcome struct {
	Result domain.EnvSyncResult
	// IsolationChanged is a run that recorded a new isolation on the worktree —
	// the one change of its identity this command makes.
	IsolationChanged bool
	Aborted          bool
}

type Presenter interface {
	flow.Presenter
	Reconciled(Outcome) error
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

func Run(params Params) (Outcome, error) {
	f := &envFlow{
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
		scans:     map[string]branchScan{},
	}
	return f.run()
}

type envFlow struct {
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter

	// statuses and scans exist only for a run that asks: they feed the picker's
	// badges and every screen after it.
	statuses []domain.WorktreeStatus
	scans    map[string]branchScan
}

func (f *envFlow) run() (Outcome, error) {
	if f.request.Check {
		target, err := f.namedTarget()
		if err != nil {
			return Outcome{}, err
		}
		return f.apply(applyParams{Target: target})
	}

	if f.prompter.Interactive() {
		if err := f.presenter.Stage(flow.StageParams{Message: domain.EnvScanLoading, Work: f.scan}); err != nil {
			return Outcome{}, err
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

	target, err := f.answeredTarget(answers.Value(KeyWorktree))
	if err != nil {
		return Outcome{}, err
	}
	params := applyParams{Target: target, Isolation: f.isolation(answers)}
	if f.prompter.Interactive() {
		resolve, _ := answers.Get(KeyResolve)
		params.Resolutions = resolutions(resolve.EnvDecisions)
	}
	return f.apply(params)
}

// namedTarget is the worktree an unattended run was given: it has no picker to
// fall back on.
func (f *envFlow) namedTarget() (target, error) {
	if f.request.Worktree == "" {
		return target{}, domain.ErrEnvWorktreeRequired
	}
	return f.lookup(f.request.Worktree)
}

func (f *envFlow) answeredTarget(branch string) (target, error) {
	if path := pathOf(f.statuses, branch); path != "" {
		return target{branch: branch, path: path}, nil
	}
	return f.lookup(branch)
}

func (f *envFlow) lookup(branch string) (target, error) {
	wt, err := worktree.FindByBranch(worktree.FindByBranchParams{ProjectDir: f.ctx.ProjectDir, Branch: branch})
	if err != nil {
		return target{}, fmt.Errorf(domain.EnvWorktreeLookupFmt, branch, err)
	}
	return target{branch: wt.Branch, path: wt.Path}, nil
}

// isolation is the one the run settles the worktree on: --isolation, else the
// wizard's adoption or its verbatim action, else none — the recorded one stays.
func (f *envFlow) isolation(answers flow.Answers) domain.Isolation {
	if answers.Value(KeyAdopt) == domain.IsolationAdoptValue {
		return domain.IsolationIsolated
	}
	if answers.Value(KeyRecap) == domain.EnvApplyVerbatimValue {
		return domain.IsolationVerbatim
	}
	return f.request.Isolation
}

func resolutions(decisions []domain.EnvFileDecision) map[string]envsvc.EnvResolution {
	out := make(map[string]envsvc.EnvResolution, len(decisions))
	for _, d := range decisions {
		out[d.Target] = envsvc.EnvResolution{
			Decisions:    d.Decisions,
			FilledValues: d.FilledValues,
			PruneKeys:    toSet(d.PruneKeys),
			SkipKeys:     toSet(d.SkipKeys),
		}
	}
	return out
}

func toSet(keys []string) map[string]bool {
	if len(keys) == 0 {
		return nil
	}
	out := make(map[string]bool, len(keys))
	for _, k := range keys {
		out[k] = true
	}
	return out
}

type applyParams struct {
	Target    target
	Isolation domain.Isolation
	// Resolutions are the wizard's decisions; nil is a run driven by its flags.
	Resolutions map[string]envsvc.EnvResolution
}

// apply reconciles the keys, then settles the isolation: it is recorded last,
// once the .env is in line with it, so a run that fails records nothing.
func (f *envFlow) apply(params applyParams) (Outcome, error) {
	if err := f.checkIsolation(params.Target, params.Isolation); err != nil {
		return Outcome{}, err
	}
	adoption, err := f.adoption(params.Target)
	if err != nil {
		return Outcome{}, err
	}

	ctx := f.envContext(params.Target.branch)
	sw, err := f.planSwitch(planSwitchParams{Target: params.Target, Ctx: ctx, Isolation: params.Isolation})
	if err != nil {
		return Outcome{}, err
	}
	pass := f.runPass(runPassParams{Target: params.Target, Adoption: adoption, Isolation: params.Isolation, Reserved: sw.keys()})

	result, err := f.reconcile(reconcileParams{Target: params.Target, Ctx: ctx, Pass: pass, Resolutions: params.Resolutions})
	if err != nil {
		return Outcome{}, err
	}

	settled, err := f.settleIsolation(sw)
	if err != nil {
		return Outcome{}, err
	}
	result = settled.decorate(pass.decorate(f.ctx, result))
	outcome := Outcome{Result: result, IsolationChanged: settled.changed}
	return outcome, f.presenter.Reconciled(outcome)
}

type reconcileParams struct {
	Target      target
	Ctx         envContext
	Pass        envRunPass
	Resolutions map[string]envsvc.EnvResolution
}

func (f *envFlow) reconcile(params reconcileParams) (domain.EnvSyncResult, error) {
	if params.Resolutions != nil {
		return envsvc.ApplyEnvSync(envsvc.ApplyEnvSyncParams{
			Branch:             params.Target.branch,
			MainPath:           f.ctx.ProjectDir,
			WorktreePath:       params.Target.path,
			ParentWorktreePath: params.Ctx.parentPath,
			ParentBranch:       params.Ctx.parentBranch,
			Files:              f.ctx.Config.Project.Env.Files,
			Strategy:           params.Ctx.strategy,
			Mode:               f.request.Mode,
			Resolutions:        params.Resolutions,
			Ports:              params.Pass.ports,
			Reserved:           params.Pass.reserved,
		})
	}
	return envsvc.SyncEnv(envsvc.SyncEnvParams{
		Branch:             params.Target.branch,
		MainPath:           f.ctx.ProjectDir,
		WorktreePath:       params.Target.path,
		ParentWorktreePath: params.Ctx.parentPath,
		ParentBranch:       params.Ctx.parentBranch,
		Files:              f.ctx.Config.Project.Env.Files,
		Strategy:           params.Ctx.strategy,
		Mode:               f.request.Mode,
		Prune:              f.request.Prune,
		Check:              f.request.Check,
		OnConflict:         f.request.OnConflict,
		Ports:              params.Pass.ports,
		Reserved:           params.Pass.reserved,
	})
}

// settleIsolation is where the worktree's identity changes. It reports whether
// it did: a run asked for the isolation the worktree already has records
// nothing new.
func (f *envFlow) settleIsolation(sw envSwitch) (switchOutcome, error) {
	if sw.isolation == "" || sw.warning != "" {
		return switchOutcome{warning: sw.warning}, nil
	}

	var outcome switchOutcome
	if rules.IsVerbatim(sw.isolation) {
		restored, err := envsvc.ApplyOwnedRestore(sw.restore)
		if err != nil {
			return switchOutcome{}, err
		}
		outcome.restored = restored
	}

	before := worktree.RecordedIsolation(sw.ref)
	if err := worktree.SetIsolation(worktree.SetIsolationParams{Ref: sw.ref, Isolation: sw.isolation}); err != nil {
		return switchOutcome{}, err
	}
	outcome.changed = worktree.RecordedIsolation(sw.ref) != before
	return outcome, nil
}

// checkIsolation refuses an isolation the worktree cannot take before anything
// is written for it.
func (f *envFlow) checkIsolation(t target, isolation domain.Isolation) error {
	if isolation == "" {
		return nil
	}
	return worktree.CheckIsolation(worktree.SetIsolationParams{Ref: f.ref(t.branch), Isolation: isolation})
}

func (f *envFlow) adoption(t target) (domain.IsolationAdoptionPlan, error) {
	return worktree.IsolationAdoptionFor(worktree.IsolationAdoptionParams{
		Ref:          f.ref(t.branch),
		WorktreePath: t.path,
	})
}

func (f *envFlow) ref(branch string) worktree.WorktreeRef {
	return worktree.WorktreeRef{ProjectDir: f.ctx.ProjectDir, StateDir: f.ctx.StateDir, Branch: branch}
}

type target struct {
	branch string
	path   string
}

func pathOf(statuses []domain.WorktreeStatus, branch string) string {
	for _, s := range statuses {
		if s.Branch == branch {
			return s.Path
		}
	}
	return ""
}

// envContext is the resolved value strategy plus the recorded parent branch
// and its worktree path — empty when it has none, so "parent" falls back to
// main.
type envContext struct {
	strategy     domain.EnvStrategy
	parentBranch string
	parentPath   string
}

func (f *envFlow) envContext(branch string) envContext {
	base := f.ctx.Config.Project.Env.Strategy
	parentBranch := ""
	if meta, ok := worktree.Metadata(worktree.ParentBranchParams{StateDir: f.ctx.StateDir, Branch: branch}); ok {
		if meta.EnvStrategy != "" {
			base = meta.EnvStrategy
		}
		parentBranch = meta.SourceBranch
	}

	parentPath := ""
	if parentBranch != "" {
		if wt, err := worktree.FindByBranch(worktree.FindByBranchParams{ProjectDir: f.ctx.ProjectDir, Branch: parentBranch}); err == nil {
			parentPath = wt.Path
		}
	}
	return envContext{
		strategy:     rules.ResolveEnvStrategy(base, f.request.From),
		parentBranch: parentBranch,
		parentPath:   parentPath,
	}
}
