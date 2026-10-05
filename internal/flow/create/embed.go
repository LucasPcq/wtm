package create

import (
	"context"
	"errors"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/decide"
	"github.com/LucasPcq/wtm/internal/flow/envports"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

// EmbedParams describes create's questions as another flow asks them, for the
// one worktree its own run creates — `wtm extract` creating its target.
type EmbedParams struct {
	Context flow.Context
	// Applies gates every step: they are part of the host's session whatever it
	// answers, and stand aside while the host is not creating anything.
	Applies func(flow.Answers) bool
	Branch  string
	// BranchFlag is what an unattended run passes to name the branch.
	BranchFlag string
	From       string
	// Parent is offered first and answers the source unattended, in place of the
	// configured base branch, which stands in when it names none.
	Parent      func(flow.Answers) string
	FastForward bool
	Isolation   domain.Isolation
	// Target and SourceUpdate replace the git lookups, for a test.
	Target       func(string) domain.BranchTarget
	SourceUpdate func(flow.Answers) decide.SourceUpdatePrompt
}

type Embedded struct {
	flow    *createFlow
	applies func(flow.Answers) bool
}

func Embed(ctx context.Context, params EmbedParams) Embedded {
	f := &createFlow{
		runCtx:       ctx,
		ctx:          params.Context,
		request:      Request{From: params.From, FastForward: params.FastForward, Isolation: params.Isolation},
		candidates:   decide.BranchCandidates(ctx, params.Context.ProjectDir),
		target:       params.Target,
		derivedNames: worktree.DerivedNamesMatter(params.Context.StateDir),
		parent:       params.Parent,
		update:       params.SourceUpdate,
		branchFlag:   params.BranchFlag,
	}
	if params.Branch != "" {
		f.request.Branches = []string{params.Branch}
	}
	if f.target == nil {
		f.target = decide.MemoizedTarget(ctx, params.Context.ProjectDir)
	}
	return Embedded{flow: f, applies: params.Applies}
}

// CheckBranch refuses the branch the flags named, as the step would have had
// anyone been asked: a preset is never validated by its step.
func (e Embedded) CheckBranch() error {
	for _, name := range e.flow.request.Branches {
		if err := e.flow.validateEntry(flow.EntryCheck{Entry: name}); err != nil {
			return err
		}
	}
	return nil
}

// Presets are the answers the flags already gave, for the host's session.
func (e Embedded) Presets() map[string]string { return e.flow.presets() }

func (e Embedded) Steps() []flow.Step {
	steps := []flow.Step{e.flow.singleBranchStep(), e.flow.sourceStep(), e.flow.isolationStep(), e.flow.sourceUpdateStep()}
	for i := range steps {
		steps[i] = e.gate(steps[i])
	}
	return steps
}

func (e Embedded) gate(step flow.Step) flow.Step {
	inner := step.Skip
	step.Skip = func(answers flow.Answers) (bool, string) {
		if !e.applies(answers) {
			return true, ""
		}
		if inner == nil {
			return false, ""
		}
		return inner(answers)
	}
	return step
}

func (f *createFlow) singleBranchStep() flow.Step {
	return flow.Step{
		Kind:        flow.StepText,
		Key:         KeyBranch,
		Label:       domain.CreateBranchLabel,
		Title:       domain.CreateBranchLabel,
		Description: domain.CreateBranchStepDescription,
		Validate: func(value string) error {
			if value == "" {
				return errors.New(domain.CreateBranchRequired)
			}
			return f.validateEntry(flow.EntryCheck{Entry: value})
		},
		Flag: f.branchFlag,
	}
}

// Plan is what the host's recap reads of the worktree about to be created.
type Plan struct {
	Branch      string
	From        string
	Reused      bool
	FastForward string
	Isolation   domain.Isolation
	Warnings    []string
	// The marks say where an answer stands in the repository's memory;
	// KeptSource names a source a remembered "keep" leaves behind.
	IsolationMark string
	UpdateMark    string
	KeptSource    []string
}

func (e Embedded) Plan(answers flow.Answers) Plan {
	f := e.flow
	plan := Plan{
		Branch:        answers.Value(KeyBranch),
		From:          answers.Value(KeySource),
		Reused:        f.reusesBranch(answers),
		Isolation:     domain.Isolation(answers.Value(KeyIsolation)),
		Warnings:      f.warnings(answers),
		IsolationMark: flow.RememberedMark(answers, KeyIsolation),
		UpdateMark:    flow.RememberedMark(answers, KeySourceUpdate),
		KeptSource:    decide.KeptSourceLines(answers, KeySourceUpdate),
	}
	if answers.Value(KeySourceUpdate) == updateFastForward {
		plan.FastForward = f.sourceUpdate(answers).Branch
	}
	return plan
}

type ProvisionParams struct {
	Answers   flow.Answers
	Prompter  flow.Prompter
	Presenter flow.Presenter
}

// Provision creates the worktree the answers describe, exactly as `wtm create`
// does for one branch: the accepted fast-forward, the worktree, its ports, its
// hooks. proceed is false when a failed fast-forward made the user back out.
func (e Embedded) Provision(params ProvisionParams) (result domain.CreateResult, proceed bool, err error) {
	f := *e.flow
	f.prompter = params.Prompter
	f.presenter = hostPresenter{Presenter: params.Presenter}
	answers := params.Answers

	if answers.Value(KeySourceUpdate) == updateFastForward {
		ok, ffErr := f.applyFastForward(fastForwardParams{Subject: f.sourceUpdate(answers).Branch})
		if ffErr != nil || !ok {
			return domain.CreateResult{}, false, ffErr
		}
	}
	result, err = f.provisionOne(provisionParams{
		Branch:    answers.Value(KeyBranch),
		Source:    answers.Value(KeySource),
		Answers:   answers,
		Preflight: envports.Preflight(f.ctx),
	})
	result.Origins = f.origins(answers)
	return result, true, err
}

// hostPresenter is a host's presenter where create's own is expected: a run that
// creates one worktree never reports a batch, and the conclusion is the host's.
type hostPresenter struct {
	flow.Presenter
}

func (hostPresenter) BranchStarted(flow.Progress)       {}
func (hostPresenter) BranchCreated(domain.CreateResult) {}
func (hostPresenter) BranchFailed(domain.BatchFailure)  {}
func (hostPresenter) Created(Outcome) error             { return nil }
