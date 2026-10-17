// Package extract moves a subset of a worktree's uncommitted changes to another worktree, existing or created for them: what the request settles on, what conflicts, then the target made and the changes moved.
package extract

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
)

const (
	name               = "extract"
	CodeConfigNotFound = "extract.config_not_found"
	CodeConflict       = "extract.conflict"
	CodeHalfMoved      = "extract.half_moved"
)

type UseCase struct {
	Git        Git
	Config     Config
	Answers    Answers
	Meta       Meta
	Shield     Shield
	Operations operation.Service
	Provision  provision.Service
	MainPath   string
	StateDir   string
}

type Outcome struct {
	Op          string
	Plan        Plan
	Fingerprint string
	Warnings    []domain.Warning
	Result      *Result
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
	if err != nil || opts.DryRun || plan.Nothing() {
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
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Plan{}, nil, err
	}
	resolved, err := Resolve(facts, req)
	if err != nil {
		return Plan{}, nil, err
	}
	conflicts, err := u.Git.Conflicts(ctx, facts.conflictCheck(resolved))
	if err != nil {
		return Plan{}, nil, err
	}
	return Decide(facts, resolved, conflicts)
}

func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	warnings := u.fastForward(ctx, plan, report)
	if ctx.Err() != nil {
		return Result{Item: Item{Subject: plan.Target.Branch, Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted}, Warnings: warnings}
	}
	report(domain.UnitStarted{Subject: plan.Target.Branch})
	item := u.target(ctx, plan, report)
	if item.Status == domain.StatusDone {
		item = u.move(ctx, plan, item, report)
	}
	report(domain.UnitFinished{Subject: item.Subject, Status: item.Status})
	if plan.Target.creates() && item.Detail != nil {
		warnings = append(warnings, u.remember(ctx, plan)...)
	}
	return Result{Item: item, Warnings: warnings}
}

func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Questions{}, err
	}
	questions := Form(facts, req)
	if len(questions.Errors) > 0 {
		return questions, nil
	}
	conflicts, err := u.Git.Conflicts(ctx, facts.conflictCheck(questions.Request()))
	if err != nil {
		return Questions{}, err
	}
	return questions.facing(conflicts), nil
}

func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
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
	if facts.Changes, err = u.changes(ctx, facts.sources(req.Source)); err != nil {
		return Facts{}, err
	}
	if facts.Records, err = u.records(facts.checkedOut(req.Source, req.To)); err != nil {
		return Facts{}, err
	}
	if facts.Folder, err = u.locate(ctx, facts.newPath(req.To)); err != nil {
		return Facts{}, err
	}
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

func (u UseCase) changes(ctx context.Context, sources []worktree.Worktree) (map[string][]worktree.Change, error) {
	found := make(map[string][]worktree.Change, len(sources))
	for _, source := range sources {
		changes, err := u.Git.Changes(ctx, source.Path)
		if err != nil {
			return nil, err
		}
		found[source.Branch()] = changes
	}
	return found, nil
}

func (u UseCase) records(branches []string) (map[string]worktree.Meta, error) {
	found := make(map[string]worktree.Meta, len(branches))
	for _, branch := range branches {
		recorded, err := u.Meta.Load(branch)
		if err != nil && !errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		found[branch] = recorded
	}
	return found, nil
}

func (u UseCase) locate(ctx context.Context, path string) (worktree.Location, error) {
	if path == "" {
		return worktree.Location{}, nil
	}
	return u.Git.Locate(ctx, path)
}

type Result struct {
	Item     Item
	Warnings []domain.Warning
}

func (r Result) Failure() *domain.Error {
	switch {
	case r.Item.Status == domain.StatusCancelled || r.Item.Reason == domain.ReasonInterrupted:
		return cmp.Or(r.Item.Err, &domain.Error{Kind: domain.KindCancelled, Code: domain.CodeInterrupted, Message: "interrupted"})
	case r.Item.Status == domain.StatusFailed:
		return r.Item.Err
	}
	return nil
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
	Path      string            `json:"path"`
	Files     []worktree.Change `json:"files"`
	Kept      bool              `json:"kept"`
	Conflicts []string          `json:"conflicts"`
	Isolation project.Isolation `json:"isolation,omitempty"`
	EnvPorts  env.Redacted      `json:"env_ports,omitzero"`
	Standing
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
			Code:    "extract.ff_dirty",
			Message: "couldn't fast-forward " + subject + ": its worktree has uncommitted changes — the target starts from your local branch",
			Params:  domain.Params{"branch": subject},
		}, true
	}
	if forwarded.Err == nil {
		return domain.Warning{}, false
	}
	return domain.Warning{
		Code:    "extract.ff_failed",
		Message: "couldn't fast-forward " + subject + ": " + forwarded.Err.Error() + " — the target starts from your local branch",
		Params:  domain.Params{"branch": subject, "cause": forwarded.Err.Error()},
	}, true
}

func (u UseCase) target(ctx context.Context, plan Plan, report domain.Report) Item {
	if !plan.Target.creates() {
		return Item{Subject: plan.Target.Branch, Status: domain.StatusDone, Detail: detail(plan.Target.Path, plan.facts.recordedIsolation(plan.Target.Branch))}
	}
	outcome := u.Provision.Run(ctx, u.provisionParams(plan, report))
	item := Item{Subject: plan.Target.Branch, Status: outcome.Status, Reason: outcome.Reason, Err: outcome.Err, Warnings: outcome.Warnings}
	if outcome.Provisioned.Meta.CreatedAt != "" {
		item.Detail = detail(plan.Target.Path, outcome.Provisioned.Meta.Isolation)
		item.Detail.EnvPorts = outcome.Provisioned.EnvPorts
		item.Detail.Standing = plan.facts.standingOf(plan.Target)
	}
	return item
}

func detail(path string, isolation project.Isolation) *Detail {
	return &Detail{Path: path, Files: []worktree.Change{}, Conflicts: []string{}, Isolation: isolation}
}

func (u UseCase) provisionParams(plan Plan, report domain.Report) provision.Params {
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
			Branch:     plan.Target.Branch,
			Path:       plan.Target.Path,
			Checkout:   plan.checkout(),
			Source:     plan.From,
			ParentPath: f.checkedOutAt(plan.From),
			Strategy:   f.Config.Env.Strategy,
			Isolation:  plan.isolation,
		},
		Report: report,
	}
}

func (u UseCase) move(ctx context.Context, plan Plan, item Item, report domain.Report) Item {
	if ctx.Err() != nil {
		item.Status, item.Reason = domain.StatusCancelled, domain.ReasonInterrupted
		return item
	}
	report(domain.PhaseStarted{Subject: item.Subject, Phase: "move"})
	shielded, release := u.Shield(ctx)
	moved, err := u.Git.Move(shielded, plan.move())
	release()
	report(domain.PhaseFinished{Subject: item.Subject, Phase: "move", Failed: err != nil})
	if err != nil {
		item.Status, item.Err = domain.StatusFailed, plan.failure(err)
		return item
	}
	item.Detail.Files, item.Detail.Kept = plan.Files, moved.Kept
	item.Detail.Conflicts = append(item.Detail.Conflicts, moved.Merged...)
	return item
}

func (p Plan) failure(err error) *domain.Error {
	if refused, ok := errors.AsType[*worktree.ConflictRefusal](err); ok {
		return conflictRefusal(conflictRefusalParams{Conflicts: refused.Conflicts, Target: p.Target.Branch})
	}
	if half, ok := errors.AsType[*worktree.HalfMoved](err); ok {
		return &domain.Error{
			Kind:    domain.KindInternal,
			Code:    CodeHalfMoved,
			Message: fmt.Sprintf("the changes are in %q but %q still holds them", p.Target.Branch, p.Source),
			Params:  domain.Params{"target": p.Target.Branch, "source": p.Source},
			Cause:   half.Cause,
		}
	}
	return domain.AsError(domain.AsErrorParams{Err: err, Code: "extract.move_failed", Message: "move failed"})
}

func (u UseCase) remember(ctx context.Context, plan Plan) []domain.Warning {
	answers := plan.answers()
	if len(answers) == 0 {
		return nil
	}
	written, err := u.Answers.Remember(ctx, answers)
	if err != nil {
		return []domain.Warning{{Code: "extract.remember_failed", Message: "could not remember the answer: " + err.Error(), Params: domain.Params{"cause": err.Error()}}}
	}
	if written.Unlocked {
		return []domain.Warning{domain.LocksUnavailable}
	}
	return nil
}
