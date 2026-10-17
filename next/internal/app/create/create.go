// Package create gives each branch asked its worktree, from one source: what the request settles on, what becomes of each branch, then each provisioned whole.
package create

import (
	"cmp"
	"context"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/app/provision"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/env"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

const CodeConfigNotFound = "create.config_not_found"

type Request struct {
	Branches    []string            `json:"branches"`
	From        string              `json:"from"`
	EnvFrom     project.EnvStrategy `json:"env_from"`
	Isolation   project.Isolation   `json:"isolation"`
	FastForward *bool               `json:"fast_forward"`
	IfNotExists bool                `json:"if_not_exists"`
	Remember    []string            `json:"remember"`
	Ask         bool                `json:"-"`
}

type Git interface {
	Worktrees(ctx context.Context) ([]worktree.Worktree, error)
	Branches(ctx context.Context) (worktree.Branches, error)
	AheadBehind(ctx context.Context, name string) (worktree.AheadBehind, error)
	Locate(ctx context.Context, path string) (worktree.Location, error)
	FetchRef(ctx context.Context, name string) error
	FastForwardRef(ctx context.Context, name string) error
	MergeFFOnly(ctx context.Context, merge worktree.Merge) error
	Dirty(ctx context.Context, path string) (bool, error)
}

type Deps struct {
	Git        Git
	LoadConfig func() (project.Config, project.Global, error)
	LoadRun    func() (project.Run, error)
	Remember   func(ctx context.Context, answers map[string]string) (wtm.Written, error)
	Provision  provision.Deps
	MainPath   string
	StateDir   string
}

func (d Deps) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := d.Observe(ctx, req.Branches)
	if err != nil {
		return Questions{}, err
	}
	return Form(facts, req), nil
}

func (d Deps) Plan(ctx context.Context, req Request) (Plan, []wtm.Warning, error) {
	facts, err := d.Observe(ctx, req.Branches)
	if err != nil {
		return Plan{}, nil, err
	}
	resolved, err := Resolve(facts, req)
	if err != nil {
		return Plan{}, nil, err
	}
	return Decide(facts, resolved)
}

func (d Deps) Apply(ctx context.Context, plan Plan, report wtm.Report) Result {
	warnings := d.fastForward(ctx, plan, report)
	items := make([]Item, 0, len(plan.Branches))
	for _, planned := range plan.Branches {
		items = append(items, d.item(ctx, plan, planned, report))
	}
	return Result{Items: items, Warnings: append(warnings, d.remember(ctx, plan, items)...)}
}

type Action string

const (
	ActionCreate    Action = "create"
	ActionCheckOut  Action = "check_out"
	ActionResume    Action = "resume"
	ActionUnchanged Action = "unchanged"
)

type Plan struct {
	Branches    []BranchPlan        `json:"branches"`
	From        string              `json:"from,omitempty"`
	EnvFrom     project.EnvStrategy `json:"env_from"`
	Isolation   project.Isolation   `json:"isolation,omitempty"`
	FastForward []string            `json:"fast_forward,omitempty"`
	Remember    []string            `json:"remember,omitempty"`
	facts       Facts
	isolation   project.Isolation
	fastForward bool
}

type BranchPlan struct {
	Branch   string `json:"branch"`
	Path     string `json:"path"`
	Action   Action `json:"action"`
	Upstream string `json:"upstream,omitempty"`
}

func (p Plan) Locks() []wtm.Lock {
	names := make([]string, 0, len(p.Branches)+len(p.FastForward))
	for _, planned := range p.Branches {
		names = append(names, planned.Branch)
	}
	return locksOf(append(names, p.FastForward...))
}

func (p Plan) Fingerprinted() any {
	return struct {
		Plan      Plan              `json:"plan"`
		Isolation project.Isolation `json:"isolation"`
	}{Plan: p, Isolation: p.isolation}
}

func (r Request) Locks() []wtm.Lock {
	return locksOf(slices.DeleteFunc(slices.Clone(r.Branches), func(name string) bool { return name == "" }))
}

func locksOf(worktrees []string) []wtm.Lock {
	locks := []wtm.Lock{{Scope: wtm.ScopeRepo, Mode: wtm.LockShared}}
	slices.Sort(worktrees)
	for _, name := range slices.Compact(worktrees) {
		locks = append(locks, wtm.Lock{Scope: wtm.ScopeWorktree, Name: name, Mode: wtm.LockExclusive})
	}
	return locks
}

type Result struct {
	Items    []Item
	Warnings []wtm.Warning
}

func (r Result) Failure() *wtm.Error {
	if at := slices.IndexFunc(r.Items, interrupted); at >= 0 {
		return cmp.Or(r.Items[at].Err, &wtm.Error{Kind: wtm.KindCancelled, Code: wtm.CodeInterrupted, Message: "interrupted"})
	}
	if at := slices.IndexFunc(r.Items, func(item Item) bool { return item.Status == wtm.StatusFailed }); at >= 0 {
		return r.Items[at].Err
	}
	return nil
}

func (r Result) Warned() []wtm.Warning {
	return r.Warnings
}

func interrupted(item Item) bool {
	return item.Status == wtm.StatusCancelled || item.Reason == wtm.ReasonInterrupted
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
	Ordinal  *int         `json:"ordinal,omitempty"`
	EnvPorts env.Redacted `json:"env_ports,omitzero"`
}

type Standing struct {
	ExistingBranch bool                `json:"existing_branch"`
	OriginState    worktree.Divergence `json:"origin_state,omitempty"`
	OriginAhead    int                 `json:"origin_ahead,omitempty"`
	OriginBehind   int                 `json:"origin_behind,omitempty"`
	Upstream       string              `json:"upstream,omitempty"`
}

func (d Deps) fastForward(ctx context.Context, plan Plan, report wtm.Report) []wtm.Warning {
	var warnings []wtm.Warning
	for _, subject := range plan.FastForward {
		if ctx.Err() != nil {
			return warnings
		}
		report(wtm.PhaseStarted{Subject: subject, Phase: "fast_forward"})
		warning, failed := d.advance(ctx, plan, subject)
		report(wtm.PhaseFinished{Subject: subject, Phase: "fast_forward", Failed: failed})
		if failed && ctx.Err() == nil {
			warnings = append(warnings, warning)
		}
	}
	return warnings
}

func (d Deps) advance(ctx context.Context, plan Plan, subject string) (wtm.Warning, bool) {
	at := plan.facts.checkedOutAt(subject)
	if at == "" {
		return notAdvanced(subject, d.Git.FastForwardRef(ctx, subject))
	}
	if err := d.Git.FetchRef(ctx, subject); err != nil {
		return notAdvanced(subject, err)
	}
	dirty, err := d.Git.Dirty(ctx, at)
	if err != nil {
		return notAdvanced(subject, err)
	}
	if dirty {
		return wtm.Warning{
			Code:    "create.ff_dirty",
			Message: "couldn't fast-forward " + subject + ": its worktree has uncommitted changes — it starts from your local branch",
			Params:  wtm.Params{"branch": subject},
		}, true
	}
	return notAdvanced(subject, d.Git.MergeFFOnly(ctx, worktree.Merge{Path: at, Onto: worktree.RemoteBranch{Name: subject}}))
}

func notAdvanced(subject string, err error) (wtm.Warning, bool) {
	if err == nil {
		return wtm.Warning{}, false
	}
	return wtm.Warning{
		Code:    "create.ff_failed",
		Message: "couldn't fast-forward " + subject + ": " + err.Error() + " — it starts from your local branch",
		Params:  wtm.Params{"branch": subject, "cause": err.Error()},
	}, true
}

func (d Deps) item(ctx context.Context, plan Plan, planned BranchPlan, report wtm.Report) Item {
	switch {
	case planned.Action == ActionUnchanged:
		return d.unchanged(planned)
	case ctx.Err() != nil:
		return Item{Subject: planned.Branch, Status: wtm.StatusSkipped, Reason: wtm.ReasonInterrupted}
	}
	report(wtm.UnitStarted{Subject: planned.Branch})
	outcome := provision.Run(ctx, d.provisionParams(plan, planned, report))
	item := Item{Subject: planned.Branch, Status: outcome.Status, Reason: outcome.Reason, Err: outcome.Err, Warnings: outcome.Warnings}
	if outcome.Provisioned.Meta.CreatedAt != "" {
		item.Detail = &Detail{
			Path:     planned.Path,
			Metadata: outcome.Provisioned.Meta,
			Standing: plan.facts.standingOf(planned),
			Ordinal:  outcome.Provisioned.Ordinal,
			EnvPorts: outcome.Provisioned.EnvPorts,
		}
	}
	report(wtm.UnitFinished{Subject: planned.Branch, Status: item.Status})
	return item
}

func (d Deps) unchanged(planned BranchPlan) Item {
	recorded, _ := d.Provision.Meta.Load(planned.Branch)
	return Item{Subject: planned.Branch, Status: wtm.StatusDone, Reason: wtm.ReasonUnchanged, Detail: &Detail{Path: planned.Path, Metadata: recorded}}
}

func (d Deps) provisionParams(plan Plan, planned BranchPlan, report wtm.Report) provision.Params {
	f := plan.facts
	return provision.Params{
		Deps: d.Provision,
		Project: provision.Project{
			MainPath: f.MainPath,
			StateDir: d.StateDir,
			EnvFiles: f.Config.Env.Files,
			OnCreate: f.Config.OnCreate,
			Run:      f.Run,
			RunErr:   f.RunErr,
			Global:   f.Global,
		},
		Worktree: provision.Worktree{
			Branch:     planned.Branch,
			Path:       planned.Path,
			Checkout:   plan.checkout(planned),
			Source:     plan.From,
			ParentPath: f.checkedOutAt(plan.From),
			Strategy:   plan.EnvFrom,
			Isolation:  plan.isolation,
		},
		Report: report,
	}
}

func (p Plan) checkout(planned BranchPlan) worktree.Checkout {
	switch planned.Action {
	case ActionCheckOut:
		return worktree.ExistingBranch{Name: planned.Branch}
	case ActionResume:
		return worktree.TrackingBranch{Name: planned.Branch}
	case ActionCreate, ActionUnchanged:
	}
	return worktree.NewBranch{Name: planned.Branch, From: p.facts.ref(p.From)}
}

func (f Facts) ref(from string) worktree.Ref {
	if remote, onOrigin := cutRemote(from); onOrigin && !f.local(from) {
		return worktree.RemoteBranch{Name: remote}
	}
	return worktree.LocalBranch{Name: from}
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

func (d Deps) remember(ctx context.Context, plan Plan, items []Item) []wtm.Warning {
	answers := plan.answers()
	if len(answers) == 0 || !slices.ContainsFunc(items, created) {
		return nil
	}
	written, err := d.Remember(ctx, answers)
	if err != nil {
		return []wtm.Warning{{Code: "create.remember_failed", Message: "could not remember the answer: " + err.Error(), Params: wtm.Params{"cause": err.Error()}}}
	}
	if written.Unlocked {
		return []wtm.Warning{wtm.LocksUnavailable}
	}
	return nil
}

func created(item Item) bool {
	return item.Status == wtm.StatusDone && item.Reason != wtm.ReasonUnchanged
}

func (p Plan) answers() map[string]string {
	sourceUpdate := project.SourceUpdateKeep
	if p.fastForward {
		sourceUpdate = project.SourceUpdateFastForward
	}
	values := map[string]string{"env_from": string(p.EnvFrom), "isolation": string(p.Isolation), "fast_forward": sourceUpdate}
	answers := map[string]string{}
	for _, r := range remembered {
		if value := values[r.field]; value != "" && slices.Contains(p.Remember, r.field) {
			answers[r.id] = value
		}
	}
	return answers
}
