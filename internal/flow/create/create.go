// Package create runs the `wtm create` flow.
package create

import (
	"context"
	"errors"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/decide"
	"github.com/LucasPcq/wtm/internal/flow/envports"
	"github.com/LucasPcq/wtm/internal/flow/ordinal"
	"github.com/LucasPcq/wtm/internal/flow/publish"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/branch"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Request struct {
	Branches    []string
	From        string
	EnvFrom     string
	FastForward bool
	IfNotExists bool
	// Isolation is --isolation, empty when it was not given.
	Isolation domain.Isolation
}

type Outcome struct {
	Results    []domain.CreateResult
	Failed     []domain.BatchFailure
	FromBranch string
	Aborted    bool
}

// Presenter hears about each branch only when the run holds several: a single
// one reads exactly as it always did.
type Presenter interface {
	flow.Presenter
	BranchStarted(flow.Progress)
	BranchCreated(domain.CreateResult)
	BranchFailed(domain.BatchFailure)
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

func Run(ctx context.Context, params Params) (Outcome, error) {
	f := &createFlow{
		runCtx:       ctx,
		ctx:          params.Context,
		request:      params.Request,
		prompter:     params.Prompter,
		presenter:    params.Presenter,
		candidates:   decide.BranchCandidates(params.Context.ProjectDir),
		target:       decide.MemoizedTarget(params.Context.ProjectDir),
		derivedNames: worktree.DerivedNamesMatter(params.Context.StateDir),
	}
	return f.run()
}

type createFlow struct {
	runCtx       context.Context
	ctx          flow.Context
	request      Request
	prompter     flow.Prompter
	presenter    Presenter
	candidates   []domain.BranchCandidate
	target       func(string) domain.BranchTarget
	derivedNames bool
	// parent, update and branchFlag are set only for a host embedding these
	// steps (Embed): the parent it offers, and its test's divergence.
	parent     func(flow.Answers) string
	update     func(flow.Answers) decide.SourceUpdatePrompt
	branchFlag string
}

func (f *createFlow) run() (Outcome, error) {
	if err := f.refuseOwnParent(); err != nil {
		return Outcome{}, err
	}
	if f.request.From != "" && !rules.BranchCandidateExists(f.candidates, f.request.From) {
		return Outcome{}, fmt.Errorf("%w: %s", domain.ErrBranchNotFound, f.request.From)
	}

	// Failing here saves the user a full interactive run that could only ever end in
	// refusal; worktree.Create's guard is the chokepoint for the other callers.
	requested, err := f.acceptRequested()
	if err != nil {
		return Outcome{}, err
	}
	f.request.Branches = requested

	answers, err := f.prompter.Ask(f.session())
	if errors.Is(err, domain.ErrUserAborted) {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}
	if err != nil {
		return Outcome{}, err
	}

	branches := f.branches(answers)
	fromBranch := answers.Value(KeySource)

	if answers.Value(KeySourceUpdate) == updateFastForward {
		proceed, ffErr := f.applyFastForward(fastForwardParams{Subject: f.sourceUpdate(answers).Branch, Many: f.many(answers)})
		if ffErr != nil {
			return Outcome{}, ffErr
		}
		if !proceed {
			f.presenter.Notice(flow.AbortedNotice)
			return Outcome{Aborted: true}, nil
		}
	}

	preflight := envports.Preflight(f.ctx)

	outcome := Outcome{FromBranch: fromBranch}
	batch := len(branches) > 1
	var firstErr error
	for i, name := range branches {
		if batch {
			f.presenter.BranchStarted(flow.Progress{Branch: name, Position: i + 1, Total: len(branches)})
		}
		result, err := f.provisionOne(provisionParams{Branch: name, Source: fromBranch, Answers: answers, Preflight: preflight, Batch: batch})
		if err == nil {
			outcome.Results = append(outcome.Results, result)
			if batch {
				f.presenter.BranchCreated(result)
			}
			continue
		}
		failure := rules.BatchFailureOf(rules.BatchFailureOfParams{Branch: name, Path: result.Path, Err: err})
		outcome.Failed = append(outcome.Failed, failure)
		if firstErr == nil {
			firstErr = err
		}
		if batch {
			f.presenter.BranchFailed(failure)
		}
	}
	if err := f.presenter.Created(outcome); err != nil {
		return outcome, err
	}
	return outcome, flow.BatchError(flow.BatchErrorParams{First: firstErr, Batch: batch})
}

// A source already up to date skips the source-update step, which must not
// swallow --ff for the existing branches of a list.
func (f *createFlow) fastForwardsEach(answers flow.Answers) bool {
	answer, _ := answers.Get(KeySourceUpdate)
	if answer.Skipped {
		return f.request.FastForward
	}
	return answer.Value == updateFastForward
}

func (f *createFlow) refuseOwnParent() error {
	for _, name := range f.request.Branches {
		if name == f.request.From {
			return fmt.Errorf(domain.BranchOwnParentFmt, name, domain.FlagFrom)
		}
	}
	return nil
}

func (f *createFlow) acceptRequested() ([]string, error) {
	names, err := rules.DistinctNames(rules.DistinctNamesParams{Names: f.request.Branches, Blank: domain.CreateBranchRequired})
	if err != nil {
		return nil, err
	}
	for index, name := range names {
		if err := f.validateEntry(flow.EntryCheck{Entry: name, Entries: names[:index]}); err != nil {
			return nil, err
		}
	}
	return names, nil
}

type provisionParams struct {
	Branch    string
	Source    string
	Answers   flow.Answers
	Preflight envports.RunCheck
	Batch     bool
}

func (f *createFlow) provisionOne(params provisionParams) (domain.CreateResult, error) {
	branchName, fromBranch, answers := params.Branch, params.Source, params.Answers

	// A reused branch is checked out as-is: the source is only its recorded sync parent.
	target := f.target(branchName)
	if params.Batch && target.State == domain.BranchTargetExisting && f.fastForwardsEach(answers) {
		_ = branch.FastForwardIfBehind(branch.BranchParams{ProjectDir: f.ctx.ProjectDir, Branch: branchName})
	}
	startPoint := fromBranch
	if !rules.SourceIsStartPoint(target.State) {
		startPoint = ""
	}

	var result domain.CreateResult
	err := f.presenter.Stage(f.runCtx, flow.StageParams{
		Message: fmt.Sprintf(domain.CreateLoadingFmt, branchName),
		Work: func(ctx context.Context) error {
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
		return result, err
	}

	if result.AlreadyExists {
		f.warnIgnoredIsolation(&result)
	} else {
		publish.Created(f.ctx, branchName)
		// Before the hooks: one of them may well read the .env this settles.
		result.EnvPorts, result.Warnings = envports.SettleFresh(envports.FreshParams{
			Params: envports.Params{
				Context:      f.ctx,
				Branch:       branchName,
				WorktreePath: result.Path,
				Presenter:    f.presenter,
			},
			Preflight: params.Preflight,
		})
		result.Warnings = append(result.Warnings, decide.WarnUnseenFallback(decide.UnseenFallbackParams{
			Fallback:  decide.EnvFallbackParams{ProjectDir: f.ctx.ProjectDir, Source: fromBranch, Config: f.ctx.Config, EnvOverride: answers.Value(KeyEnv)},
			Prompter:  f.prompter,
			Presenter: f.presenter,
		})...)
		hookErr := f.runHooks(result.Path, branchName, fromBranch)
		publish.Provisioned(publish.ProvisionedParams{Context: f.ctx, Branch: branchName, Err: hookErr})
		if hookErr != nil {
			return result, hookErr
		}
	}
	result.Isolation = worktree.IsolationOf(worktree.WorktreeRef{ProjectDir: f.ctx.ProjectDir, StateDir: f.ctx.StateDir, Branch: branchName})
	return result, nil
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
	ordinal.BeforeHooks(f.ctx, branchName)
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

type fastForwardParams struct {
	Subject string
	Many    bool
}

func (f *createFlow) applyFastForward(ff fastForwardParams) (bool, error) {
	return decide.ApplyFastForward(f.runCtx, decide.ApplyFastForwardParams{
		ProjectDir: f.ctx.ProjectDir,
		Subject:    ff.Subject,
		Many:       ff.Many,
		Prompter:   f.prompter,
		Presenter:  f.presenter,
	}), nil
}
