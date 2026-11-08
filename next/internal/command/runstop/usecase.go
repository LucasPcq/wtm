// Package runstop stops one job wtm's daemon runs in worktrees of a repository: which worktrees and which job the request settles on, what the stop does to the job in each (stopped, a shared service released for the worktrees still holding it, a crash cleared, not running), then each stop, the worktrees at once, and what became of it.
package runstop

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell; its command line is `run stop`.
const Name = domain.CommandRunStop

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// A UseCase stops one job in worktrees of one repository: the ports it reaches the repository and wtm's daemon through, and the services it shares with the other commands.
type UseCase struct {
	Repository repository.Service
	Operations operation.Service
	Stops      runjobs.Stops
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied (a dry run, or a refusal before the apply).
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: one Item per worktree, in the plan's order, and what follows.
type Result = domain.Result[Detail]

// An Item is what became of the stop in one worktree: its status and why, the error it failed on, and its Detail.
type Item = domain.Unit[Detail]

// A Detail is, as the JSON envelope reports it, the stop in one worktree (runjobs.StopDetail).
type Detail = runjobs.StopDetail

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: it takes the locks (none on a dry run, which plans once), plans under them, fingerprints the plan, stops on a dry run, refuses a stale plan and applies it between the operation's started and finished events. An error returned is a refusal before the apply; a stop that fails is in the Outcome's Result.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Begin the operation, which takes the request's locks before anything is read; a dry run takes none.
	op, err := u.Operations.Begin(Name, req.Locks(), opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Plan; the worktrees defaulting to the caller's, or named by a path, are planned again once their locks are held.
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

	// 5. Apply between the started and finished events other processes listen to; the daemon's job.stopped events carry the caller's correlation ID.
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

// Plan decides what the request stops as the repository and wtm's daemon stand, changing nothing: the plan and its warnings, or the refusal that stops the request (a *domain.Error, field errors included).
func (u UseCase) Plan(ctx context.Context, req Request) (Plan, []domain.Warning, error) {
	facts, err := u.observe(ctx, req, u.Stops.Reach)
	if err != nil {
		return Plan{}, nil, err
	}

	resolved, err := Resolve(facts, req)
	if err != nil {
		return Plan{}, nil, err
	}

	planned, warnings := Decide(facts, resolved, DecideParams{Restart: u.Repository.Restart})
	planned.Recap, planned.Empty = planned.recap(), planned.unchanged()
	return planned, warnings, nil
}

// ----------------------------------------------------------------------------
// Apply

// CodeStopFailed fails a worktree a job of which the daemon could not stop.
const CodeStopFailed = Name + runjobs.StopFailed

// Apply stops the job in each worktree (runjobs.Stop: the worktrees at once, each between its unit's and its phase's events); a worktree it is not up in is unchanged, saying so. The items keep the plan's order.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	items := runjobs.Stop(ctx, u.Stops.Daemon, runjobs.StopParams{Command: Name, Targets: plan.Worktrees, CorrelationID: plan.correlationID, Report: report})
	for i := range items {
		items[i] = plan.notRunning(items[i])
	}
	return Result{Items: items, FollowUps: plan.next(items)}
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form the wizard asks for the request: the worktrees it may stop the job in, the jobs, and the request's problems as field errors.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// CodeDaemonUnavailable refuses a stop whose jobs cannot be read: wtm's daemon answers in a protocol this wtm does not read, or one that died left processes in its index and no daemon could be started to take them over.
const CodeDaemonUnavailable = Name + runjobs.DaemonUnavailable

// Observe reads what run stop decides from: the project's config and its run.toml (one that does not load is no refusal: what runs stays stoppable, a job taken by its name), the repository's snapshot, which worktrees' directory is gone, where the caller stands, where each name asked as a path leads, and the jobs wtm's daemon runs, never starting it: with none answering and processes in its index, the jobs are Unknown. A repository with no wtm config is refused with CodeConfigNotFound.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	return u.observe(ctx, req, u.Stops.Observe)
}

// observe reads the repository, then what a stop decides from through read: Observe for a form, Reach for a plan.
func (u UseCase) observe(ctx context.Context, req Request, read func(context.Context, runjobs.ObserveStopParams) (runjobs.Running, error)) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	running, err := read(ctx, runjobs.ObserveStopParams{Command: Name, Repository: repo, Names: req.Worktrees, Caller: req.Caller, Restart: u.Repository.Restart})
	return Facts{Running: running}, err
}
