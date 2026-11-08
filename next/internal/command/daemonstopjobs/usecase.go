// Package daemonstopjobs stops every job wtm's daemon runs, in every repository of the machine or in one: each worktree's own jobs, its stacks by their stop command, its holds on shared services, a crashed job cleared; the daemon and its proxy stay up.
package daemonstopjobs

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell; its command line is `daemon stop-jobs`.
const Name = "daemon-stop-jobs"

// A UseCase stops the jobs of wtm's daemon through Daemon, under Operations, a repository asked by a path in it read through RepoRoot.
type UseCase struct {
	Operations operation.Service
	Daemon     Daemon
	RepoRoot   RepoRoot
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied.
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: one Item per worktree, in the plan's order, and what follows.
type Result = domain.Result[Detail]

// An Item is what became of the stop in one worktree.
type Item = domain.Unit[Detail]

// A Detail is the stop in one worktree (runjobs.StopDetail).
type Detail = runjobs.StopDetail

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: the daemon is the machine's, so it takes no lock (shortcut: a stop-jobs and a run up of one worktree do not take turns; lock each worktree once the locks span repositories); it plans, fingerprints the plan, stops on a dry run, refuses a stale plan and applies it between the operation's started and finished events. An error returned is a refusal before the apply; a stop that fails is in the Outcome's Result.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Begin the operation; the daemon is no repository's, so it takes no lock.
	op, err := u.Operations.Begin(Name, nil, opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Plan.
	plan, warnings, err := u.Plan(ctx, req)
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

	// 5. Apply between the started and finished events; the daemon's job.stopped events carry the caller's correlation ID.
	plan.correlationID = opts.CorrelationID
	report := op.Started()
	result := u.Apply(ctx, plan, report)
	op.Finished(result.Failure())

	outcome.Result = &result
	outcome.Warnings = op.Warnings(outcome.Warnings)
	return outcome, nil
}

// ----------------------------------------------------------------------------
// Plan

// Plan decides what the request stops as the daemon stands, changing nothing: the plan and its warnings, or the refusal that stops the request.
func (u UseCase) Plan(ctx context.Context, req Request) (Plan, []domain.Warning, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Plan{}, nil, err
	}
	if questions := NewQuestions(facts, req); len(questions.Errors) > 0 {
		return Plan{}, nil, domain.Invalid(questions.Errors)
	}

	return Decide(facts, req)
}

// ----------------------------------------------------------------------------
// Apply

// Apply stops each worktree's jobs (runjobs.Stop: the worktrees at once, each between its unit's events, its own jobs then its holds on shared services, each between its phase's events), then follows the ones that did not stop with the same request again.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	items := runjobs.Stop(ctx, u.Daemon, runjobs.StopParams{Command: Name, Targets: plan.targets(), CorrelationID: plan.correlationID, Report: report})
	return Result{Items: items, FollowUps: plan.next(items)}
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form the request is asked through: the repository, and its problem as a field error.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Questions{}, err
	}
	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what stop-jobs decides from: the daemon as it stands, never starting it, and the repository the request names, by its main checkout.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	daemon, err := u.Daemon.Status(ctx)
	if err != nil {
		return Facts{}, err
	}
	facts := Facts{Daemon: daemon}
	if req.Repo != "" {
		facts.Repo, facts.RepoErr = u.RepoRoot(ctx, req.path())
	}
	return facts, nil
}
