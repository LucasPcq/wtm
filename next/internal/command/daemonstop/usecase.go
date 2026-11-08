// Package daemonstop stops wtm's daemon, the machine's one: what its shutdown does to each job it holds (the processes it runs stop, a shared job's own run among them; its stacks run on, taken over by the next daemon; the claims on a job that stops go with it), then the shutdown.
package daemonstop

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell; its command line is `daemon stop`.
const Name = "daemon-stop"

// A UseCase stops wtm's daemon through Daemon, under Operations.
type UseCase struct {
	Operations operation.Service
	Daemon     Daemon
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied.
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: one Item, the daemon.
type Result = domain.Result[Detail]

// An Item is what became of the daemon: its status, and the error its shutdown failed on.
type Item = domain.Unit[Detail]

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: the daemon is the machine's, so it takes no lock; it plans, fingerprints the plan, stops on a dry run, refuses a stale plan and applies it between the operation's started and finished events. An error returned is a refusal before the apply; a shutdown that fails is in the Outcome's Result.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Begin the operation; the daemon is no repository's, so it takes no lock.
	op, err := u.Operations.Begin(Name, nil, opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Plan.
	plan, err := u.Plan(ctx, req)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}

	// 3. Fingerprint what would be applied; a dry run ends here with the plan and its fingerprint.
	outcome := Outcome{Op: op.ID, Plan: plan}
	outcome.Fingerprint, err = operation.Fingerprint(plan.Fingerprinted())
	if err != nil || opts.DryRun {
		return outcome, err
	}

	// 4. A preview made on other facts is refused rather than applied.
	if err := op.Expect(outcome.Fingerprint); err != nil {
		return outcome, err
	}

	// 5. Apply between the started and finished events.
	report := op.Started()
	result := u.Apply(ctx, plan, report)
	op.Finished(result.Failure())

	outcome.Result = &result
	outcome.Warnings = op.Warnings(outcome.Warnings)
	return outcome, nil
}

// ----------------------------------------------------------------------------
// Plan

// Plan decides what stopping the daemon does as it stands, changing nothing.
func (u UseCase) Plan(ctx context.Context, _ Request) (Plan, error) {
	daemon, err := u.Daemon.Status(ctx)
	if err != nil {
		return Plan{}, err
	}

	return Decide(daemon), nil
}

// ----------------------------------------------------------------------------
// Apply

// CodeShutdownFailed fails a daemon that did not stop when asked.
const CodeShutdownFailed = Name + ".shutdown_failed"

// Apply asks the daemon to stop and waits for it to be gone, between its unit's events; a plan with no daemon running applies nothing.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	if !plan.Running {
		return Result{}
	}
	report(domain.UnitStarted{Subject: subject})
	item := Item{Subject: subject, Status: domain.StatusDone}
	if err := u.Daemon.Shutdown(ctx); err != nil {
		item.Status, item.Err = domain.StatusFailed, domain.AsError(domain.AsErrorParams{Err: err, Code: CodeShutdownFailed, Message: "couldn't stop wtm's daemon: " + err.Error()})
	}
	report(domain.UnitFinished{Subject: subject, Status: item.Status})
	return Result{Items: []Item{item}}
}

// ----------------------------------------------------------------------------
// Questions

// Questions is daemon stop's form: it asks nothing.
func (u UseCase) Questions(context.Context, Request) (Questions, error) {
	return Questions{}, nil
}
