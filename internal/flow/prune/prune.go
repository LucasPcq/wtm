// Package prune runs the `wtm prune` flow.
package prune

import (
	"errors"
	"fmt"
	"os"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/owed"
	"github.com/LucasPcq/wtm/internal/flow/teardown"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/github"
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
	// KeepData withholds the namespaces the pruned worktrees carved out of shared
	// services, which are otherwise given back as clean gives them back.
	KeepData bool
	// DropData drops it now instead, starting the services that are down.
	DropData bool
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

	plan      domain.PrunePlan
	snapshots map[string]owed.Snapshot
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

	force := f.request.Force || answers.Value(KeyConfirm) == confirmForce
	f.plan = rules.FinalizePrunePlan(rules.FinalizePrunePlanParams{
		Plan:       f.plan,
		Chosen:     answers.Values(KeySelection),
		BaseBranch: f.request.BaseBranch,
		Force:      force,
	})
	if len(f.plan.Selected) == 0 {
		return f.conclude(Outcome{Empty: true})
	}

	return f.remove(removeParams{
		ReparentChildren: answers.Value(KeyReparent) == reparentYes,
		StartDown:        answers.Value(KeyData) == owed.DataStart,
		Force:            force,
	})
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

type removeParams struct {
	ReparentChildren bool
	StartDown        bool
	Force            bool
}

// remove runs the whole teardown on one worktree before moving to the next, and
// stops at the first that fails: the ones after it keep their worktree and
// their data, and the ones before it are gone with theirs.
func (f *pruneFlow) remove(params removeParams) (Outcome, error) {
	cwdTarget := f.cwdCandidate()

	// Old debts first: their lines would otherwise repeat the ones this removal
	// is about to print for the same services.
	f.settleOwedNamespaces()

	// Read before the first removal, while every worktree's environment exists.
	dropper := owed.NewDropper(owed.DropperParams{
		Context:   f.ctx,
		Presenter: f.presenter,
		Snapshot:  owed.Read(owed.ReadParams{Context: f.ctx, Branches: f.selectedBranches()}),
		StartDown: params.StartDown,
		KeepData:  f.request.KeepData,
	})

	result := domain.PruneResult{
		Pruned:     []domain.PruneCandidate{},
		Reparented: []domain.ReparentResult{},
		Skipped:    f.plan.Skipped,
		Namespaces: []domain.NamespaceOutcome{},
	}
	var removed []teardown.Target
	var failure error
	for _, candidate := range f.plan.Selected {
		target := teardown.Target{Branch: candidate.Branch, Path: candidate.Path}
		namespaces, err := f.removeOne(removeOneParams{Target: target, Dropper: dropper, Force: params.Force})
		if err != nil {
			failure = err
			result.Failed = &domain.PruneFailure{Branch: candidate.Branch, Path: candidate.Path, Error: err.Error()}
			break
		}
		removed = append(removed, target)
		result.Pruned = append(result.Pruned, candidate)
		result.Namespaces = append(result.Namespaces, namespaces...)
	}

	// Claims go last, all together: the one a pruned worktree released may be
	// what kept a service up for the next one's drop.
	for _, target := range removed {
		teardown.Release(teardown.ReleaseParams{Presenter: f.presenter, Target: target})
	}
	dropper.Close()

	reparents := reparentsOf(reparentsOfParams{Reparents: f.plan.Reparents, Pruned: result.Pruned})
	if params.ReparentChildren {
		applied, err := worktree.ApplyReparents(worktree.ApplyReparentsParams{Reparents: reparents, StateDir: f.ctx.StateDir})
		if err != nil {
			return Outcome{}, err
		}
		result.Reparented = append(result.Reparented, applied...)
	} else {
		result.Orphaned = reparents
	}

	if cwdTarget != "" && pruned(result.Pruned, cwdTarget) {
		shell.RequestCd(f.ctx.ProjectDir)
	}
	outcome := Outcome{Result: result}
	if err := f.presenter.Pruned(outcome); err != nil {
		return outcome, err
	}
	if failure != nil {
		return outcome, fmt.Errorf("%w: %w", domain.ErrAborted, failure)
	}
	return outcome, nil
}

type removeOneParams struct {
	Target  teardown.Target
	Dropper *owed.Dropper
	Force   bool
}

func (f *pruneFlow) removeOne(params removeOneParams) ([]domain.NamespaceOutcome, error) {
	target := params.Target
	if err := teardown.Stop(teardown.StopParams{Context: f.ctx, Presenter: f.presenter, Target: target, Force: params.Force}); err != nil {
		return nil, err
	}
	if err := teardown.Hooks(teardown.HooksParams{
		Context:   f.ctx,
		Presenter: f.presenter,
		Target:    target,
		Title:     fmt.Sprintf(domain.HooksTitleOnCleanFmt, target.Branch),
	}); err != nil {
		return nil, err
	}

	clean := domain.CleanParams{
		ProjectDir: f.ctx.ProjectDir,
		StateDir:   f.ctx.StateDir,
		Branch:     target.Branch,
		// Safety was decided during classification.
		Force:      true,
		BaseBranch: f.request.BaseBranch,
		Config:     f.ctx.Config,
		SkipHooks:  true,
	}
	err := f.presenter.Stage(flow.StageParams{
		Message: fmt.Sprintf(domain.CleanLoadingFmt, target.Branch),
		Work:    func() error { return worktree.Clean(clean) },
	})
	if errors.Is(err, domain.ErrWorktreeRemoveFailed) {
		err = teardown.Salvage(teardown.SalvageParams{Presenter: f.presenter, Clean: clean, Path: target.Path, Cause: err})
	}
	// An absent worktree was already pruned, which keeps a re-run idempotent.
	if err != nil && !errors.Is(err, domain.ErrWorktreeNotFound) {
		return nil, err
	}
	return teardown.Reclaim(teardown.ReclaimParams{Context: f.ctx, Target: target, Dropper: params.Dropper}), nil
}

func (f *pruneFlow) selectedBranches() []string {
	branches := make([]string, 0, len(f.plan.Selected))
	for _, candidate := range f.plan.Selected {
		branches = append(branches, candidate.Branch)
	}
	return branches
}

type reparentsOfParams struct {
	Reparents []domain.ReparentResult
	Pruned    []domain.PruneCandidate
}

// reparentsOf keeps the moves whose old parent is actually gone: a prune that
// stopped short leaves the children of what it did not reach where they are.
func reparentsOf(params reparentsOfParams) []domain.ReparentResult {
	moves := []domain.ReparentResult{}
	for _, move := range params.Reparents {
		if pruned(params.Pruned, move.OldParent) {
			moves = append(moves, move)
		}
	}
	return moves
}

func pruned(candidates []domain.PruneCandidate, branch string) bool {
	for _, candidate := range candidates {
		if candidate.Branch == branch {
			return true
		}
	}
	return false
}

// cwdCandidate is the selected worktree the shell stands in, if any. Decided
// before the removal, while the paths still resolve their symlinks.
func (f *pruneFlow) cwdCandidate() string {
	cwd, err := os.Getwd()
	if err != nil || cwd == "" {
		return ""
	}
	resolvedCwd := flow.ResolveSymlinks(cwd)
	for _, candidate := range f.plan.Selected {
		if candidate.Path == "" {
			continue
		}
		if rules.IsPathWithin(flow.ResolveSymlinks(candidate.Path), resolvedCwd) {
			return candidate.Branch
		}
	}
	return ""
}

func (f *pruneFlow) conclude(outcome Outcome) (Outcome, error) {
	f.settleOwedNamespaces()
	return outcome, f.presenter.Pruned(outcome)
}

// settleOwedNamespaces gives back what a clean could not, because the shared
// service holding it was down at the time, and says what is still owed rather
// than passing over it. A dry run settles nothing: it previews, and giving a
// namespace back is a mutation like any other.
func (f *pruneFlow) settleOwedNamespaces() {
	if f.request.DryRun {
		return
	}
	result := owed.Settle(owed.Params{Context: f.ctx, Presenter: f.presenter})
	for _, line := range rules.OwedLines(result.Owed) {
		f.presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: line})
	}
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
