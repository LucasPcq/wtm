// Package envports settles the host ports a freshly provisioned .env holds onto
// the ones the worktree it belongs to actually binds.
package envports

import (
	"context"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/ordinal"
	"github.com/LucasPcq/wtm/internal/rules"
	envsvc "github.com/LucasPcq/wtm/internal/service/env"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Params struct {
	Context      flow.Context
	Branch       string
	WorktreePath string
	Presenter    flow.Presenter
}

// IsolationApplies says whether run.toml declares anything a worktree could
// isolate, which is what makes the question worth asking at all. A surface
// reads it before the worktree exists, so it cannot be derived from the plan.
func IsolationApplies(ctx flow.Context) bool {
	cfg, err := runconfig.Load(ctx.StateDir)
	if err != nil {
		return false
	}
	return rules.IsolationApplies(cfg)
}

// DefaultIsolation is what a new worktree gets when nobody is asked: run.toml's
// answer, else isolated. An unreadable run.toml is refused by whatever reads it
// next; here it falls back to what every worktree got before the choice.
func DefaultIsolation(ctx flow.Context) domain.Isolation {
	cfg, err := runconfig.Load(ctx.StateDir)
	if err != nil {
		return domain.IsolationIsolated
	}
	return rules.EffectiveIsolation(cfg.Isolation)
}

// IsolationOrigin is what DefaultIsolation answers from: run.toml when it says,
// else the built-in default.
func IsolationOrigin(ctx flow.Context) domain.AnswerOrigin {
	cfg, err := runconfig.Load(ctx.StateDir)
	if err != nil || cfg.Isolation == "" {
		return domain.AnswerOriginDefault
	}
	return domain.AnswerOriginConfig
}

// Settle moves the host ports a freshly provisioned .env holds onto the ones
// this worktree binds. The values were just copied from main or from a parent,
// so they carry that worktree's ports and nothing else would fix them. A
// verbatim worktree resolves to nothing to settle, and is left as copied.
//
// It never asks: the question belongs to the run that creates the worktree,
// where it is one confirmation among the others rather than a second one, put
// after the point of no return. What is left here is a report of what happened.
func Settle(ctx context.Context, params Params) (domain.EnvPortPlan, error) {
	ignored, err := runconfig.Check(runconfig.CheckParams{StateDir: params.Context.StateDir, EnvFiles: params.Context.Config.Project.Env.Files})
	if err != nil {
		return domain.EnvPortPlan{}, err
	}
	reportIgnored(params.Presenter, ignored)
	return settle(ctx, settleParams{Params: params, Notices: rules.EnvPortNotices})
}

type settleParams struct {
	Params
	Notices func(domain.EnvPortPlan) []rules.EnvPortNotice
}

func settle(ctx context.Context, params settleParams) (domain.EnvPortPlan, error) {
	var resolved envsvc.EnvPortsParams
	err := ordinal.Retry(ctx, ordinal.RetryParams{
		Context: params.Context,
		Branch:  func() string { return params.Branch },
		Do: func() error {
			ports, resolveErr := worktree.ResolveEnvPorts(ctx, worktree.ResolveEnvPortsParams{
				ProjectDir:   params.Context.ProjectDir,
				StateDir:     params.Context.StateDir,
				Branch:       params.Branch,
				WorktreePath: params.WorktreePath,
				EnvFiles:     params.Context.Config.Project.Env.Files,
				Global:       params.Context.Config.Global,
			})
			resolved = ports
			return resolveErr
		},
	})
	if err != nil || resolved.Empty() {
		return domain.EnvPortPlan{}, err
	}

	plan, err := envsvc.ComputeEnvPorts(resolved)
	if err != nil {
		return domain.EnvPortPlan{}, err
	}
	if anomalies := rules.EnvPortAnomalyLines(plan); len(anomalies) > 0 {
		params.Presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: domain.EnvPortAnomaliesTitle, Lines: anomalies})
	}
	for _, notice := range params.Notices(plan) {
		params.Presenter.Status(flow.Notice{Kind: flow.NoticeNote, Text: notice.Title, Lines: []string{notice.Line}})
	}

	if len(rules.EnvPortRewrites(plan)) == 0 {
		err = envsvc.ApplyOwnedEnv(resolved)
	} else {
		_, err = envsvc.ApplyEnvPorts(resolved)
	}
	if err != nil {
		return domain.EnvPortPlan{}, err
	}
	plan.Applied = true
	return plan, nil
}

// RunCheck is run.toml read before the worktree exists: Err refuses the
// whole run part, Ignored names the links the pass goes ahead without.
type RunCheck struct {
	Err     error
	Ignored []string
}

// Preflight reads run.toml before the worktree exists so the creation can go
// ahead without the run part.
func Preflight(ctx flow.Context) RunCheck {
	ignored, err := runconfig.Check(runconfig.CheckParams{StateDir: ctx.StateDir, EnvFiles: ctx.Config.Project.Env.Files})
	return RunCheck{Err: err, Ignored: ignored}
}

type FreshParams struct {
	Params
	Preflight RunCheck
}

// SettleFresh is Settle for a worktree a core command has just created, which
// the run module must never fail: whatever stands in the way is a warning,
// returned for the command's JSON, and the .env stays as it was copied.
func SettleFresh(ctx context.Context, params FreshParams) (domain.EnvPortPlan, []string) {
	if params.Preflight.Err != nil {
		return domain.EnvPortPlan{}, notSettled(notSettledParams{Params: params.Params, Cause: params.Preflight.Err, RunConfig: true})
	}
	warnings := reportIgnored(params.Presenter, params.Preflight.Ignored)
	plan, err := settle(ctx, settleParams{Params: params.Params, Notices: rules.EnvPortNoticesOnCreate})
	if err != nil {
		return domain.EnvPortPlan{}, append(warnings, notSettled(notSettledParams{Params: params.Params, Cause: err})...)
	}
	return plan, warnings
}

func reportIgnored(presenter flow.Presenter, ignored []string) []string {
	for _, line := range ignored {
		presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: line})
	}
	return ignored
}

type notSettledParams struct {
	Params
	Cause     error
	RunConfig bool
}

func notSettled(params notSettledParams) []string {
	warning := rules.PortsNotSettledWarning(rules.PortsNotSettledWarningParams{
		Cause:                 params.Cause.Error(),
		PortsNotSettledParams: rules.PortsNotSettledParams{Branch: params.Branch, RunConfig: params.RunConfig},
	})
	params.Presenter.Status(flow.Notice{Kind: flow.NoticeWarning, Text: warning})
	return []string{warning}
}
