// Package create gives each branch asked its worktree, from one source: what the request settles on, what becomes of each branch, then each provisioned whole.
package create

import (
	"context"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = "create"

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + repository.ConfigNotFound

// A UseCase creates worktrees in one repository: the ports it reaches the repository through and the services it shares with the other commands.
type UseCase struct {
	Git        Git
	Meta       Meta
	Repository repository.Service
	Operations operation.Service
	Provision  provision.Service
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied (a dry run, or a refusal before the apply).
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: one Item per branch of the plan, in its order, and the warnings of the whole run.
type Result = domain.Result[Detail]

// An Item is what became of one branch: its status and why, the error it failed on, its warnings, and its Detail once its worktree stands.
type Item = domain.Unit[Detail]

// A Detail describes a branch's worktree as the JSON envelope reports it: where it is, its meta record, where the branch stood against origin and the env ports it was given.
type Detail struct {
	Path     string        `json:"path"`
	Metadata worktree.Meta `json:"metadata"`
	worktree.Standing
	EnvPorts env.Redacted `json:"env_ports,omitzero"`
}

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: it takes the locks, plans under them, fingerprints the plan, stops on a dry run, refuses a stale plan and applies it between the operation's started and finished events. An error returned is a refusal before the apply; a failure during the apply is in the Outcome's Result.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Take the operation's locks before reading anything, so the facts cannot change under the plan.
	op, err := u.Operations.Begin(Name, req.Locks(), opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Plan; a plan naming a worktree the request did not is planned again once that lock is held.
	plan, warnings, err := operation.PlanUnderLocks(ctx, op, func(ctx context.Context) (Plan, []domain.Warning, error) {
		return u.Plan(ctx, req)
	})
	if err != nil {
		return Outcome{Op: op.ID}, err
	}

	// 3. Fingerprint what would be applied; a dry run ends here with the plan and its fingerprint.
	outcome := Outcome{Op: op.ID, Plan: plan, Warnings: warnings}
	outcome.Fingerprint, err = operation.Fingerprint(plan.Fingerprinted())
	if err != nil || opts.DryRun {
		return outcome, err
	}

	// 4. A preview made on other facts is refused rather than applied.
	if err := op.Expect(outcome.Fingerprint); err != nil {
		return outcome, err
	}

	// 5. Apply between the started and finished events other processes listen to.
	report := op.Started()
	result := u.Apply(ctx, plan, report)
	op.Finished(result.Failure())

	outcome.Result = &result
	outcome.Warnings = op.Warnings(append(outcome.Warnings, result.Warnings...))
	return outcome, nil
}

// ----------------------------------------------------------------------------
// Plan

// Plan decides what the request does to the repository as it stands, changing nothing: the plan and its warnings, or the refusal that stops the request (a *domain.Error, field errors included).
func (u UseCase) Plan(ctx context.Context, req Request) (Plan, []domain.Warning, error) {
	facts, err := u.Observe(ctx, req.Branches)
	if err != nil {
		return Plan{}, nil, err
	}

	resolved, err := Resolve(facts, req)
	if err != nil {
		return Plan{}, nil, err
	}

	planned, warnings := Decide(facts, resolved)
	return planned, warnings, nil
}

// ----------------------------------------------------------------------------
// Apply

// Apply carries out the plan, reporting each step: it fast-forwards the sources asked, provisions each branch in turn, a failure stopping none of the others, and remembers the answers asked once a worktree is created. Once ctx is cancelled, the branches not yet started are skipped.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	warnings := u.fastForward(ctx, plan, report)

	items := make([]Item, 0, len(plan.Branches))
	for _, planned := range plan.Branches {
		items = append(items, u.item(ctx, plan, planned, report))
	}

	return Result{Items: items, Warnings: append(warnings, u.remember(ctx, plan, items)...)}
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form the wizard asks for the request: each question with its options, its answer so far and where that answer comes from, and the request's problems as field errors.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req.Branches)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what create decides from: the project's config, the repository's snapshot and what lies at the path each of branches would take. A repository with no wtm config is refused with CodeConfigNotFound.
func (u UseCase) Observe(ctx context.Context, branches []string) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	facts := Facts{Repository: repo}
	facts.Folders, err = u.locate(ctx, facts.pathsOf(branches))
	if err != nil {
		return Facts{}, err
	}

	return facts, nil
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

// ----------------------------------------------------------------------------
// The steps of Apply

// fastForward fast-forwards each source the plan names; one that cannot be is a warning, its worktrees starting from the local branch.
func (u UseCase) fastForward(ctx context.Context, plan Plan, report domain.Report) []domain.Warning {
	var warnings []domain.Warning
	for _, missed := range u.Provision.FastForwardEach(ctx, provision.FastForwardEachParams{Subjects: plan.FastForward, Snapshot: plan.facts.Snapshot, Report: report}) {
		warnings = append(warnings, notAdvanced(missed))
	}

	return warnings
}

func notAdvanced(missed provision.Missed) domain.Warning {
	subject := missed.Subject
	switch {
	case missed.Dirty:
		return domain.Warning{
			Code:    "create.ff_dirty",
			Message: "couldn't fast-forward " + subject + ": its worktree has uncommitted changes — it starts from your local branch",
			Params:  domain.Params{"branch": subject},
		}
	case missed.Busy:
		return domain.Warning{
			Code:    "create.ff_busy",
			Message: "couldn't fast-forward " + subject + ": its worktree is in the middle of a rebase or a bisect — it starts from your local branch",
			Params:  domain.Params{"branch": subject},
		}
	}

	return domain.Warning{
		Code:    "create.ff_failed",
		Message: "couldn't fast-forward " + subject + ": " + missed.Err.Error() + " — it starts from your local branch",
		Params:  domain.Params{"branch": subject, "cause": missed.Err.Error()},
	}
}

// item provisions one planned branch between its unit events; a worktree already open is reported as it stands, and a branch reached after a cancel is skipped.
func (u UseCase) item(ctx context.Context, plan Plan, planned BranchPlan, report domain.Report) Item {
	switch {
	case planned.Action == ActionUnchanged:
		return u.unchanged(planned)
	case ctx.Err() != nil:
		return Item{Subject: planned.Branch, Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted}
	}

	report(domain.UnitStarted{Subject: planned.Branch})
	outcome := u.Provision.Run(ctx, provision.ParamsOf(plan.facts.Repository, provision.Planned{Branch: planned.Branch, Path: planned.Path, Placement: planned.placement(), From: plan.From, Answers: plan.settled}, report))

	item := Item{Subject: planned.Branch, Status: outcome.Status, Reason: outcome.Reason, Err: outcome.Err, Warnings: outcome.Warnings}
	if outcome.Provisioned.Stands() {
		item.Detail = &Detail{
			Path:     planned.Path,
			Metadata: outcome.Provisioned.Meta,
			Standing: plan.facts.StandingOf(planned.Branch, planned.placement()),
			EnvPorts: outcome.Provisioned.EnvPorts,
		}
	}

	report(domain.UnitFinished{Subject: planned.Branch, Status: item.Status})
	return item
}

func (u UseCase) unchanged(planned BranchPlan) Item {
	// A record that cannot be read only leaves the detail's metadata empty: the worktree is there all the same.
	recorded, _ := u.Meta.Load(planned.Branch)
	return Item{Subject: planned.Branch, Status: domain.StatusDone, Reason: domain.ReasonUnchanged, Detail: &Detail{Path: planned.Path, Metadata: recorded}}
}

// remember writes the answers the plan asks to remember, once at least one worktree was created.
func (u UseCase) remember(ctx context.Context, plan Plan, items []Item) []domain.Warning {
	if !slices.ContainsFunc(items, created) {
		return nil
	}

	return u.Provision.Remember(ctx, Name, plan.settled, plan.Remember)
}

// ----------------------------------------------------------------------------
// Helpers

func created(item Item) bool {
	return item.Status == domain.StatusDone && item.Reason != domain.ReasonUnchanged
}
