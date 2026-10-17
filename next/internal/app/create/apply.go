package create

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/app/provision"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/settle"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

type ApplyParams struct {
	Deps        Deps
	Facts       Facts
	Plan        Planned
	FastForward bool
	Report      wtm.Report
}

type Result struct {
	Items    []Item
	Warnings []wtm.Warning
}

type Item struct {
	Subject  string
	Status   wtm.Status
	Reason   wtm.Reason
	Err      *wtm.Error
	Warnings []wtm.Warning
	Detail   *Detail
}

type Detail struct {
	Path     string        `json:"path"`
	Metadata worktree.Meta `json:"metadata"`
	Standing
	Ordinal  *int            `json:"ordinal,omitempty"`
	EnvPorts settle.Redacted `json:"env_ports,omitzero"`
}

type Standing struct {
	ExistingBranch bool              `json:"existing_branch"`
	OriginState    branch.Divergence `json:"origin_state,omitempty"`
	OriginAhead    int               `json:"origin_ahead,omitempty"`
	OriginBehind   int               `json:"origin_behind,omitempty"`
	Upstream       string            `json:"upstream,omitempty"`
}

func Apply(ctx context.Context, params ApplyParams) Result {
	warnings := params.fastForward(ctx)
	items := make([]Item, 0, len(params.Plan.Branches))
	for _, planned := range params.Plan.Branches {
		items = append(items, params.item(ctx, planned))
	}
	return Result{Items: items, Warnings: append(warnings, params.remember(ctx, items)...)}
}

func (p ApplyParams) item(ctx context.Context, planned BranchPlan) Item {
	switch {
	case planned.Action == ActionUnchanged:
		return p.unchanged(planned)
	case ctx.Err() != nil:
		return Item{Subject: planned.Branch, Status: wtm.StatusSkipped, Reason: wtm.ReasonInterrupted}
	}
	p.Report(wtm.UnitStarted{Subject: planned.Branch})
	outcome := provision.Run(ctx, p.provisionParams(planned))
	item := Item{Subject: planned.Branch, Status: outcome.Status, Reason: outcome.Reason, Err: outcome.Err, Warnings: outcome.Warnings}
	if outcome.Provisioned.Meta.CreatedAt != "" {
		item.Detail = &Detail{
			Path:     planned.Path,
			Metadata: outcome.Provisioned.Meta,
			Standing: p.Facts.standingOf(planned),
			Ordinal:  outcome.Provisioned.Ordinal,
			EnvPorts: outcome.Provisioned.EnvPorts,
		}
	}
	p.Report(wtm.UnitFinished{Subject: planned.Branch, Status: item.Status})
	return item
}

func (p ApplyParams) unchanged(planned BranchPlan) Item {
	recorded, _ := p.Deps.Provision.Meta.Load(planned.Branch)
	return Item{Subject: planned.Branch, Status: wtm.StatusDone, Reason: wtm.ReasonUnchanged, Detail: &Detail{Path: planned.Path, Metadata: recorded}}
}

func (p ApplyParams) provisionParams(planned BranchPlan) provision.Params {
	f := p.Facts
	return provision.Params{
		Deps: p.Deps.Provision,
		Project: provision.Project{
			MainPath: f.MainPath,
			StateDir: p.Deps.StateDir,
			EnvFiles: f.Config.Env.Files,
			OnCreate: f.Config.OnCreate,
			Run:      f.Run,
			RunErr:   f.RunErr,
			Global:   f.Global,
		},
		Worktree: provision.Worktree{
			Branch:     planned.Branch,
			Path:       planned.Path,
			Checkout:   p.checkout(planned),
			Source:     p.Plan.From,
			ParentPath: f.checkedOutAt(p.Plan.From),
			Strategy:   p.Plan.EnvFrom,
			Isolation:  p.Plan.Isolation,
		},
		Report: p.Report,
	}
}

func (p ApplyParams) checkout(planned BranchPlan) worktree.Checkout {
	switch planned.Action {
	case ActionCheckOut:
		return worktree.ExistingBranch{Name: planned.Branch}
	case ActionResume:
		return worktree.TrackingBranch{Name: planned.Branch}
	case ActionCreate, ActionUnchanged:
	}
	return worktree.NewBranch{Name: planned.Branch, From: p.Facts.ref(p.Plan.From)}
}

func (f Facts) ref(from string) branch.Ref {
	if remote, onOrigin := cutRemote(from); onOrigin && !f.local(from) {
		return branch.RemoteBranch{Name: remote}
	}
	return branch.LocalBranch{Name: from}
}

func (f Facts) standingOf(planned BranchPlan) Standing {
	switch planned.Action {
	case ActionCheckOut:
		counts := f.Divergence[planned.Branch]
		return Standing{ExistingBranch: true, OriginState: f.standing(planned.Branch), OriginAhead: counts.Ahead, OriginBehind: counts.Behind}
	case ActionResume:
		return Standing{Upstream: planned.Upstream}
	case ActionCreate, ActionUnchanged:
	}
	return Standing{}
}
