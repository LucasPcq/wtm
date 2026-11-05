// Package rundown stops the jobs the run daemon runs in worktrees of a repository: which worktrees the request settles on, what the stop does to each job (stopped, a shared service released for the worktrees still holding it, a crash cleared), then each stop, the worktrees at once, and what became of each job.
package rundown

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell; its command line is `run down`.
const Name = domain.CommandRunDown

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// A UseCase stops the jobs of worktrees of one repository: the ports it reaches the repository and the run daemon through, and the services it shares with the other commands.
type UseCase struct {
	Git        Git
	Daemon     Daemon
	Repository repository.Service
	Operations operation.Service
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied (a dry run, or a refusal before the apply).
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: one Item per worktree, in the plan's order, and what follows.
type Result = domain.Result[Detail]

// An Item is what became of the stop in one worktree: its status and why, the error it failed on, and its Detail.
type Item = domain.Unit[Detail]

// A Detail is, as the JSON envelope reports it, the stop in one worktree: where, and each job it reached with what became of it.
type Detail struct {
	Path string      `json:"path"`
	Jobs []JobResult `json:"jobs"`
}

// A JobResult is what became of one job: its status and, for a stop that failed, the daemon's reason.
type JobResult struct {
	Name    string    `json:"name"`
	Status  JobStatus `json:"status"`
	Message string    `json:"message,omitempty"`
}

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

	// 2. Plan; the worktrees defaulting to the caller's, or all of them, are planned again once their locks are held.
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

// Plan decides what the request stops as the repository and the run daemon stand, changing nothing: the plan and its warnings, or the refusal that stops the request (a *domain.Error, field errors included).
func (u UseCase) Plan(ctx context.Context, req Request) (Plan, []domain.Warning, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Plan{}, nil, err
	}
	// A daemon that died leaving processes in its index is started to take them over: they are what run down must reach. The form never starts one.
	if facts.Unknown {
		if err := u.Daemon.Ensure(ctx); err != nil {
			return Plan{}, nil, u.daemonFailure(err, "couldn't start the run daemon to reach the jobs its index holds")
		}
		if facts, err = u.Observe(ctx, req); err != nil {
			return Plan{}, nil, err
		}
		if facts.Unknown {
			return Plan{}, nil, &domain.Error{Kind: domain.KindInternal, Code: CodeDaemonUnavailable, Message: "the run daemon started cannot tell which of the jobs its index holds are up"}
		}
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
const CodeStopFailed = Name + ".stop_failed"

// Apply stops each worktree's jobs, the worktrees at once, each between its unit's events: its own jobs, then its holds on shared services, as the daemon's own stop of a worktree does; a worktree that fails stops none of the others. Once ctx is cancelled, the stops not sent are not reached. The items keep the plan's order.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	// The worktrees report from several goroutines; a surface draws one event at a time.
	var mu sync.Mutex
	serial := func(event domain.Event) {
		mu.Lock()
		defer mu.Unlock()
		report(event)
	}

	items := make([]Item, len(plan.Worktrees))
	var wg sync.WaitGroup
	for i, t := range plan.Worktrees {
		wg.Go(func() { items[i] = u.stop(ctx, plan, t, serial) })
	}
	wg.Wait()

	items = plan.settled(items)
	return Result{Items: items, FollowUps: plan.next(items)}
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form the wizard asks for the request: the worktrees it may stop the jobs of, the profiles, and the request's problems as field errors.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// CodeDaemonUnavailable refuses a stop whose jobs cannot be read: the run daemon answers in a protocol this wtm does not read, or one that died left processes in its index and no daemon could be started to take them over.
const CodeDaemonUnavailable = Name + ".daemon_unavailable"

// Observe reads what run down decides from: the project's config and its run.toml (one that does not load is no refusal: what runs stays stoppable), the repository's snapshot, which worktrees' directory is gone, where the caller stands, where each name asked as a path leads, and the jobs the run daemon runs, never starting it: with none answering and processes in its index, the jobs are Unknown. A repository with no wtm config is refused with CodeConfigNotFound.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	facts := Facts{Repository: repo, Gone: map[string]bool{}}
	if facts.Located, err = u.locate(ctx, facts, req); err != nil {
		return Facts{}, err
	}
	for _, w := range repo.Checkouts() {
		if facts.Gone[w.Path], err = repository.Gone(ctx, u.Git, w); err != nil {
			return Facts{}, repository.Interrupted(ctx, err)
		}
	}

	cwd, err := repository.Located(ctx, u.Git, req.Caller.Cwd, domain.Caller{})
	if err != nil {
		return Facts{}, err
	}
	facts.Cwd = cmp.Or(cwd, req.Caller.Cwd)

	facts.Jobs, err = u.Daemon.Jobs(ctx)
	if skewed, ok := errors.AsType[*run.Skewed](err); ok {
		facts.Skewed, err = skewed, nil
	}
	if errors.Is(err, run.ErrUnknown) {
		facts.Unknown, err = true, nil
	}
	if err != nil {
		return Facts{}, u.daemonFailure(err, "couldn't read the jobs the run daemon runs")
	}
	return facts, ctx.Err()
}

// daemonFailure is why the jobs cannot be reached: a daemon of another protocol, or one holding jobs that cannot be handed over as it stands, followed by its restart; else the daemon unavailable, saying message.
func (u UseCase) daemonFailure(err error, message string) *domain.Error {
	if refusal := repository.SkewRefusal(repository.SkewRefusalParams{Command: Name, Undone: "nothing stopped", Err: err, Restart: u.Repository.Restart}); refusal != nil {
		return refusal
	}
	return domain.AsError(domain.AsErrorParams{Err: err, Code: CodeDaemonUnavailable, Message: message})
}

// locate resolves each name req asks that is no worktree's name as the path it stands for from where the caller stands, whether or not something is there (a worktree removed), spelled as git spells the worktree root it is.
func (u UseCase) locate(ctx context.Context, facts Facts, req Request) (map[string]string, error) {
	located := map[string]string{}
	for _, name := range req.Worktrees {
		if strings.TrimSpace(name) == "" || slices.ContainsFunc(facts.Worktrees, func(w worktree.Worktree) bool { return w.Name() == name }) {
			continue
		}

		location, err := repository.Resolved(ctx, u.Git, name, req.Caller)
		if err != nil {
			return nil, err
		}
		if located[name], err = repository.Root(ctx, u.Git, facts.Worktrees, location.Path); err != nil {
			return nil, err
		}
	}

	return located, nil
}

// ----------------------------------------------------------------------------
// The steps of Apply

// stop stops the jobs of t one after the other between its unit's events, each between its phase's events: its own, then its holds on shared services; a worktree reaching no job is unchanged, with no event.
func (u UseCase) stop(ctx context.Context, plan Plan, t Target, report domain.Report) Item {
	item := Item{Subject: t.Worktree, Detail: t.notReached()}
	switch {
	case ctx.Err() != nil:
		item.Status, item.Reason = domain.StatusSkipped, domain.ReasonNotReached
		return item
	case len(t.Jobs) == 0:
		item.Status, item.Reason = domain.StatusDone, domain.ReasonUnchanged
		return item
	}

	report(domain.UnitStarted{Subject: t.Worktree})
	defer func() { report(domain.UnitFinished{Subject: item.Subject, Status: item.Status}) }()

	// Its own jobs first, then its holds: whether a shared service stops depends on the holds the others leave.
	for _, shared := range []bool{false, true} {
		for i, job := range t.Jobs {
			if job.shared() == shared {
				item.Detail.Jobs[i] = u.stopJob(ctx, stopParams{Plan: plan, Target: t, Job: job, Report: report})
			}
		}
	}

	item = finished(item)
	return item
}

// stopParams are a job of a worktree to stop, the plan it is part of, and where its events go.
type stopParams struct {
	Plan   Plan
	Target Target
	Job    JobPlan
	Report domain.Report
}

// stopJob stops one job between its phase's events, and says what became of it.
func (u UseCase) stopJob(ctx context.Context, params stopParams) JobResult {
	t, job := params.Target, params.Job
	if ctx.Err() != nil {
		return JobResult{Name: job.Name, Status: JobNotReached}
	}

	params.Report(domain.PhaseStarted{Subject: t.Worktree, Phase: domain.PhaseJob, Job: job.Name})
	// A stop sent is let finish: a stack stopped half-way is worse than one stopped or not.
	outcome, err := u.Daemon.Stop(context.WithoutCancel(ctx), run.Stop{Name: job.Name, WorkDir: t.Path, CorrelationID: params.Plan.correlationID})
	params.Report(domain.PhaseFinished{Subject: t.Worktree, Phase: domain.PhaseJob, Job: job.Name, Failed: err != nil})
	if err != nil {
		return JobResult{Name: job.Name, Status: JobFailed, Message: err.Error()}
	}
	return JobResult{Name: job.Name, Status: statusOf(job.Fate, outcome)}
}

// finished is item once each of its stops ended: failed naming the jobs that would not stop, cancelled when the run was cut short before every stop was sent, else done.
func finished(item Item) Item {
	var failed, reasons []string
	for _, job := range item.Detail.Jobs {
		if job.Status == JobFailed {
			failed = append(failed, job.Name)
			reasons = append(reasons, job.Name+": "+job.Message)
		}
	}
	switch {
	case len(failed) > 0:
		item.Status, item.Err = domain.StatusFailed, &domain.Error{Kind: domain.KindInternal, Code: CodeStopFailed, Message: "couldn't stop " + strings.Join(reasons, "; "), Params: domain.Params{"jobs": strings.Join(failed, ", ")}}
	case slices.ContainsFunc(item.Detail.Jobs, func(job JobResult) bool { return job.Status == JobNotReached }):
		item.Status, item.Reason = domain.StatusCancelled, domain.ReasonInterrupted
	default:
		item.Status = domain.StatusDone
	}
	return item
}
