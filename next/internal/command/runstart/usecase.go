// Package runstart starts one job of run.toml in a worktree of a repository, or starts it again: which worktree and which job the request settles on, what the other worktrees' jobs become, then the job through wtm's daemon (service/runjobs), stopped first for a restart, and what it became.
package runstart

import (
	"context"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell; its command line is `run start`.
const Name = domain.CommandRunStart

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// A UseCase starts one job in a worktree of one repository: the meta records it reads the ordinal a start numbered with from, and the services it shares with the other commands, the start of jobs among them.
type UseCase struct {
	Meta       Meta
	Repository repository.Service
	Operations operation.Service
	Teardown   teardown.Service
	Jobs       runjobs.Service
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied (a dry run, or a refusal before the apply).
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: one Item, the worktree's, then one per drop owed it planned to pay, and what follows.
type Result = domain.Result[Detail]

// An Item is what became of the start in the worktree, or of one drop owed: its status and why, the error it failed on, its warnings, and its Detail once it began, or the debt.
type Item = domain.Unit[Detail]

// A Detail is, as the JSON envelope reports it, the start in the worktree: where, and the job with what became of it; or a drop owed the start paid (Owed), its item's subject the namespace, its jobs none.
type Detail struct {
	Path string              `json:"path,omitempty"`
	Jobs []runjobs.JobResult `json:"jobs"`
	Owed *teardown.Debt      `json:"owed,omitempty"`
}

// A JobResult is what became of the job (runjobs.JobResult), named here for run start's cli.
type JobResult = runjobs.JobResult

// JobStatus is what became of the job (runjobs.JobStatus), named here for run start's cli.
type JobStatus = runjobs.JobStatus

// The statuses of the job, as runjobs gives them.
const (
	JobStarted        = runjobs.JobStarted
	JobAlreadyRunning = runjobs.JobAlreadyRunning
	JobJoined         = runjobs.JobJoined
	JobDone           = runjobs.JobDone
	JobFailed         = runjobs.JobFailed
	JobCrashed        = runjobs.JobCrashed
	JobNotStarted     = runjobs.JobNotStarted
	JobStopped        = runjobs.JobStopped
)

// Reached are the jobs of the start that answer at a URL: the one it started or found running.
func (d Detail) Reached() []JobResult {
	return slices.DeleteFunc(slices.Clone(d.Jobs), func(job JobResult) bool {
		return job.URL == "" || (job.Status != JobStarted && job.Status != JobAlreadyRunning)
	})
}

// LeftUp reports whether the start left its job up.
func (d Detail) LeftUp() bool {
	return slices.ContainsFunc(d.Jobs, func(job runjobs.JobResult) bool {
		return job.Status == runjobs.JobStarted || job.Status == runjobs.JobAlreadyRunning || job.Status == runjobs.JobJoined
	})
}

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: it takes the locks (none on a dry run, which plans once), plans under them, fingerprints the plan, stops on a dry run, refuses a stale plan and applies it between the operation's started and finished events. An error returned is a refusal before the apply; a job that fails is in the Outcome's Result.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Begin the operation, which takes the request's locks before anything is read; a dry run takes none.
	op, err := u.Operations.Begin(Name, req.Locks(), opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Plan; a worktree defaulting to the caller's, or named by its path, is planned again once its lock is held.
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

	// 5. Apply between the started and finished events other processes listen to; the job's own events carry the caller's correlation ID.
	plan.correlationID = opts.CorrelationID
	report := op.Started()
	result := u.Apply(ctx, plan, report)
	op.Finished(result.Failure())

	applied := u.renumbered(plan)
	applied.Recap = applied.recap()
	outcome.Result, outcome.Plan = &result, applied
	outcome.Warnings = op.Warnings(append(outcome.Warnings, result.Warnings...))
	return outcome, nil
}

// ----------------------------------------------------------------------------
// Plan

// Plan decides what the request starts as the repository and wtm's daemon stand, changing nothing: the plan and its warnings, or the refusal that stops the request (a *domain.Error, field errors included).
func (u UseCase) Plan(ctx context.Context, req Request) (Plan, []domain.Warning, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Plan{}, nil, err
	}

	resolved, err := Resolve(facts, req)
	if err != nil {
		return Plan{}, nil, err
	}

	planned, warnings, err := Decide(facts, resolved, DecideParams{Restart: u.Repository.Restart})
	if err != nil {
		return Plan{}, nil, err
	}
	planned.Recap, planned.Empty = planned.recap(), planned.unchanged()
	return planned, warnings, nil
}

// ----------------------------------------------------------------------------
// Apply

// CodeDaemonUnavailable fails a start that could not begin: wtm's daemon of this build could not be made sure of.
const CodeDaemonUnavailable = Name + runjobs.DaemonUnavailable

// CodeStopFailed warns of a worktree whose jobs an exclusive start could not stop.
const CodeStopFailed = Name + runjobs.StopFailed

// The codes of what becomes of the worktree: it could not be numbered, its environment could not be read, the job failed to start, to stop before its restart, or ran as a task that failed, a service crashed once started.
const (
	CodeUnnumbered     = Name + runjobs.Unnumbered
	CodeEnvUnavailable = Name + runjobs.EnvUnavailable
	CodeJobFailed      = Name + runjobs.JobFailure
	CodeCrashed        = Name + runjobs.Crashed
)

// The codes of the warnings of the worktree: a port nothing answered on, a namespace its record could not keep.
const (
	CodePortSilent          = Name + runjobs.PortSilent
	CodeNamespaceUnrecorded = Name + runjobs.NamespaceUnrecorded
)

// Apply makes sure of a daemon of this build, stops the other worktrees' jobs when the plan says so, then starts the job (runjobs.Service.Start: a restart stops it first, within its phase, and its log continues behind the run mark). Then the drops owed the plan pays are made (teardown.Service.Pay). Once ctx is cancelled, what the daemon took keeps running.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	items := make([]Item, len(plan.Worktrees))
	if failure := u.Jobs.Ensure(ctx, Name, u.Repository.Restart); failure != nil {
		for i, t := range plan.Worktrees {
			items[i] = Item{Subject: t.Worktree, Status: domain.StatusFailed, Err: failure, Detail: notStarted(t)}
		}
		// No job started: the drops owed are listed unpaid.
		return Result{Items: append(items, paidItems(teardown.Unpaid(plan.Owed))...)}
	}

	warnings := u.Jobs.StopOthers(ctx, runjobs.StopOthersParams{Command: Name, Others: plan.Others, CorrelationID: plan.correlationID})
	for i, t := range plan.Worktrees {
		started := u.Jobs.Start(ctx, runjobs.StartParams{Command: Name, Facts: plan.facts.Facts, Target: t, Restart: plan.Restart, NoProbe: plan.NoProbe, CorrelationID: plan.correlationID, Report: report})
		items[i] = Item{Subject: t.Worktree, Status: started.Status, Reason: started.Reason, Err: started.Err, Warnings: started.Warnings, Detail: &Detail{Path: t.Path, Jobs: started.Jobs}}
	}

	// A shared service this start brought up takes back what removed worktrees owe it.
	paid, withdrawn := u.Teardown.Pay(ctx, teardown.PayParams{Command: Name, Facts: plan.facts.teardown(), Debts: plan.Owed})
	items = append(items, paidItems(paid)...)
	return Result{Items: items, Warnings: append(warnings, withdrawn...), FollowUps: plan.next(items)}
}

// paidItems are the drops owed a start paid, an item each, its subject the namespace.
func paidItems(paid []teardown.Paid) []Item {
	items := make([]Item, 0, len(paid))
	for _, p := range paid {
		items = append(items, Item{Subject: p.Debt.Name, Status: p.Status, Reason: p.Reason, Err: p.Err, Warnings: p.Warnings, Detail: &Detail{Jobs: []runjobs.JobResult{}, Owed: &p.Debt}})
	}
	return items
}

// renumbered is plan with the worktree the start numbered planned again from the ordinal it now records, so the outcome's plan names the ports and namespaces its detail reports.
func (u UseCase) renumbered(plan Plan) Plan {
	worktrees := slices.Clone(plan.Worktrees)
	for i, t := range worktrees {
		if t.Numbered {
			continue
		}
		record, err := u.Meta.Load(t.Branch)
		if err != nil || record.Ordinal == 0 {
			continue
		}
		t.Numbered = true
		job, _ := plan.facts.Declared(plan.Job)
		worktrees[i] = plan.facts.restarting(plan.facts.Planned(t, []project.Job{job}, record.Ordinal), plan.asked)
	}
	plan.Worktrees = worktrees
	return plan
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form the wizard asks for the request: the worktrees it may start in, the jobs, whether the others' jobs stop first, whether a job that changes foreign data starts, and the request's problems as field errors.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// The codes of a repository run start cannot start in: run.toml does not exist (followed by run init), or does not load.
const (
	CodeNotInitialized = Name + runjobs.NotInitialized
	CodeInit           = Name + runjobs.RunInit
	CodeRunInvalid     = Name + runjobs.RunInvalid
)

// Observe reads what run start decides from: the project's config and its run.toml, the repository's snapshot, and what a start decides from besides (runjobs.Service.Observe), never starting wtm's daemon (B12). A repository with no wtm config is refused with CodeConfigNotFound, one with no run.toml with CodeNotInitialized.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	facts, err := u.Jobs.Observe(ctx, runjobs.ObserveParams{Command: Name, Repository: repo, Cwd: req.Caller.Cwd})
	return Facts{Facts: facts}, err
}
