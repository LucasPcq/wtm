// Package extract moves a subset of a worktree's uncommitted changes to another worktree, existing or created for them: what the request settles on, what conflicts, then the target made and the changes moved.
package extract

import (
	"context"
	"errors"
	"fmt"
	"io/fs"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = "extract"

// The codes extract refuses or fails with that a caller may act on.
const (
	CodeConfigNotFound = Name + repository.ConfigNotFound // no wtm config: `wtm init` has not run in the repository
	CodeConflict       = "extract.conflict"               // the changes conflict with the target and the request does not resolve them
	CodeHalfMoved      = "extract.half_moved"             // the changes reached the target but could not be removed from the source
)

// A UseCase moves changes between the worktrees of one repository: the ports it reaches the repository through and the services it shares with the other commands.
type UseCase struct {
	Git        Git
	Meta       Meta
	Shield     Shield
	Repository repository.Service
	Operations operation.Service
	Provision  provision.Service
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied (a dry run, nothing to extract, or a refusal before the apply).
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: the one Item of the target and the warnings of the whole run.
type Result = domain.Result[Detail]

// An Item is what became of the target: its status and why, the error it failed on, its warnings, and its Detail once its worktree stands.
type Item = domain.Unit[Detail]

// A Detail describes the target as the JSON envelope reports it: where it is, the files moved or copied there, the files left with conflict markers, its isolation, the env ports a created target was given and where its branch stood against origin.
type Detail struct {
	Path      string            `json:"path"`
	Files     []worktree.Change `json:"files"`
	Kept      bool              `json:"kept"` // the changes were copied: the source still holds them
	Conflicts []string          `json:"conflicts"`
	Isolation project.Isolation `json:"isolation,omitempty"`
	EnvPorts  env.Redacted      `json:"env_ports,omitzero"`
	worktree.Standing
}

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: it takes the locks, plans under them, fingerprints the plan, stops on a dry run, refuses a stale plan, stops when there is nothing to extract and applies it between the operation's started and finished events. An error returned is a refusal before the apply; a failure during the apply is in the Outcome's Result.
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

	// 4. A preview made on other facts is refused rather than applied, even when nothing is left to extract.
	if err := op.Expect(outcome.Fingerprint); err != nil {
		return outcome, err
	}

	// 5. A source with no uncommitted change is not a failure: the run ends with no apply and no events, nothing created.
	if plan.Nothing() {
		return outcome, nil
	}

	// 6. Apply between the started and finished events other processes listen to.
	report := op.Started()
	result := u.Apply(ctx, plan, report)
	op.Finished(result.Failure())

	outcome.Result = &result
	outcome.Warnings = op.Warnings(append(outcome.Warnings, result.Warnings...))
	return outcome, nil
}

// ----------------------------------------------------------------------------
// Plan

// Plan decides what the request does to the repository as it stands, changing nothing: the plan and its warnings, or the refusal that stops the request (a *domain.Error, field errors and conflicts with CodeConflict included).
func (u UseCase) Plan(ctx context.Context, req Request) (Plan, []domain.Warning, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Plan{}, nil, err
	}

	resolved, err := Resolve(facts, req)
	if err != nil {
		return Plan{}, nil, err
	}

	// Conflicts depend on the resolved target and files, so they are read only once the request is settled.
	conflicts, err := u.Git.Conflicts(ctx, facts.conflictCheck(resolved))
	if err != nil {
		return Plan{}, nil, err
	}

	return Decide(facts, resolved, conflicts)
}

// ----------------------------------------------------------------------------
// Apply

// Apply carries out the plan, reporting each step: it fast-forwards the source asked, makes the target when it does not exist, moves or copies the changes into it, and remembers the answers asked once the changes reached a target it created. Once ctx is cancelled, what is not yet started is skipped.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	warnings := u.fastForward(ctx, plan, report)
	if ctx.Err() != nil {
		return Result{Items: []Item{{Subject: plan.Target.Branch, Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted}}, Warnings: warnings}
	}

	report(domain.UnitStarted{Subject: plan.Target.Branch})
	item := u.target(ctx, plan, report)
	if item.Status == domain.StatusDone {
		item = u.move(ctx, plan, item, report)
	}
	report(domain.UnitFinished{Subject: item.Subject, Status: item.Status})

	if plan.Target.creates() && item.Status == domain.StatusDone {
		warnings = append(warnings, u.Provision.Remember(ctx, Name, plan.settled, plan.Remember)...)
	}

	return Result{Items: []Item{item}, Warnings: warnings}
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form the wizard asks for the request: each question with its options, its answer so far and where that answer comes from, and the request's problems as field errors. Once the request has no problem, the conflict question carries the conflicts the target would meet.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	questions := NewQuestions(facts, req)
	if len(questions.Errors) > 0 {
		return questions, nil
	}

	conflicts, err := u.Git.Conflicts(ctx, facts.conflictCheck(questions.Request()))
	if err != nil {
		return Questions{}, err
	}

	return questions.facing(conflicts), nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what extract decides from: the project's config, the repository's snapshot, the uncommitted changes of the source (of every worktree when none is given), the meta records of the source and target, and what lies at the path a new target would take. A repository with no wtm config is refused with CodeConfigNotFound.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	facts := Facts{Repository: repo}
	if facts.Changes, err = u.changes(ctx, facts.sources(req.Source)); err != nil {
		return Facts{}, err
	}
	if facts.Records, err = u.records(facts.checkedOut(req.Source, req.To)); err != nil {
		return Facts{}, err
	}
	if facts.Folder, err = u.locate(ctx, facts.newPath(req.To)); err != nil {
		return Facts{}, err
	}

	return facts, nil
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

// records loads the meta record of each branch; a branch with none has the zero record.
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

// ----------------------------------------------------------------------------
// The steps of Apply

// fastForward fast-forwards the source the plan names; one that cannot be is a warning, the target starting from the local branch.
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
			Code:    "extract.ff_dirty",
			Message: "couldn't fast-forward " + subject + ": its worktree has uncommitted changes — the target starts from your local branch",
			Params:  domain.Params{"branch": subject},
		}
	case missed.Busy:
		return domain.Warning{
			Code:    "extract.ff_busy",
			Message: "couldn't fast-forward " + subject + ": its worktree is in the middle of a rebase or a bisect — the target starts from your local branch",
			Params:  domain.Params{"branch": subject},
		}
	}

	return domain.Warning{
		Code:    "extract.ff_failed",
		Message: "couldn't fast-forward " + subject + ": " + missed.Err.Error() + " — the target starts from your local branch",
		Params:  domain.Params{"branch": subject, "cause": missed.Err.Error()},
	}
}

// target makes the target worktree whole when the plan creates it, or takes the existing one as it stands.
func (u UseCase) target(ctx context.Context, plan Plan, report domain.Report) Item {
	if !plan.Target.creates() {
		return Item{Subject: plan.Target.Branch, Status: domain.StatusDone, Detail: detail(plan.Target.Path, plan.facts.recordedIsolation(plan.Target.Branch))}
	}

	outcome := u.Provision.Run(ctx, provision.ParamsOf(plan.facts.Repository, provision.Planned{Branch: plan.Target.Branch, Path: plan.Target.Path, Placement: plan.Target.placement(), From: plan.From, Answers: plan.settled}, report))

	item := Item{Subject: plan.Target.Branch, Status: outcome.Status, Reason: outcome.Reason, Err: outcome.Err, Warnings: outcome.Warnings}
	if outcome.Provisioned.Stands() {
		item.Detail = detail(plan.Target.Path, outcome.Provisioned.Meta.Isolation)
		item.Detail.EnvPorts = outcome.Provisioned.EnvPorts
		item.Detail.Standing = plan.facts.StandingOf(plan.Target.Branch, plan.Target.placement())
	}

	return item
}

// move moves or copies the planned changes into the target, between the move phase's events.
func (u UseCase) move(ctx context.Context, plan Plan, item Item, report domain.Report) Item {
	if ctx.Err() != nil {
		item.Status, item.Reason = domain.StatusCancelled, domain.ReasonInterrupted
		return item
	}

	report(domain.PhaseStarted{Subject: item.Subject, Phase: "move"})
	// Shielded: a cancel arriving mid-move would leave the changes half in the source, half in the target.
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

// failure turns the error a move failed on into extract's: a conflict found at the last moment is refused with CodeConflict, changes left in both worktrees fail with CodeHalfMoved.
func (p Plan) failure(err error) *domain.Error {
	if refused, ok := errors.AsType[*worktree.ConflictRefusal](err); ok {
		return conflictRefusal(refused.Conflicts, p.Target.Branch)
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

// ----------------------------------------------------------------------------
// Helpers

// detail is the Detail of a target before anything is moved: empty lists rather than nil, so the JSON envelope shows [].
func detail(path string, isolation project.Isolation) *Detail {
	return &Detail{Path: path, Files: []worktree.Change{}, Conflicts: []string{}, Isolation: isolation}
}
