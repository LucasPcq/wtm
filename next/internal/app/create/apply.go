package create

import (
	"context"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/app/provision"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
	"github.com/LucasPcq/wtm/next/internal/wtm/settle"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

type ApplyParams struct {
	Deps    Deps
	Facts   Facts
	Request Request
	Plan    Planned
	Report  wtm.Report
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

type applying struct {
	ApplyParams
}

func Apply(ctx context.Context, params ApplyParams) Result {
	a := applying{ApplyParams: params}
	warnings := a.fastForward(ctx)
	items := make([]Item, 0, len(params.Plan.Branches))
	for _, planned := range params.Plan.Branches {
		items = append(items, a.item(ctx, planned))
	}
	return Result{Items: items, Warnings: append(warnings, a.remember(ctx, items)...)}
}

func (a applying) item(ctx context.Context, planned BranchPlan) Item {
	switch {
	case planned.Action == ActionUnchanged:
		return a.unchanged(planned)
	case ctx.Err() != nil:
		return Item{Subject: planned.Branch, Status: wtm.StatusSkipped, Reason: wtm.ReasonInterrupted}
	}
	a.Report(wtm.UnitStarted{Subject: planned.Branch})
	outcome := provision.Run(ctx, a.provisionParams(planned))
	item := Item{Subject: planned.Branch, Status: outcome.Status, Reason: outcome.Reason, Err: outcome.Err, Warnings: outcome.Warnings}
	if outcome.Provisioned.Meta.CreatedAt != "" {
		item.Detail = &Detail{
			Path:     planned.Path,
			Metadata: outcome.Provisioned.Meta,
			Standing: a.Facts.standingOf(planned),
			Ordinal:  outcome.Provisioned.Ordinal,
			EnvPorts: outcome.Provisioned.EnvPorts,
		}
	}
	a.Report(wtm.UnitFinished{Subject: planned.Branch, Status: item.Status})
	return item
}

func (a applying) unchanged(planned BranchPlan) Item {
	recorded, _ := a.Deps.Provision.Meta.Load(planned.Branch)
	return Item{Subject: planned.Branch, Status: wtm.StatusDone, Reason: wtm.ReasonUnchanged, Detail: &Detail{Path: planned.Path, Metadata: recorded}}
}

func (a applying) provisionParams(planned BranchPlan) provision.Params {
	f, req := a.Facts, a.Request
	return provision.Params{
		Deps: a.Deps.Provision,
		Project: provision.Project{
			MainPath: f.MainPath,
			StateDir: a.Deps.StateDir,
			EnvFiles: f.Config.Env.Files,
			OnCreate: f.Config.OnCreate,
			Run:      f.Run,
			RunErr:   f.RunErr,
			Global:   f.Global,
		},
		Worktree: provision.Worktree{
			Branch:     planned.Branch,
			Path:       planned.Path,
			Checkout:   a.checkout(planned),
			Source:     req.From,
			ParentPath: f.checkedOutAt(req.From),
			Strategy:   req.EnvFrom,
			Isolation:  req.Isolation,
		},
		Report: a.Report,
	}
}

func (a applying) checkout(planned BranchPlan) worktree.Checkout {
	switch planned.Action {
	case ActionCheckOut:
		return worktree.ExistingBranch{Name: planned.Branch}
	case ActionResume:
		return worktree.TrackingBranch{Name: planned.Branch}
	case ActionCreate, ActionUnchanged:
	}
	return worktree.NewBranch{Name: planned.Branch, From: a.Facts.ref(a.Request.From)}
}

func (f Facts) ref(from string) branch.Ref {
	if remote, onOrigin := strings.CutPrefix(from, remotePrefix); onOrigin && !f.local(from) {
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
