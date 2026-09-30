// Package create runs the `wtm create` flow.
package create

import (
	"errors"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/decide"
	"github.com/LucasPcq/wtm/internal/flow/envports"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Request struct {
	Branch      string
	From        string
	EnvFrom     string
	FastForward bool
	IfNotExists bool
	// Isolation is --isolation, empty when it was not given.
	Isolation domain.Isolation
}

type Outcome struct {
	Result     domain.CreateResult
	Branch     string
	FromBranch string
	Aborted    bool
}

type Presenter interface {
	flow.Presenter
	Created(Outcome) error
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

// Operation declares how a surface must schedule a create: the hooks can run long,
// so the run goes to the background and holds the branch it is provisioning.
func Operation() flow.Operation {
	return flow.Operation{Kind: domain.OpKindCreate, Mode: flow.ModeBackground, TargetKey: KeyBranch}
}

func Run(params Params) (Outcome, error) {
	f := &createFlow{
		ctx:        params.Context,
		request:    params.Request,
		prompter:   params.Prompter,
		presenter:  params.Presenter,
		candidates: decide.BranchCandidates(params.Context.ProjectDir),
		target:     decide.MemoizedTarget(params.Context.ProjectDir),
	}
	return f.run()
}

type createFlow struct {
	ctx        flow.Context
	request    Request
	prompter   flow.Prompter
	presenter  Presenter
	candidates []domain.BranchCandidate
	target     func(string) domain.BranchTarget
}

func (f *createFlow) run() (Outcome, error) {
	if f.request.From != "" && !rules.BranchCandidateExists(f.candidates, f.request.From) {
		return Outcome{}, fmt.Errorf("%w: %s", domain.ErrBranchNotFound, f.request.From)
	}

	// Failing here saves the user a full interactive run that could only ever end in
	// refusal; worktree.Create's guard is the chokepoint for the other callers.
	if f.request.Branch != "" {
		if target := f.target(f.request.Branch); target.State == domain.BranchTargetCheckedOut && !f.request.IfNotExists {
			return Outcome{}, fmt.Errorf("%w: "+domain.BranchCheckedOutElsewhereFmt,
				domain.ErrWorktreeExists, f.request.Branch, target.WorktreePath, f.request.Branch)
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

	branchName := answers.Value(KeyBranch)
	fromBranch := answers.Value(KeySource)

	if answers.Value(KeySourceUpdate) == updateFastForward {
		proceed, ffErr := f.applyFastForward(f.sourceUpdate(answers).Branch)
		if ffErr != nil {
			return Outcome{}, ffErr
		}
		if !proceed {
			return Outcome{Aborted: true}, nil
		}
	}

	// A reused branch is checked out as-is: the source is only its recorded sync parent.
	target := f.target(branchName)
	startPoint := fromBranch
	if !rules.SourceIsStartPoint(target.State) {
		startPoint = ""
	}

	preflight := envports.Preflight(f.ctx)

	var result domain.CreateResult
	err = f.presenter.Stage(flow.StageParams{
		Message: fmt.Sprintf(domain.CreateLoadingFmt, branchName),
		Work: func() error {
			var createErr error
			result, createErr = worktree.Create(domain.CreateParams{
				ProjectDir:      f.ctx.ProjectDir,
				StateDir:        f.ctx.StateDir,
				Branch:          branchName,
				FromBranch:      startPoint,
				SourceBranch:    fromBranch,
				Config:          f.ctx.Config,
				EnvFromOverride: answers.Value(KeyEnv),
				IfNotExists:     f.request.IfNotExists,
				SkipHooks:       true,
				Isolation:       f.isolation(answers),
			})
			return createErr
		},
	})
	if err != nil {
		return Outcome{}, err
	}

	if result.AlreadyExists {
		f.warnIgnoredIsolation(&result)
	} else {
		// Before the hooks: one of them may well read the .env this settles.
		result.EnvPorts, result.Warnings = envports.SettleFresh(envports.FreshParams{
			Params: envports.Params{
				Context:      f.ctx,
				Branch:       branchName,
				WorktreePath: result.Path,
				Presenter:    f.presenter,
			},
			Preflight: preflight,
		})
		if hookErr := f.runHooks(result.Path, branchName, fromBranch); hookErr != nil {
			return Outcome{}, hookErr
		}
	}
	result.Isolation = worktree.IsolationOf(worktree.WorktreeRef{ProjectDir: f.ctx.ProjectDir, StateDir: f.ctx.StateDir, Branch: branchName})

	outcome := Outcome{Result: result, Branch: branchName, FromBranch: fromBranch}
	return outcome, f.presenter.Created(outcome)
}

func (f *createFlow) warnIgnoredIsolation(result *domain.CreateResult) {
	warning := rules.IsolationIgnoredWarning(rules.IsolationIgnoredParams{
		Branch:    result.Branch,
		Requested: f.request.Isolation,
		Current:   worktree.IsolationOf(worktree.WorktreeRef{ProjectDir: f.ctx.ProjectDir, StateDir: f.ctx.StateDir, Branch: result.Branch}),
	})
	if warning == "" {
		return
	}
	f.presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: warning})
	result.Warnings = append(result.Warnings, warning)
}

// isolation is the step's answer, else the project's default: a skipped step
// has nothing to isolate, and records what a later run.toml would assume.
func (f *createFlow) isolation(answers flow.Answers) domain.Isolation {
	if value := answers.Value(KeyIsolation); value != "" {
		return domain.Isolation(value)
	}
	return envports.DefaultIsolation(f.ctx)
}

func (f *createFlow) runHooks(worktreePath, branchName, fromBranch string) error {
	hooks := f.ctx.Config.Project.Hooks.OnCreate
	if len(hooks) == 0 {
		return nil
	}
	return f.presenter.HookPhase(flow.HookPhaseParams{
		Title:   domain.HooksTitleOnCreate,
		LogPath: rules.HooksLogPath(rules.HooksLogPathParams{StateDir: f.ctx.StateDir, Phase: domain.HookOnCreate, Branch: branchName}),
		Run: func(sink flow.HookSink) error {
			return worktree.RunCreateHooks(domain.CreateHooksParams{
				ProjectDir:   f.ctx.ProjectDir,
				StateDir:     f.ctx.StateDir,
				WorktreePath: worktreePath,
				Branch:       branchName,
				FromBranch:   fromBranch,
				Hooks:        hooks,
				Output:       sink.Output,
				OnHook:       sink.OnHook,
			})
		},
	})
}

func (f *createFlow) applyFastForward(subject string) (bool, error) {
	params := branch.BranchParams{ProjectDir: f.ctx.ProjectDir, Branch: subject}

	// --ff is best effort: a branch that cannot be cleanly fast-forwarded is left
	// as-is and creation proceeds from it, and no prompt can run.
	if !f.prompter.Interactive() {
		_ = branch.FastForwardIfBehind(params)
		return true, nil
	}

	ffErr := f.presenter.Stage(flow.StageParams{
		Message: fmt.Sprintf(domain.SourceFastForwardLoadingFmt, subject),
		Work:    func() error { return branch.FastForwardToOrigin(params) },
	})
	if ffErr == nil {
		return true, nil
	}

	_, ab := branch.Divergence(params)
	proceed, confirmErr := f.prompter.Confirm(flow.ConfirmParams{
		Title:      fmt.Sprintf(domain.SourceProceedStalePrompt, subject, ab.Behind),
		Warning:    fmt.Sprintf(domain.SourceProceedStaleWarning, ffErr),
		DefaultYes: false,
	})
	if confirmErr != nil {
		return false, nil
	}
	return proceed, nil
}
