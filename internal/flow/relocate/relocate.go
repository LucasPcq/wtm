// Package relocate runs the `wtm relocate` flow.
package relocate

import (
	"context"
	"errors"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/publish"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Request struct {
	// To is the base_path the surface already settled on. Empty keeps the
	// configured one and leaves changing it to the wizard.
	To string
	// Force is the safety axis: it lifts the dirty/locked refusals, never the
	// occupied destination nor the running jobs.
	Force bool
	// DryRun previews the plan and mutates nothing.
	DryRun     bool
	BaseBranch string
}

type Outcome struct {
	Result domain.RelocateResult
	// Plan is what a dry run computed, for a surface that renders a preview
	// differently from a result.
	Plan   domain.RelocatePlan
	DryRun bool
	// FromBasePath is the base_path in effect before the run.
	FromBasePath string
	// Empty reports that nothing would change: no worktree to move or adopt, and
	// base_path kept.
	Empty   bool
	Aborted bool
}

type Presenter interface {
	flow.Presenter
	Relocated(Outcome) error
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

func Run(ctx context.Context, params Params) (Outcome, error) {
	f := &relocateFlow{
		runCtx:    ctx,
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
	}
	return f.run()
}

type relocateFlow struct {
	runCtx    context.Context
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter

	plan domain.RelocatePlan
}

func (f *relocateFlow) run() (Outcome, error) {
	plan, err := f.planAt(f.targetBasePath())
	if err != nil {
		return Outcome{}, err
	}
	f.plan = plan

	if f.request.DryRun {
		return f.preview()
	}

	answers, err := f.prompter.Ask(f.session())
	if errors.Is(err, domain.ErrUserAborted) {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}
	if err != nil {
		return Outcome{}, err
	}
	if recap, _ := answers.Get(KeyRecap); recap.Skipped {
		return f.conclude(Outcome{Empty: true})
	}

	return f.apply(applyParams{BasePath: f.basePath(answers), Parents: f.parents(answers)})
}

func (f *relocateFlow) preview() (Outcome, error) {
	target := f.targetBasePath()
	if !f.changes(target) {
		return f.conclude(Outcome{Empty: true})
	}
	return f.conclude(Outcome{
		Plan:   f.plan,
		DryRun: true,
		Result: f.execute(executeParams{Plan: f.plan, BasePath: target, DryRun: true}),
	})
}

type applyParams struct {
	BasePath string
	Parents  map[string]string
}

// apply plans again against the filesystem: the wizard may have changed
// base_path, and a worktree may have changed while it was open.
func (f *relocateFlow) apply(params applyParams) (Outcome, error) {
	var result domain.RelocateResult
	err := f.presenter.Stage(f.runCtx, flow.StageParams{
		Message: domain.RelocateStageMessage,
		Work: func(ctx context.Context) error {
			plan, err := f.planAt(params.BasePath)
			if err != nil {
				return err
			}
			result = f.execute(executeParams{Plan: plan, BasePath: params.BasePath, Parents: params.Parents})
			return f.rewriteBasePath(&result)
		},
	})
	if err != nil {
		return Outcome{}, err
	}

	outcome, err := f.conclude(Outcome{Result: result})
	if err != nil {
		return outcome, err
	}
	if rules.RelocateHasFailure(result) {
		return outcome, domain.ErrAborted
	}
	return outcome, nil
}

type executeParams struct {
	Plan     domain.RelocatePlan
	BasePath string
	Parents  map[string]string
	DryRun   bool
}

// execute keeps going past a worktree that fails: each one is reported on its
// own line, and the others are no less safe to move for it.
func (f *relocateFlow) execute(params executeParams) domain.RelocateResult {
	result := domain.RelocateResult{BasePath: params.BasePath, Steps: []domain.RelocateStepResult{}}
	for _, step := range params.Plan.Steps {
		res := rules.RelocateStepStart(rules.RelocateStepStartParams{Step: step, Parents: params.Parents, BaseBranch: f.request.BaseBranch})
		if res.Status == domain.RelocateStatusMove || res.Status == domain.RelocateStatusAdopt {
			res = f.carryOut(carryOutParams{Step: step, Result: res, DryRun: params.DryRun})
		}
		result.Steps = append(result.Steps, res)
	}
	return result
}

type carryOutParams struct {
	Step   domain.RelocateStep
	Result domain.RelocateStepResult
	DryRun bool
}

// carryOut takes a moved-and-adopted worktree through two acts, the move and
// then the adoption, each one observable on its own.
func (f *relocateFlow) carryOut(params carryOutParams) domain.RelocateStepResult {
	res := params.Result
	if params.DryRun {
		res.Status = rules.RelocateStepDone(params.Step)
		return res
	}
	if params.Step.Status == domain.RelocateStatusMove {
		if err := f.move(res); err != nil {
			return rules.RelocateStepFailed(res, err)
		}
	}
	if params.Step.Adopt {
		if err := f.adopt(res); err != nil {
			return rules.RelocateStepFailed(res, err)
		}
	}
	res.Status = rules.RelocateStepDone(params.Step)
	return res
}

func (f *relocateFlow) move(res domain.RelocateStepResult) error {
	if err := worktree.Move(worktree.MoveParams{
		ProjectDir: f.ctx.ProjectDir,
		From:       res.FromPath,
		To:         res.ToPath,
		Force:      f.request.Force,
	}); err != nil {
		return err
	}
	publish.Relocated(publish.RelocatedParams{Context: f.ctx, Branch: res.Branch, FromPath: res.FromPath})
	return nil
}

func (f *relocateFlow) adopt(res domain.RelocateStepResult) error {
	if err := worktree.Adopt(worktree.AdoptParams{
		StateDir: f.ctx.StateDir,
		Branch:   res.Branch,
		Parent:   res.Parent,
	}); err != nil {
		return err
	}
	publish.Updated(publish.UpdatedParams{Context: f.ctx, Branch: res.Branch, Changed: []domain.IdentityField{domain.IdentityParent, domain.IdentityCreatedAt}})
	return nil
}

// rewriteBasePath follows the worktrees, whatever became of them: one held
// back by a refusal is reported, and the next relocate moves it.
func (f *relocateFlow) rewriteBasePath(result *domain.RelocateResult) error {
	if result.BasePath == f.configBasePath() {
		return nil
	}
	if err := worktree.SetBasePath(worktree.SetBasePathParams{
		ProjectDir: f.ctx.ProjectDir,
		StateDir:   f.ctx.StateDir,
		Project:    f.ctx.Config.Project,
		BasePath:   result.BasePath,
	}); err != nil {
		return err
	}
	result.BasePathUpdated = true
	return nil
}

func (f *relocateFlow) conclude(outcome Outcome) (Outcome, error) {
	outcome.FromBasePath = f.configBasePath()
	if outcome.Empty {
		outcome.Result = domain.RelocateResult{BasePath: f.configBasePath(), Steps: []domain.RelocateStepResult{}}
	}
	return outcome, f.presenter.Relocated(outcome)
}

func (f *relocateFlow) planAt(basePath string) (domain.RelocatePlan, error) {
	return worktree.PlanRelocate(worktree.PlanRelocateParams{
		ProjectDir:     f.ctx.ProjectDir,
		StateDir:       f.ctx.StateDir,
		TargetBasePath: basePath,
		BaseBranch:     f.request.BaseBranch,
		Force:          f.request.Force,
	})
}

// changes reports whether a run ending on basePath would do anything.
func (f *relocateFlow) changes(basePath string) bool {
	return rules.PlanHasWork(f.plan) || basePath != f.configBasePath()
}

func (f *relocateFlow) targetBasePath() string {
	if f.request.To != "" {
		return f.request.To
	}
	return f.configBasePath()
}

func (f *relocateFlow) configBasePath() string {
	return f.ctx.Config.Project.Worktrees.BasePath
}
