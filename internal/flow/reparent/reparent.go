// Package reparent runs the `wtm reparent` flow.
package reparent

import (
	"errors"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/decide"
	"github.com/LucasPcq/wtm/internal/flow/publish"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Request struct {
	Branches []string
	To       string
}

type Outcome struct {
	Results []domain.ReparentResult
	Aborted bool
}

type Presenter interface {
	flow.Presenter
	Reparented(Outcome) error
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

// Operation declares how a surface must schedule a reparent: it only rewrites
// metadata, but it asks first, so it holds the surface until it is answered. It
// names no target — a reparent can hold several worktrees at once, so a surface
// starting it on one names that one itself.
func Operation() flow.Operation {
	return flow.Operation{Kind: domain.OpKindReparent, Mode: flow.ModeBlocking}
}

func Run(params Params) (Outcome, error) {
	f := &reparentFlow{
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
	}
	return f.run()
}

type reparentFlow struct {
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter

	nodes      []domain.WorktreeNode
	candidates []domain.BranchCandidate
}

func (f *reparentFlow) run() (Outcome, error) {
	nodes, err := worktree.Nodes(worktree.NodesParams{
		ProjectDir: f.ctx.ProjectDir,
		StateDir:   f.ctx.StateDir,
	})
	if err != nil {
		return Outcome{}, err
	}
	f.nodes = nodes
	f.candidates = decide.BranchCandidates(f.ctx.ProjectDir)

	answers, err := f.prompter.Ask(f.session())
	if errors.Is(err, domain.ErrUserAborted) {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}
	if err != nil {
		return Outcome{}, err
	}

	var results []domain.ReparentResult
	err = f.presenter.Stage(flow.StageParams{
		Message: domain.ReparentStageMessage,
		Work: func() error {
			var batchErr error
			results, batchErr = worktree.ReparentBatch(domain.ReparentBatchParams{
				ProjectDir: f.ctx.ProjectDir,
				StateDir:   f.ctx.StateDir,
				Branches:   answers.Values(KeyBranches),
				NewParent:  answers.Value(KeyParent),
				BaseBranch: f.baseBranch(),
			})
			return batchErr
		},
	})
	publish.ReparentedAll(f.ctx, results)
	if err != nil {
		return Outcome{}, err
	}

	outcome := Outcome{Results: results}
	return outcome, f.presenter.Reparented(outcome)
}

func (f *reparentFlow) baseBranch() string {
	return f.ctx.Config.Project.Worktrees.BaseBranch
}
