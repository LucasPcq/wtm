// Package sync runs the `wtm sync` flow.
package sync

import (
	"context"
	"errors"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Request struct {
	// Branches and All fix the selection (branch args / --all): the step is not
	// asked, but stays read back in the recap.
	Branches []string
	All      bool
	// Precheck is what arrives checked when the selection step IS asked. A surface
	// that already knows a likely answer offers it; the selection stays exact.
	Precheck     []string
	KeepConflict bool
	FFParents    bool
	NoFFParents  bool
	Push         bool
	NoPush       bool
	// DryRun previews the cascade and rebases nothing.
	DryRun     bool
	BaseBranch string
}

type Outcome struct {
	Result domain.SyncResult
	Plan   domain.SyncPlan
	// Empty reports that the selection resolved to nothing to rebase.
	Empty   bool
	Aborted bool
}

// Presenter carries the three moments of a sync: the plan a run that cannot ask
// has to print, the recap the user reads before being asked to push, and the
// conclusion. They are separate because the push question falls between the last
// two.
type Presenter interface {
	flow.Presenter
	Planned(domain.SyncPlan)
	Rebased(domain.SyncResult)
	Synced(Outcome) error
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

// Operation declares how a surface must schedule a sync: it rebases several
// worktrees and asks first, so it holds the surface for its whole run and needs
// no per-worktree lock.
func Operation() flow.Operation {
	return flow.Operation{Kind: domain.OpKindSync, Mode: flow.ModeBlocking}
}

type syncFlow struct {
	runCtx    context.Context
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter

	statuses   []domain.WorktreeStatus
	classified []domain.ParentUpdate
	// stale memoizes the parent narrowing per selection: a step rebuilt on every
	// keystroke reads it from Skip and again from Build.
	stale map[string][]domain.ParentUpdate
	// selection is what the branch arguments resolved to, kept here rather than
	// written back over the request.
	selection []string
}

func Run(ctx context.Context, params Params) (Outcome, error) {
	f := &syncFlow{
		runCtx:    ctx,
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
	}
	return f.run()
}

func (f *syncFlow) run() (Outcome, error) {
	if err := f.load(); err != nil {
		return Outcome{}, err
	}

	answers, err := f.prompter.Ask(f.session())
	if errors.Is(err, domain.ErrUserAborted) {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}
	if err != nil {
		return Outcome{}, err
	}
	if answers.Value(KeyConfirm) == domain.WizardCancelValue {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}

	syncParams := f.syncParams(syncParamsInput{
		Selected:     answers.Values(KeySelection),
		KeepConflict: answers.Value(KeyConflict) == conflictKeep,
		FastForward:  answers.Value(KeyParents) == parentFF,
	})

	// Rebuilt for the answered selection: what the recap memoized served the
	// preview, and the answers may have narrowed it since.
	plan, err := worktree.PlanSync(f.runCtx, syncParams)
	if err != nil {
		return Outcome{}, err
	}
	if len(plan.Steps) == 0 && !rules.SyncIncludesBase(rules.SyncIncludesBaseParams{
		Selected:   syncParams.SelectedBranches,
		BaseBranch: f.request.BaseBranch,
	}) {
		return f.conclude(Outcome{
			Empty:  true,
			Result: domain.SyncResult{BaseBranch: f.request.BaseBranch},
		})
	}

	// The plan is shown here exactly when the recap did not show it: unattended, or
	// previewing (--dry-run skips the recap). An empty cascade has none to show.
	if len(plan.Steps) > 0 && !answers.Answered(KeyConfirm) {
		f.presenter.Planned(plan)
	}

	result, err := f.rebase(syncParams)
	if err != nil {
		return Outcome{}, err
	}
	f.presenter.Rebased(result)

	if !f.request.DryRun && f.shouldPush(result) {
		pushed, pushErr := f.push(result)
		if pushErr != nil {
			return Outcome{}, pushErr
		}
		result = pushed
	}

	outcome, err := f.conclude(Outcome{Result: result, Plan: plan})
	if err != nil {
		return outcome, err
	}
	if !f.request.DryRun && rules.HasSyncFailure(result.Steps) {
		return outcome, domain.ErrAborted
	}
	return outcome, nil
}

// load reads what the session needs before it can ask anything. The parent scan
// is I/O, so only the run that could actually ask pays for it; every other
// outcome is settled by the flags alone.
func (f *syncFlow) load() error {
	statuses, err := worktree.List(f.runCtx, domain.ListParams{
		ProjectDir: f.ctx.ProjectDir,
		StateDir:   f.ctx.StateDir,
		Config:     f.ctx.Config,
	})
	if err != nil {
		return err
	}
	f.statuses = statuses

	selection, err := f.resolvedBranches()
	if err != nil {
		return err
	}
	f.selection = selection

	if !f.prompter.Interactive() || f.request.DryRun {
		return nil
	}
	return f.scanParents()
}

func (f *syncFlow) resolvedBranches() ([]string, error) {
	if len(f.request.Branches) == 0 {
		return nil, nil
	}
	return worktree.ResolveSyncBranches(f.runCtx, worktree.ResolveSyncBranchesParams{
		ProjectDir: f.ctx.ProjectDir,
		Queries:    f.request.Branches,
	})
}

func (f *syncFlow) scanParents() error {
	return f.presenter.Stage(f.runCtx, flow.StageParams{
		Message: domain.SyncParentScanning,
		Work: func(ctx context.Context) error {
			classified, err := worktree.ClassifyParents(ctx, worktree.ClassifyParentsParams{
				ProjectDir: f.ctx.ProjectDir,
				StateDir:   f.ctx.StateDir,
				BaseBranch: f.request.BaseBranch,
			})
			f.classified = classified
			return err
		},
	})
}

func (f *syncFlow) rebase(params worktree.SyncParams) (domain.SyncResult, error) {
	var result domain.SyncResult
	err := f.presenter.Stage(f.runCtx, flow.StageParams{
		Message: domain.SyncRebasing,
		Work: func(ctx context.Context) error {
			var syncErr error
			result, syncErr = worktree.Sync(ctx, params)
			return syncErr
		},
	})
	return result, err
}

func (f *syncFlow) push(result domain.SyncResult) (domain.SyncResult, error) {
	pushed := result
	err := f.presenter.Stage(f.runCtx, flow.StageParams{
		Message: domain.SyncPushing,
		Work: func(ctx context.Context) error {
			pushed = worktree.PushSynced(ctx, worktree.PushSyncedParams{
				ProjectDir: f.ctx.ProjectDir,
				Result:     result,
			})
			return nil
		},
	})
	return pushed, err
}

// shouldPush resolves the pure decision and, when it asks, asks after the run —
// the user reads what happened before deciding to publish it.
func (f *syncFlow) shouldPush(result domain.SyncResult) bool {
	ready := rules.PushableCount(result.Steps)
	switch rules.DecidePush(rules.DecidePushParams{
		Push:          f.request.Push,
		NoPush:        f.request.NoPush,
		Interactive:   f.prompter.Interactive(),
		PushableCount: ready,
	}) {
	case rules.PushForce:
		return true
	case rules.PushConfirm:
		// DefaultYes stays false: force-pushing with lease is opt-in, so keeping the
		// branches local leads.
		confirmed, err := f.prompter.Confirm(flow.ConfirmParams{
			Title:    fmt.Sprintf(domain.SyncPushPrompt, ready),
			Warning:  domain.SyncPushWarning,
			YesLabel: domain.SyncPushOption,
			NoLabel:  domain.SyncKeepLocalOption,
		})
		return err == nil && confirmed
	default:
		return false
	}
}

func (f *syncFlow) conclude(outcome Outcome) (Outcome, error) {
	return outcome, f.presenter.Synced(outcome)
}
