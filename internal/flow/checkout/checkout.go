// Package checkout runs the `wtm checkout` flow: a worktree from a pull request.
package checkout

import (
	"errors"
	"fmt"
	"sync"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/decide"
	"github.com/LucasPcq/wtm/internal/flow/envports"
	"github.com/LucasPcq/wtm/internal/flow/ordinal"
	"github.com/LucasPcq/wtm/internal/flow/publish"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// Request.Number is zero when the pull request is to be picked among the open ones.
type Request struct {
	Number      int
	Filter      domain.PRFilter
	From        string
	EnvFrom     string
	FastForward bool
	Isolation   domain.Isolation
}

// Outcome.Target is the PR branch's state once checked out, so a reused branch
// is reported as it is after an accepted fast-forward.
type Outcome struct {
	PR      domain.PRInfo
	Result  domain.CreateResult
	Target  domain.BranchTarget
	Aborted bool
}

type Presenter interface {
	flow.Presenter
	CheckedOut(Outcome) error
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

func Run(params Params) (Outcome, error) {
	f := &checkoutFlow{
		ctx:        params.Context,
		request:    params.Request,
		prompter:   params.Prompter,
		presenter:  params.Presenter,
		candidates: decide.BranchCandidates(params.Context.ProjectDir),
		target:     decide.MemoizedTarget(params.Context.ProjectDir),
		applies:    envports.IsolationApplies(params.Context),
	}
	return f.run()
}

type checkoutFlow struct {
	ctx        flow.Context
	request    Request
	prompter   flow.Prompter
	presenter  Presenter
	candidates []domain.BranchCandidate
	target     func(string) domain.BranchTarget
	applies    bool
	fetched    bool

	// prs is written by the picker's load, off the surface's own goroutine.
	mu  sync.Mutex
	prs []domain.PRInfo
}

func (f *checkoutFlow) run() (Outcome, error) {
	if f.request.From != "" && !rules.BranchCandidateExists(f.candidates, f.request.From) {
		return Outcome{}, fmt.Errorf("%w: %s", domain.ErrBranchNotFound, f.request.From)
	}
	if f.request.Number > 0 {
		pr, err := f.fetchPR()
		if err != nil {
			return Outcome{}, err
		}
		if err := rules.ValidatePRForCheckout(pr); err != nil {
			return Outcome{}, err
		}
		f.setPRs([]domain.PRInfo{pr})
		// Before the questions: whether the branch is behind origin decides one of
		// them, and --ff answers it from what origin holds now, not from the last fetch.
		if err := f.fetchBranch(pr); err != nil {
			return Outcome{}, err
		}
		// worktree.Create refuses the same, but only once every question is answered.
		if err := f.acceptBranch(pr.Branch); err != nil {
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

	pr, found := f.pr(answers)
	if !found {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}
	if err := rules.ValidatePRForCheckout(pr); err != nil {
		return Outcome{}, err
	}

	if answers.Value(KeySourceUpdate) == decide.UpdateFastForward {
		proceed := decide.ApplyFastForward(decide.ApplyFastForwardParams{
			ProjectDir: f.ctx.ProjectDir,
			Subject:    pr.Branch,
			Prompter:   f.prompter,
			Presenter:  f.presenter,
		})
		if !proceed {
			f.presenter.Notice(flow.AbortedNotice)
			return Outcome{Aborted: true}, nil
		}
	}
	return f.checkout(checkoutParams{PR: pr, Answers: answers})
}

func (f *checkoutFlow) fetchPR() (domain.PRInfo, error) {
	var pr domain.PRInfo
	err := f.presenter.Stage(flow.StageParams{
		Message: domain.CheckoutFetchingPR,
		Work: func() error {
			var fetchErr error
			pr, fetchErr = ghservice.GetPRDetail(ghservice.GetPRDetailParams{ProjectDir: f.ctx.ProjectDir, Number: f.request.Number})
			return fetchErr
		},
	})
	if err != nil {
		return domain.PRInfo{}, fmt.Errorf("fetch PR: %w", err)
	}
	return pr, nil
}

type checkoutParams struct {
	PR      domain.PRInfo
	Answers flow.Answers
}

func (f *checkoutFlow) acceptBranch(name string) error {
	if target := f.target(name); target.State == domain.BranchTargetCheckedOut {
		return fmt.Errorf("%w: "+domain.BranchCheckedOutElsewhereFmt, domain.ErrWorktreeExists, name, target.WorktreePath, name)
	}
	return worktree.CheckNameFree(worktree.NameCheckParams{ProjectDir: f.ctx.ProjectDir, StateDir: f.ctx.StateDir, Branch: name})
}

func (f *checkoutFlow) fetchBranch(pr domain.PRInfo) error {
	if f.fetched {
		return nil
	}
	err := f.presenter.Stage(flow.StageParams{
		Message: domain.CheckoutFetchingBranch,
		Work: func() error {
			return branch.FetchFromOrigin(branch.BranchParams{ProjectDir: f.ctx.ProjectDir, Branch: pr.Branch})
		},
	})
	f.fetched = err == nil
	return err
}

func (f *checkoutFlow) checkout(params checkoutParams) (Outcome, error) {
	pr := params.PR
	branchParams := branch.BranchParams{ProjectDir: f.ctx.ProjectDir, Branch: pr.Branch}
	if err := f.fetchBranch(pr); err != nil {
		return Outcome{}, err
	}

	// A local branch of the PR's name is checked out as-is, keeping commits never
	// pushed; only one another worktree holds is refused, by worktree.Create. It is
	// read again here because the fetch just moved what it is compared with.
	target := branch.Target(branchParams)
	startPoint := domain.RemoteBranchPrefix + pr.Branch
	if target.State == domain.BranchTargetExisting {
		startPoint = ""
	}

	parent := params.Answers.Value(KeyParent)
	preflight := envports.Preflight(f.ctx)
	result, err := f.create(createParams{PR: pr, StartPoint: startPoint, Parent: parent, Answers: params.Answers})
	if err != nil {
		return Outcome{}, err
	}
	publish.Created(f.ctx, result.Branch)

	// Before the hooks: one of them may read the .env, and it has to read what
	// this worktree binds rather than what it was copied with.
	result.EnvPorts, result.Warnings = envports.SettleFresh(envports.FreshParams{
		Params: envports.Params{
			Context:      f.ctx,
			Branch:       result.Branch,
			WorktreePath: result.Path,
			Presenter:    f.presenter,
		},
		Preflight: preflight,
	})
	result.Warnings = append(result.Warnings, decide.WarnUnseenFallback(decide.UnseenFallbackParams{
		Fallback:  decide.EnvFallbackParams{ProjectDir: f.ctx.ProjectDir, Source: parent, Config: f.ctx.Config, EnvOverride: params.Answers.Value(KeyEnv)},
		Prompter:  f.prompter,
		Presenter: f.presenter,
	})...)

	// A reused branch has no start-point, so the hooks see its recorded parent.
	if err := f.runHooks(hooksParams{WorktreePath: result.Path, Branch: pr.Branch, FromBranch: rules.FirstNonEmpty(startPoint, parent)}); err != nil {
		return Outcome{}, err
	}
	result.Isolation = worktree.IsolationOf(worktree.WorktreeRef{ProjectDir: f.ctx.ProjectDir, StateDir: f.ctx.StateDir, Branch: result.Branch})

	outcome := Outcome{PR: pr, Result: result, Target: target}
	return outcome, f.presenter.CheckedOut(outcome)
}

type createParams struct {
	PR         domain.PRInfo
	StartPoint string
	Parent     string
	Answers    flow.Answers
}

// create is the one point where the worktree comes into existence: a checkout
// never reuses a worktree, so whatever it returns without an error is new, and
// is published before its hooks run.
func (f *checkoutFlow) create(params createParams) (domain.CreateResult, error) {
	var result domain.CreateResult
	err := f.presenter.Stage(flow.StageParams{
		Message: fmt.Sprintf(domain.CreateLoadingFmt, params.PR.Branch),
		Work: func() error {
			var createErr error
			result, createErr = worktree.Create(domain.CreateParams{
				ProjectDir:      f.ctx.ProjectDir,
				StateDir:        f.ctx.StateDir,
				Branch:          params.PR.Branch,
				FromBranch:      params.StartPoint,
				SourceBranch:    params.Parent,
				Config:          f.ctx.Config,
				EnvFromOverride: params.Answers.Value(KeyEnv),
				SkipHooks:       true,
				Isolation:       f.isolation(params.Answers),
			})
			return createErr
		},
	})
	return result, err
}

type hooksParams struct {
	WorktreePath string
	Branch       string
	FromBranch   string
}

func (f *checkoutFlow) runHooks(params hooksParams) error {
	hooks := f.ctx.Config.Project.Hooks.OnCreate
	if len(hooks) == 0 {
		return nil
	}
	ordinal.BeforeHooks(f.ctx, params.Branch)
	return f.presenter.HookPhase(flow.HookPhaseParams{
		Title:   domain.HooksTitleOnCreate,
		LogPath: rules.HooksLogPath(rules.HooksLogPathParams{StateDir: f.ctx.StateDir, Phase: domain.HookOnCreate, Branch: params.Branch}),
		Run: func(sink flow.HookSink) error {
			return worktree.RunCreateHooks(domain.CreateHooksParams{
				ProjectDir:   f.ctx.ProjectDir,
				StateDir:     f.ctx.StateDir,
				WorktreePath: params.WorktreePath,
				Branch:       params.Branch,
				FromBranch:   params.FromBranch,
				Hooks:        hooks,
				Output:       sink.Output,
				OnHook:       sink.OnHook,
			})
		},
	})
}

func (f *checkoutFlow) isolation(answers flow.Answers) domain.Isolation {
	if value := answers.Value(KeyIsolation); value != "" {
		return domain.Isolation(value)
	}
	return envports.DefaultIsolation(f.ctx)
}

func (f *checkoutFlow) setPRs(prs []domain.PRInfo) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.prs = prs
}

func (f *checkoutFlow) findPR(number int) (domain.PRInfo, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, pr := range f.prs {
		if pr.Number == number {
			return pr, true
		}
	}
	return domain.PRInfo{}, false
}
