// Package create gives each branch asked its worktree, from one source: what the request settles on, what becomes of each branch, then each provisioned whole.
package create

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
)

const (
	name               = "create"
	CodeConfigNotFound = "create.config_not_found"
)

type UseCase struct {
	Git        Git
	Config     Config
	Answers    Answers
	Operations operation.Service
	Provision  provision.Service
	MainPath   string
	StateDir   string
}

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

type Outcome struct {
	Op          string
	Plan        Plan
	Fingerprint string
	Warnings    []domain.Warning
	Result      *Result
}

func (o Outcome) Failure() *domain.Error {
	if o.Result == nil {
		return nil
	}
	return o.Result.Failure()
}

func (u UseCase) Run(ctx context.Context, req Request, opts operation.Options) (Outcome, error) {
	op, err := u.Operations.Begin(operation.BeginParams{Command: name, Locks: req.Locks(), Options: opts})
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()
	plan, warnings, err := operation.PlanUnderLocks(ctx, op, func(ctx context.Context) (Plan, []domain.Warning, error) {
		return u.Plan(ctx, req)
	})
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	outcome := Outcome{Op: op.ID, Plan: plan, Warnings: warnings}
	outcome.Fingerprint, err = operation.Fingerprint(plan.Fingerprinted())
	if err != nil || opts.DryRun {
		return outcome, err
	}
	if err := op.Expect(outcome.Fingerprint); err != nil {
		return outcome, err
	}
	report := op.Started()
	result := u.Apply(ctx, plan, report)
	op.Finished(result.Failure())
	outcome.Result = &result
	outcome.Warnings = op.Warnings(append(outcome.Warnings, result.Warnings...))
	return outcome, nil
}

func (u UseCase) Plan(ctx context.Context, req Request) (Plan, []domain.Warning, error) {
	facts, err := u.Observe(ctx, req.Branches)
	if err != nil {
		return Plan{}, nil, err
	}
	resolved, err := Resolve(facts, req)
	if err != nil {
		return Plan{}, nil, err
	}
	return Decide(facts, resolved)
}

func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	warnings := u.fastForward(ctx, plan, report)
	items := make([]Item, 0, len(plan.Branches))
	for _, planned := range plan.Branches {
		items = append(items, u.item(ctx, plan, planned, report))
	}
	return Result{Items: items, Warnings: append(warnings, u.remember(ctx, plan, items)...)}
}

func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req.Branches)
	if err != nil {
		return Questions{}, err
	}
	return Form(facts, req), nil
}

func (u UseCase) Observe(ctx context.Context, branches []string) (Facts, error) {
	config, global, err := u.Config.Load()
	if errors.Is(err, project.ErrConfigNotFound) {
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeConfigNotFound, Message: "no wtm config found — run `wtm init` first", Cause: err}
	}
	if err != nil {
		return Facts{}, err
	}
	worktrees, err := u.Git.Worktrees(ctx)
	if err != nil {
		return Facts{}, err
	}
	refs, err := u.Git.Branches(ctx)
	if err != nil {
		return Facts{}, err
	}
	divergence, err := u.divergence(ctx, refs)
	if err != nil {
		return Facts{}, err
	}
	facts := Facts{MainPath: u.MainPath, Config: config, Global: global, Worktrees: worktrees, Branches: refs, Divergence: divergence}
	folders, err := u.locate(ctx, facts.pathsOf(branches))
	if err != nil {
		return Facts{}, err
	}
	facts.Folders = folders
	facts.Run, facts.RunErr = u.Config.LoadRun()
	return facts, nil
}

func (u UseCase) divergence(ctx context.Context, refs worktree.Branches) (map[string]worktree.AheadBehind, error) {
	found := map[string]worktree.AheadBehind{}
	for _, local := range refs.Local {
		at := slices.IndexFunc(refs.Remote, func(remote worktree.Branch) bool { return remote.Name == local.Name })
		if at < 0 || refs.Remote[at].Tip == local.Tip {
			continue
		}
		counts, err := u.Git.AheadBehind(ctx, local.Name)
		if err != nil {
			return nil, err
		}
		found[local.Name] = counts
	}
	return found, nil
}

func (u UseCase) locate(ctx context.Context, paths []string) (map[string]worktree.Location, error) {
	folders := make(map[string]worktree.Location, len(paths))
	for _, path := range paths {
		location, err := u.Git.Locate(ctx, path)
		if err != nil {
			return nil, err
		}
		folders[path] = location
	}
	return folders, nil
}

type Result struct {
	Items    []Item
	Warnings []domain.Warning
}

func (r Result) Failure() *domain.Error {
	if at := slices.IndexFunc(r.Items, interrupted); at >= 0 {
		return cmp.Or(r.Items[at].Err, &domain.Error{Kind: domain.KindCancelled, Code: domain.CodeInterrupted, Message: "interrupted"})
	}
	if at := slices.IndexFunc(r.Items, func(item Item) bool { return item.Status == domain.StatusFailed }); at >= 0 {
		return r.Items[at].Err
	}
	return nil
}

func interrupted(item Item) bool {
	return item.Status == domain.StatusCancelled || item.Reason == domain.ReasonInterrupted
}

type Item struct {
	Subject  string
	Status   domain.Status
	Reason   domain.Reason
	Err      *domain.Error
	Warnings []domain.Warning
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

func (u UseCase) fastForward(ctx context.Context, plan Plan, report domain.Report) []domain.Warning {
	var warnings []domain.Warning
	for _, subject := range plan.FastForward {
		if ctx.Err() != nil {
			return warnings
		}
		report(domain.PhaseStarted{Subject: subject, Phase: "fast_forward"})
		warning, failed := u.advance(ctx, plan, subject)
		report(domain.PhaseFinished{Subject: subject, Phase: "fast_forward", Failed: failed})
		if failed && ctx.Err() == nil {
			warnings = append(warnings, warning)
		}
	}
	return warnings
}

func (u UseCase) advance(ctx context.Context, plan Plan, subject string) (domain.Warning, bool) {
	forwarded := u.Provision.FastForward(ctx, provision.FastForwardParams{Subject: subject, CheckedOutAt: plan.facts.checkedOutAt(subject)})
	if forwarded.Dirty {
		return domain.Warning{
			Code:    "create.ff_dirty",
			Message: "couldn't fast-forward " + subject + ": its worktree has uncommitted changes — it starts from your local branch",
			Params:  domain.Params{"branch": subject},
		}, true
	}
	return notAdvanced(subject, forwarded.Err)
}

func notAdvanced(subject string, err error) (domain.Warning, bool) {
	if err == nil {
		return domain.Warning{}, false
	}
	return domain.Warning{
		Code:    "create.ff_failed",
		Message: "couldn't fast-forward " + subject + ": " + err.Error() + " — it starts from your local branch",
		Params:  domain.Params{"branch": subject, "cause": err.Error()},
	}, true
}

func (u UseCase) item(ctx context.Context, plan Plan, planned BranchPlan, report domain.Report) Item {
	switch {
	case planned.Action == ActionUnchanged:
		return u.unchanged(planned)
	case ctx.Err() != nil:
		return Item{Subject: planned.Branch, Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted}
	}
	report(domain.UnitStarted{Subject: planned.Branch})
	outcome := u.Provision.Run(ctx, u.provisionParams(plan, planned, report))
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
	report(domain.UnitFinished{Subject: planned.Branch, Status: item.Status})
	return item
}

func (u UseCase) unchanged(planned BranchPlan) Item {
	recorded, _ := u.Provision.Meta.Load(planned.Branch)
	return Item{Subject: planned.Branch, Status: domain.StatusDone, Reason: domain.ReasonUnchanged, Detail: &Detail{Path: planned.Path, Metadata: recorded}}
}

func (u UseCase) provisionParams(plan Plan, planned BranchPlan, report domain.Report) provision.Params {
	f := plan.facts
	return provision.Params{
		Project: provision.Project{
			MainPath: f.MainPath,
			StateDir: u.StateDir,
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

func (u UseCase) remember(ctx context.Context, plan Plan, items []Item) []domain.Warning {
	answers := plan.answers()
	if len(answers) == 0 || !slices.ContainsFunc(items, created) {
		return nil
	}
	written, err := u.Answers.Remember(ctx, answers)
	if err != nil {
		return []domain.Warning{{Code: "create.remember_failed", Message: "could not remember the answer: " + err.Error(), Params: domain.Params{"cause": err.Error()}}}
	}
	if written.Unlocked {
		return []domain.Warning{domain.LocksUnavailable}
	}
	return nil
}

func created(item Item) bool {
	return item.Status == domain.StatusDone && item.Reason != domain.ReasonUnchanged
}
