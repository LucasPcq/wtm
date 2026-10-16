package create

import (
	"context"
	"errors"
	"slices"

	"github.com/LucasPcq/wtm/internal/command/provision"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra/git"
	"github.com/LucasPcq/wtm/internal/kernel"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/identity"
)

const PhaseFastForward = "fast_forward"

type input = kernel.ApplyInput[Request, Facts, Plan, provision.Detail]

func applier(ports Ports) func(context.Context, input) (kernel.Outcome[provision.Detail], error) {
	return func(ctx context.Context, in input) (kernel.Outcome[provision.Detail], error) {
		run := applyRun{ports: ports, in: in}
		advanced := run.fastForward(ctx)
		items := run.provision(ctx)
		return kernel.Outcome[provision.Detail]{Items: items, Warnings: append(advanced, run.remember(ctx, items)...)}, nil
	}
}

type applyRun struct {
	ports Ports
	in    input
}

func (r applyRun) fastForward(ctx context.Context) []kernel.Warning {
	var warnings []kernel.Warning
	for _, subject := range r.in.Plan.Detail.FastForward {
		if ctx.Err() != nil {
			return warnings
		}
		finished := kernel.Report(r.in.Emit, subject).Phase(PhaseFastForward)
		warning, failed := r.advance(ctx, subject)
		finished()
		if failed && ctx.Err() == nil {
			warnings = append(warnings, warning)
		}
	}
	return warnings
}

func (r applyRun) advance(ctx context.Context, subject string) (kernel.Warning, bool) {
	at := r.in.Facts.Identity.CheckedOutAt(subject)
	if at == "" {
		_, err := r.ports.Git.FastForwardRef(ctx, subject)
		return failedFastForward(subject, err)
	}
	if err := r.ports.Git.FetchRef(ctx, subject); err != nil {
		return failedFastForward(subject, err)
	}
	dirty, err := r.ports.Git.Dirty(ctx, at)
	if err != nil {
		return failedFastForward(subject, err)
	}
	if dirty {
		return kernel.Warning{Code: CodeFastForwardDirty, Params: kernel.Params{ParamBranch: subject}}, true
	}
	_, mergeErr := r.ports.Git.MergeFFOnly(ctx, git.MergeFFOnlyParams{Worktree: at, Onto: git.RemoteBranch{Name: subject}})
	return failedFastForward(subject, mergeErr)
}

func failedFastForward(subject string, err error) (kernel.Warning, bool) {
	if err == nil {
		return kernel.Warning{}, false
	}
	return kernel.Warning{Code: CodeFastForwardFailed, Params: kernel.Params{ParamBranch: subject, ParamCause: err.Error()}}, true
}

func (r applyRun) provision(ctx context.Context) []kernel.Item[provision.Detail] {
	items := make([]kernel.Item[provision.Detail], 0, len(r.in.Plan.Detail.Branches))
	for _, planned := range r.in.Plan.Detail.Branches {
		if planned.Action == ActionUnchanged {
			items = append(items, kernel.ItemFor[provision.Detail](planned.Branch).Unchanged(r.existing(planned)))
			continue
		}
		items = append(items, kernel.Each(ctx, kernel.EachParams[Planned, provision.Detail]{
			Items:      []Planned{planned},
			Subject:    func(p Planned) string { return p.Branch },
			Unit:       func(p Planned) kernel.Unit[provision.Detail] { return provision.Unit(r.unitParams(p)) },
			Emit:       r.in.Emit,
			Shield:     r.in.Shield,
			Milestones: r.in.Milestones,
		})...)
	}
	return items
}

func (r applyRun) existing(planned Planned) provision.Detail {
	recorded, ok := r.in.Facts.Identity.Records[planned.Branch].(identity.Recorded)
	if !ok {
		return provision.Detail{Path: planned.Path}
	}
	return provision.Detail{Path: planned.Path, Metadata: recorded.Meta}
}

func (r applyRun) unitParams(planned Planned) provision.UnitParams {
	f, req := r.in.Facts, r.in.Request
	return provision.UnitParams{
		Ports: provision.Ports{Git: r.ports.Git, Meta: r.ports.Meta, Env: r.ports.Env, StateDir: r.ports.StateDir, PublicPort: r.ports.PublicPort, Now: r.ports.Now},
		Project: provision.Project{
			MainPath: f.MainPath,
			EnvFiles: f.Config.Env.Files,
			OnCreate: f.Config.Hooks.OnCreate,
			Run:      f.Run,
			RunErr:   runErr(f),
			Global:   f.Global,
		},
		Worktree: provision.Worktree{
			Branch:     planned.Branch,
			Path:       planned.Path,
			Checkout:   checkoutOf(checkoutParams{Facts: f, Planned: planned, From: req.From}),
			Source:     req.From,
			ParentPath: f.Identity.CheckedOutAt(req.From),
			Strategy:   req.EnvFrom,
			Isolation:  r.in.Plan.Detail.Isolation,
			Origin:     originOf(f, planned),
		},
	}
}

func runErr(f Facts) error {
	if f.RunProblem == "" {
		return nil
	}
	return errors.New(f.RunProblem)
}

type checkoutParams struct {
	Facts   Facts
	Planned Planned
	From    string
}

func checkoutOf(params checkoutParams) git.Checkout {
	switch params.Planned.Action {
	case ActionCheckOut:
		return git.ExistingBranch{Name: params.Planned.Branch}
	case ActionResume:
		return git.TrackingBranch{Name: params.Planned.Branch}
	case ActionCreate, ActionUnchanged:
	}
	return git.NewBranch{Name: params.Planned.Branch, From: sourceRef(params.Facts, params.From)}
}

func originOf(f Facts, planned Planned) provision.Origin {
	switch planned.Action {
	case ActionCheckOut:
		state, counts := f.divergence(planned.Branch)
		return provision.Origin{ExistingBranch: true, OriginState: rules.DivergenceStateString(state), OriginAhead: counts.Ahead, OriginBehind: counts.Behind}
	case ActionResume:
		return provision.Origin{Upstream: planned.Upstream}
	case ActionCreate, ActionUnchanged:
	}
	return provision.Origin{}
}

func (r applyRun) remember(ctx context.Context, items []kernel.Item[provision.Detail]) []kernel.Warning {
	change := r.remembered()
	if len(change.Remember) == 0 || !slices.ContainsFunc(items, created) {
		return nil
	}
	written, err := r.ports.Config.Remember(ctx, change)
	if err != nil {
		return []kernel.Warning{{Code: CodeRememberFailed, Params: kernel.Params{ParamCause: err.Error()}}}
	}
	if written.Unlocked {
		return []kernel.Warning{{Code: kernel.CodeLocksUnavailable}}
	}
	return nil
}

func created(item kernel.Item[provision.Detail]) bool {
	return item.Status == kernel.StatusDone && item.Reason != kernel.ReasonUnchanged
}

func (r applyRun) remembered() rules.RememberedChange {
	req := r.in.Request
	values := map[string]string{
		PathEnvFrom:     string(req.EnvFrom),
		PathIsolation:   string(req.Isolation),
		PathFastForward: fastForwardAnswer(req.FastForward),
	}
	ids := map[string]string{PathEnvFrom: domain.RememberEnvStrategy, PathIsolation: domain.RememberIsolation, PathFastForward: domain.RememberSourceUpdate}
	change := rules.RememberedChange{Remember: map[string]string{}}
	for _, path := range req.Remember {
		if value := values[path]; value != "" {
			change.Remember[ids[path]] = value
		}
	}
	return change
}

func fastForwardAnswer(fastForward bool) string {
	if fastForward {
		return updateFF
	}
	return updateKeep
}
