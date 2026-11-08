// Package daemonrestart restarts wtm's daemon, the machine's one, as this build, its jobs brought back as pm2 does: the services it runs itself stop with it and start again on the new daemon from its index's record of how each was started (its profile, worktree, environment and addresses), a shared one with the worktrees that hold it; its stacks run on and the new daemon takes them over; a task is not run again, nor a job its index has no record of.
package daemonrestart

import (
	"context"
	"errors"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell; its command line is `daemon restart`.
const Name = "daemon-restart"

// A UseCase restarts wtm's daemon through Daemon, under Operations, its apply under Shield; Version is this wtm's build, the one the new daemon runs.
type UseCase struct {
	Operations operation.Service
	Daemon     Daemon
	Shield     Shield
	Exists     Exists
	Version    string
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied.
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: one Item, the daemon.
type Result = domain.Result[Detail]

// An Item is what became of the daemon: its status, the error it failed on, and its Detail.
type Item = domain.Unit[Detail]

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation, which takes the machine's lock; it plans, fingerprints the plan, stops on a dry run, refuses a stale plan and applies it between the operation's started and finished events. An error returned is a refusal before the apply; a daemon that did not start, or a job that did not come back, is in the Outcome's Result.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Begin the operation under the machine's lock: the daemon is no repository's.
	op, err := u.Operations.Begin(Name, domain.MachineLocks(), opts)
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

	// 5. Apply between the started and finished events; the jobs brought back carry the caller's correlation ID.
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

// Plan decides what the restart does as the daemon and its index stand, changing nothing: the plan and its warnings.
func (u UseCase) Plan(ctx context.Context, req Request) (Plan, []domain.Warning, error) {
	daemon, err := u.Daemon.Status(ctx)
	if err != nil {
		return Plan{}, nil, err
	}

	return Decide(Facts{Daemon: daemon, Records: u.Daemon.Records(), Version: u.Version, Exists: u.Exists}, req)
}

// ----------------------------------------------------------------------------
// Apply

// The codes of a restart that did not go through: the new daemon did not start, the old one did not stop, or a job did not come back.
const (
	CodeStartFailed    = Name + ".start_failed"
	CodeShutdownFailed = Name + ".shutdown_failed"
	CodeNotBack        = Name + ".not_back"
)

// Apply restarts the daemon between its unit's events: it stops the one running, starts one of this build, which takes the stacks over, then starts each job of the plan again from its record, each between its phase's events, the shared ones with their holders after the others; a start the new daemon finds already up is back.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	report(domain.UnitStarted{Subject: subject})
	item := u.restart(ctx, plan, report)
	report(domain.UnitFinished{Subject: subject, Status: item.Status})
	return Result{Items: []Item{item}}
}

func (u UseCase) restart(ctx context.Context, plan Plan, report domain.Report) Item {
	item := Item{Subject: subject, Status: domain.StatusDone, Detail: &Detail{Back: []Back{}}}
	// Shielded from the shutdown on: a daemon asked to stop stops whatever its client does, and a restart cut short would leave the jobs it stopped down, recorded nowhere.
	ctx, release := u.Shield(ctx)
	defer release()
	if plan.Running {
		// A daemon slow to stop is waited for by Ensure, which starts the next once the lock is free.
		if err := u.Daemon.Shutdown(ctx); err != nil && !isStopTimeout(err) {
			item.Status, item.Err = domain.StatusFailed, domain.AsError(domain.AsErrorParams{Err: err, Code: CodeShutdownFailed, Message: "couldn't stop wtm's daemon"})
			return item
		}
	}
	if err := u.Daemon.Ensure(ctx); err != nil {
		item.Status, item.Err = domain.StatusFailed, domain.AsError(domain.AsErrorParams{Err: err, Code: CodeStartFailed, Message: "the new daemon didn't start"})
		return item
	}

	for _, spec := range plan.starts {
		back := Back{Name: spec.Job.Name, Path: spec.WorkDir}
		report(domain.PhaseStarted{Subject: subject, Phase: domain.PhaseJob, Job: spec.Job.Name})
		spec.Origin = withCorrelation(spec.Origin, plan.correlationID)
		_, err := u.Daemon.Start(ctx, spec, func(line run.Line) {
			report(domain.Output{Subject: subject, Job: spec.Job.Name, Line: line.Text})
		})
		if refused, ok := errors.AsType[*run.Error](err); ok && refused.Code == run.CodeJobRunning {
			err = nil
		}
		report(domain.PhaseFinished{Subject: subject, Phase: domain.PhaseJob, Job: spec.Job.Name, Failed: err != nil})
		back.Back = err == nil
		if err != nil {
			back.Message = err.Error()
		}
		item.Detail.Back = append(item.Detail.Back, back)
	}
	return notBack(item)
}

// isStopTimeout reports whether err is a daemon that did not stop in time.
func isStopTimeout(err error) bool {
	refused, ok := errors.AsType[*run.Error](err)
	return ok && refused.Code == run.CodeStopTimeout
}

// withCorrelation is origin with the restart's correlation ID, nil left nil.
func withCorrelation(origin *run.Origin, id string) *run.Origin {
	if origin == nil {
		return nil
	}
	with := *origin
	with.CorrelationID = id
	return &with
}

// ----------------------------------------------------------------------------
// Questions

// Questions is daemon restart's form: it asks nothing.
func (u UseCase) Questions(_ context.Context, req Request) (Questions, error) {
	return Questions{Request: req}, nil
}
