// Package fastforward runs the `wtm fast-forward` flow.
package fastforward

import (
	"context"
	"errors"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/service/branch"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Request struct {
	// Branches and All fix the selection (branch args / --all): the step is not
	// asked, but stays read back in the recap.
	Branches []string
	All      bool
	// Precheck is what arrives checked when the selection step IS asked.
	Precheck []string
	// Force lifts the dirty refusal, and only that one.
	Force bool
}

type Outcome struct {
	Results []domain.FastForwardResult
	// Empty reports that the selection resolved to nothing.
	Empty   bool
	Aborted bool
}

type Presenter interface {
	flow.Presenter
	FastForwarded(Outcome) error
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

// Operation declares how a surface must schedule a fast-forward: it asks before
// it acts and its selection may span several worktrees, so it holds the surface
// for its whole run rather than locking one target.
func Operation() flow.Operation {
	return flow.Operation{Kind: domain.OpKindFastForward, Mode: flow.ModeBlocking}
}

func Run(ctx context.Context, params Params) (Outcome, error) {
	f := &fastForwardFlow{
		runCtx:    ctx,
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
		checks:    map[string]domain.FastForwardCheck{},
	}
	return f.run()
}

type fastForwardFlow struct {
	runCtx    context.Context
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter

	statuses  []domain.WorktreeStatus
	selection []string
	// checks memoizes what Check fetched: the recap needs it, Resolve needs it,
	// and the run hands it back to the service rather than fetching again.
	checks map[string]domain.FastForwardCheck
}

func (f *fastForwardFlow) run() (Outcome, error) {
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

	selected := answers.Values(KeySelection)
	if len(selected) == 0 {
		return f.conclude(Outcome{Empty: true})
	}

	// The danger option is an answer like any other, so it is read back here:
	// --force is the flag saying the same thing before anything was asked.
	results, err := f.advance(advanceParams{
		Branches: selected,
		Force:    f.request.Force || answers.Value(KeyConfirm) == confirmForce,
	})
	if err != nil {
		return Outcome{}, err
	}

	outcome, err := f.conclude(Outcome{Results: results})
	if err != nil {
		return outcome, err
	}
	if hasFailure(results) {
		return outcome, domain.ErrAborted
	}
	return outcome, nil
}

func (f *fastForwardFlow) load() error {
	statuses, err := worktree.List(f.runCtx, domain.ListParams{
		ProjectDir: f.ctx.ProjectDir,
		StateDir:   f.ctx.StateDir,
		Config:     f.ctx.Config,
	})
	if err != nil {
		return err
	}
	f.statuses = statuses

	if len(f.request.Branches) == 0 {
		return nil
	}
	selection, err := worktree.ResolveSyncBranches(f.runCtx, worktree.ResolveSyncBranchesParams{
		ProjectDir: f.ctx.ProjectDir,
		Queries:    f.request.Branches,
	})
	if err != nil {
		return err
	}
	f.selection = selection
	return nil
}

type advanceParams struct {
	Branches []string
	Force    bool
}

func (f *fastForwardFlow) advance(params advanceParams) ([]domain.FastForwardResult, error) {
	var results []domain.FastForwardResult
	err := f.presenter.Stage(f.runCtx, flow.StageParams{
		Message: domain.FastForwardStage,
		Work: func(ctx context.Context) error {
			for _, name := range params.Branches {
				check := f.check(name)
				results = append(results, branch.FastForward(ctx, branch.FastForwardParams{
					ProjectDir: f.ctx.ProjectDir,
					Branch:     name,
					Force:      params.Force,
					Check:      &check,
				}))
			}
			return nil
		},
	})
	return results, err
}

// check reads the memo, filling it on the first read: an unattended run reaches
// the branches without ever having built a recap.
func (f *fastForwardFlow) check(name string) domain.FastForwardCheck {
	if cached, ok := f.checks[name]; ok {
		return cached
	}
	result, err := branch.Check(f.runCtx, branch.BranchParams{ProjectDir: f.ctx.ProjectDir, Branch: name})
	if err != nil {
		result = domain.FastForwardCheck{Branch: name, State: domain.DivergenceUnknown}
	}
	f.checks[name] = result
	return result
}

func (f *fastForwardFlow) conclude(outcome Outcome) (Outcome, error) {
	return outcome, f.presenter.FastForwarded(outcome)
}

// hasFailure excludes the refusals: a diverged branch or one with no counterpart
// is a reported fact, not a run that went wrong.
func hasFailure(results []domain.FastForwardResult) bool {
	for _, result := range results {
		if result.Status == domain.FFFailed {
			return true
		}
	}
	return false
}
