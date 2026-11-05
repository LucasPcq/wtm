// Package runup starts a profile of run.toml in worktrees of a repository, several at once: which worktrees and which profile the request settles on, what the other worktrees' jobs become, then each job in order through the run daemon (service/runjobs), and what each became.
package runup

import (
	"context"
	"slices"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell; its command line is `run up`.
const Name = domain.CommandRunUp

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// A UseCase starts a profile in worktrees of one repository: the meta records it reads the ordinal a start numbered with from, and the services it shares with the other commands, the start of jobs among them.
type UseCase struct {
	Meta       Meta
	Repository repository.Service
	Operations operation.Service
	Teardown   teardown.Service
	Jobs       runjobs.Service
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied (a dry run, or a refusal before the apply).
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: one Item per worktree, in the plan's order, then one per drop owed it planned to pay, and what follows.
type Result = domain.Result[Detail]

// An Item is what became of the start in one worktree, or of one drop owed: its status and why, the error it failed on, its warnings, and its Detail once it began, or the debt.
type Item = domain.Unit[Detail]

// A Detail is, as the JSON envelope reports it, the start in one worktree: where, the profile, and each job of the profile with what became of it; or a drop owed the start paid (Owed), its item's subject the namespace, its jobs none.
type Detail struct {
	Path    string         `json:"path,omitempty"`
	Profile string         `json:"profile"`
	Jobs    []JobResult    `json:"jobs"`
	Owed    *teardown.Debt `json:"owed,omitempty"`
}

// A JobResult is what became of one job of the start (runjobs.JobResult), named here for run up's cli.
type JobResult = runjobs.JobResult

// JobStatus is what became of one job of the start (runjobs.JobStatus), named here for run up's cli.
type JobStatus = runjobs.JobStatus

// The statuses of a job of the start, as runjobs gives them.
const (
	JobStarted        = runjobs.JobStarted
	JobAlreadyRunning = runjobs.JobAlreadyRunning
	JobJoined         = runjobs.JobJoined
	JobDone           = runjobs.JobDone
	JobFailed         = runjobs.JobFailed
	JobCrashed        = runjobs.JobCrashed
	JobNotStarted     = runjobs.JobNotStarted
)

// Reached are the jobs of the start that answer at a URL: those it started or found running.
func (d Detail) Reached() []JobResult {
	return slices.DeleteFunc(slices.Clone(d.Jobs), func(job JobResult) bool {
		return job.URL == "" || (job.Status != JobStarted && job.Status != JobAlreadyRunning)
	})
}

// LeftUp reports whether the start left a job up in its worktree.
func (d Detail) LeftUp() bool {
	return slices.ContainsFunc(d.Jobs, func(job JobResult) bool {
		return job.Status == JobStarted || job.Status == JobAlreadyRunning || job.Status == JobJoined
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

	// 2. Plan; the worktrees defaulting to the caller's are planned again once their locks are held.
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

	// 5. Apply between the started and finished events other processes listen to; the jobs' own events carry the caller's correlation ID.
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

// Plan decides what the request starts as the repository and the run daemon stand, changing nothing: the plan and its warnings, or the refusal that stops the request (a *domain.Error, field errors included).
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

// CodeDaemonUnavailable fails a start that could not begin: the run daemon of this build could not be made sure of.
const CodeDaemonUnavailable = Name + runjobs.DaemonUnavailable

// The codes of the drops removed worktrees owed, paid by a start: one withdrawn (a worktree reaches its namespace again), one that failed, one the queue could not give up or take back.
const (
	CodeOwedWithdrawn  = Name + teardown.OwedWithdrawn
	CodeDropFailed     = Name + teardown.DropFailed
	CodeOwedUnrecorded = Name + teardown.OwedUnrecorded
	CodeOwedLost       = Name + teardown.OwedLost
)

// CodeStopFailed warns of a worktree whose jobs an exclusive start could not stop.
const CodeStopFailed = Name + runjobs.StopFailed

// The codes of what becomes of a worktree: it could not be numbered, its environment could not be read, a job failed to start or a task failed, a service crashed once started.
const (
	CodeUnnumbered     = Name + runjobs.Unnumbered
	CodeEnvUnavailable = Name + runjobs.EnvUnavailable
	CodeJobFailed      = Name + runjobs.JobFailure
	CodeCrashed        = Name + runjobs.Crashed
)

// The codes of the warnings of a worktree: a port nothing answered on, a namespace its record could not keep.
const (
	CodePortSilent          = Name + runjobs.PortSilent
	CodeNamespaceUnrecorded = Name + runjobs.NamespaceUnrecorded
)

// Apply makes sure of a run daemon of this build, stops the other worktrees' jobs when the plan says so, then starts each worktree's jobs (runjobs.Service.Start), the worktrees at once, each one's in order; a worktree that fails stops none of the others. Then the drops owed the plan pays are made (teardown.Service.Pay). Once ctx is cancelled, the jobs the daemon took keep running and the jobs not reached are not started. The items keep the plan's order.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	// The worktrees report from several goroutines; a surface draws one event at a time.
	var mu sync.Mutex
	serial := func(event domain.Event) {
		mu.Lock()
		defer mu.Unlock()
		report(event)
	}

	items := make([]Item, len(plan.Worktrees))
	if failure := u.Jobs.Ensure(ctx, Name, u.Repository.Restart); failure != nil {
		for i, t := range plan.Worktrees {
			items[i] = Item{Subject: t.Worktree, Status: domain.StatusFailed, Err: failure, Detail: plan.notStarted(t)}
		}
		return Result{Items: append(items, paidItems(teardown.Unpaid(plan.Owed))...)}
	}

	warnings := u.Jobs.StopOthers(ctx, runjobs.StopOthersParams{Command: Name, Others: plan.Others, CorrelationID: plan.correlationID})

	var wg sync.WaitGroup
	for i, t := range plan.Worktrees {
		wg.Go(func() { items[i] = u.start(ctx, plan, t, serial) })
	}
	wg.Wait()

	// A shared service this start brought up takes back what removed worktrees owe it.
	paid, withdrawn := u.Teardown.Pay(ctx, teardown.PayParams{Command: Name, Facts: plan.facts.teardown(), Debts: plan.Owed})

	return Result{Items: append(items, paidItems(paid)...), Warnings: append(warnings, withdrawn...), FollowUps: plan.next(items)}
}

// start starts the profile's jobs in t, its item what became of them.
func (u UseCase) start(ctx context.Context, plan Plan, t runjobs.Target, report domain.Report) Item {
	started := u.Jobs.Start(ctx, runjobs.StartParams{Command: Name, Facts: plan.facts.Facts, Target: t, Profile: plan.Profile, NoProbe: plan.NoProbe, CorrelationID: plan.correlationID, Report: report})
	return Item{Subject: t.Worktree, Status: started.Status, Reason: started.Reason, Err: started.Err, Warnings: started.Warnings, Detail: &Detail{Path: t.Path, Profile: plan.Profile, Jobs: started.Jobs}}
}

// paidItems are the drops owed a start paid, an item each, its subject the namespace.
func paidItems(paid []teardown.Paid) []Item {
	items := make([]Item, 0, len(paid))
	for _, p := range paid {
		items = append(items, Item{Subject: p.Debt.Name, Status: p.Status, Reason: p.Reason, Err: p.Err, Warnings: p.Warnings, Detail: &Detail{Jobs: []JobResult{}, Owed: &p.Debt}})
	}
	return items
}

// renumbered is plan with each worktree the start numbered planned again from the ordinal it now records, so the outcome's plan names the ports and namespaces its detail reports.
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
		worktrees[i] = plan.facts.Planned(t, plan.facts.jobsOf(plan.Profile), record.Ordinal)
	}
	plan.Worktrees = worktrees
	return plan
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form the wizard asks for the request: the worktrees it may start in, the profiles, whether the others' jobs stop first, whether a job that changes foreign data starts, and the request's problems as field errors.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// The codes of a repository run up cannot start in: run.toml does not exist (followed by run init), or does not load.
const (
	CodeNotInitialized = Name + runjobs.NotInitialized
	CodeInit           = Name + runjobs.RunInit
	CodeRunInvalid     = Name + runjobs.RunInvalid
)

// Observe reads what run up decides from: the project's config and its run.toml, the repository's snapshot, and what a start decides from besides (runjobs.Service.Observe), never starting the run daemon. A repository with no wtm config is refused with CodeConfigNotFound, one with no run.toml with CodeNotInitialized.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	facts, err := u.Jobs.Observe(ctx, runjobs.ObserveParams{Command: Name, Repository: repo, Cwd: req.Caller.Cwd})
	return Facts{Facts: facts}, err
}
