// Package envports settles the host ports a freshly provisioned .env holds onto
// the ones the worktree it belongs to actually binds.
package envports

import (
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/rules"
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Params struct {
	Context      flow.Context
	Branch       string
	WorktreePath string
	// Rewrite is the decision, made before the worktree existed: the surfaces ask
	// it as a step of the run that creates it, and resolve it to true when nobody
	// can be asked. False still writes the identity keys — a worktree whose
	// COMPOSE_PROJECT_NAME is another's collides whatever its ports say.
	Rewrite   bool
	Presenter flow.Presenter
}

// Linked says whether this project links any .env value to a port, which is what
// makes the question worth asking at all. A surface reads it to skip its step —
// before the worktree exists, so it cannot be derived from the plan.
func Linked(ctx flow.Context) bool {
	cfg, err := runconfig.Load(ctx.StateDir)
	if err != nil {
		return false
	}
	return len(cfg.EnvPorts) > 0
}

// Settle moves the host ports a freshly provisioned .env holds onto the ones
// this worktree binds. The values were just copied from main or from a parent,
// so they carry that worktree's ports and nothing else would fix them.
//
// It never asks: the question belongs to the run that creates the worktree,
// where it is one confirmation among the others rather than a second one, put
// after the point of no return. What is left here is a report of what happened.
func Settle(params Params) (domain.EnvPortSettlement, error) {
	resolved, err := worktree.ResolveEnvPorts(worktree.ResolveEnvPortsParams{
		ProjectDir:   params.Context.ProjectDir,
		StateDir:     params.Context.StateDir,
		Branch:       params.Branch,
		WorktreePath: params.WorktreePath,
		EnvFiles:     params.Context.Config.Project.Env.Files,
		Global:       params.Context.Config.Global,
	})
	if err != nil || resolved.Empty() {
		return domain.EnvPortSettlement{}, err
	}

	plan, err := envsvc.ComputeEnvPorts(resolved)
	if err != nil {
		return domain.EnvPortSettlement{}, err
	}
	if anomalies := rules.EnvPortAnomalyLines(plan); len(anomalies) > 0 {
		params.Presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: domain.EnvPortAnomaliesTitle, Lines: anomalies})
	}
	for _, notice := range rules.EnvPortNotices(plan) {
		params.Presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: notice.Title, Lines: []string{notice.Line}})
	}

	settlement := domain.EnvPortSettlement{Shifted: len(rules.EnvPortRewrites(plan)), Offset: plan.Offset}
	if !params.Rewrite || settlement.Shifted == 0 {
		return settlement, envsvc.ApplyOwnedEnv(resolved)
	}

	settlement.Applied = true
	_, err = envsvc.ApplyEnvPorts(resolved)
	return settlement, err
}
