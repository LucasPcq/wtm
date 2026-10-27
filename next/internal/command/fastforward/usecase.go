// Package fastforward brings worktrees' branches up to origin's, never merging: one fetch of origin, then each worktree behind it moved in turn, those it cannot move safely refused when named and left as they are when the command chose them.
package fastforward

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = domain.CommandFastForward

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// A UseCase fast-forwards worktrees of one repository: the ports it reaches the repository through and the services it shares with the other commands.
type UseCase struct {
	Git        Git
	Shield     Shield
	Repository repository.Service
	Operations operation.Service
	Provision  provision.Service
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied (a dry run, or a refusal before the apply).
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: one Item per worktree asked, in the plan's order.
type Result = domain.Result[Detail]

// An Item is what became of one worktree asked: its status and why, the error it failed on, and its Detail once it moved.
type Item = domain.Unit[Detail]

// A Detail describes a worktree fast-forwarded as the JSON envelope reports it: where it is, and the commits its branch moved from and to.
type Detail struct {
	Path string `json:"path"`
	From string `json:"from"`
	To   string `json:"to"`
}

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: it takes the locks (none on a dry run, which plans once), fetches origin, plans under them, fingerprints the plan, stops on a dry run, refuses a stale plan and applies it between the operation's started and finished events. An error returned is a refusal before the apply, an unsafe worktree named among them; a failure during the apply is in the Outcome's Result.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Begin the operation, which takes the request's locks before anything is read, so the facts cannot change under the plan; a dry run takes none.
	op, err := u.Operations.Begin(Name, req.Locks(), opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Fetch origin once, so each worktree stands where origin has it now; a fetch that fails leaves the plan on origin as last fetched, said in a warning.
	fetched := u.Git.Fetch(ctx)
	if fetched != nil && ctx.Err() != nil {
		return Outcome{Op: op.ID}, ctx.Err()
	}

	// 3. Plan; a plan moving a worktree the request did not name, one the command chose, is planned again once that lock is held.
	plan, warnings, err := operation.PlanUnderLocks(ctx, op, func(ctx context.Context) (Plan, []domain.Warning, error) {
		return u.Plan(ctx, req, fetched)
	})
	if err != nil {
		return Outcome{Op: op.ID}, err
	}

	// 4. Fingerprint what would be applied; a dry run ends here with the plan and its fingerprint.
	outcome := Outcome{Op: op.ID, Plan: plan, Warnings: warnings}
	outcome.Fingerprint, err = operation.Fingerprint(plan.Fingerprinted())
	if err != nil || opts.DryRun {
		return outcome, err
	}

	// 5. A preview made on other facts, origin moved since among them, is refused rather than applied.
	if err := op.Expect(outcome.Fingerprint); err != nil {
		return outcome, err
	}

	// 6. Apply between the started and finished events other processes listen to.
	report := op.Started()
	result := u.Apply(ctx, plan, report)
	op.Finished(result.Failure())

	outcome.Result = &result
	outcome.Warnings = op.Warnings(outcome.Warnings)
	return outcome, nil
}

// ----------------------------------------------------------------------------
// Plan

// Plan decides what the request does to the repository as origin was fetched last, fetched said why that fetch failed, nil when it did not: the plan and its warnings, or the refusal that stops the request (a *domain.Error, field errors and unsafe worktrees with CodeUnsafe included).
func (u UseCase) Plan(ctx context.Context, req Request, fetched error) (Plan, []domain.Warning, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Plan{}, nil, err
	}
	facts.FetchErr = fetched

	resolved, err := Resolve(facts, req)
	if err != nil {
		return Plan{}, nil, err
	}

	planned, warnings, err := Decide(facts, resolved)
	planned.Recap, planned.Empty = planned.recap(), planned.empty()
	return planned, warnings, err
}

// ----------------------------------------------------------------------------
// Apply

// Apply carries out the plan, reporting each step: it fast-forwards each worktree that moves in turn, a failure stopping none of the others, and reports the others as they stand. Once ctx is cancelled, the worktrees not yet started are skipped.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	items := make([]Item, 0, len(plan.Worktrees))
	for _, a := range plan.Worktrees {
		items = append(items, u.advance(ctx, plan, a, report))
	}

	return Result{Items: items}
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form the wizard asks for the request, on origin as last fetched: the worktrees it may fast-forward, those behind origin picked by default, and whether to force the unsafe ones, and the request's problems as field errors. It never fetches: a form is asked again on each answer.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what fast-forward decides from: the project's config, the repository's snapshot, where each name asked as a path leads, and, for each worktree asked (each on a branch when none is named, for the picker and its default pick), whether its directory is gone and its uncommitted changes. A repository with no wtm config is refused with CodeConfigNotFound.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	facts := Facts{Repository: repo, Located: map[string]string{}, Checks: map[string]Check{}}
	for _, name := range req.Worktrees {
		if facts.Located[name], err = repository.Located(ctx, u.Git, name, req.Caller); err != nil {
			return Facts{}, err
		}
	}
	for _, w := range facts.asked(req) {
		var check Check
		if check.Missing, err = repository.Gone(ctx, u.Git, w); err != nil {
			return Facts{}, err
		}
		if !check.Missing {
			changes, err := u.Git.Changes(ctx, w.Path)
			if err != nil {
				return Facts{}, err
			}
			check.Changes = len(changes)
		}
		facts.Checks[w.Name()] = check
	}

	return facts, nil
}

// ----------------------------------------------------------------------------
// The steps of Apply

// advance fast-forwards one worktree between its unit events, shielded from a cancel, as the plan settled it; a worktree the plan leaves stands as it is, with no event, and one reached after a cancel is skipped.
func (u UseCase) advance(ctx context.Context, plan Plan, a Advance, report domain.Report) (item Item) {
	item.Subject = a.Worktree
	switch {
	case !a.moves() && (a.Standing == StandingUpToDate || a.Standing == StandingAhead):
		item.Status, item.Reason = domain.StatusDone, domain.ReasonUnchanged
		return item
	case !a.moves():
		item.Status, item.Reason = domain.StatusSkipped, domain.ReasonRuledOut
		return item
	case ctx.Err() != nil:
		item.Status, item.Reason = domain.StatusSkipped, domain.ReasonInterrupted
		return item
	}

	report(domain.UnitStarted{Subject: a.Worktree})
	defer func() { report(domain.UnitFinished{Subject: item.Subject, Status: item.Status}) }()

	// Shielded: git's merge stopped half-way would leave the worktree's files between two commits.
	shielded, release := u.Shield(ctx)
	defer release()
	report(domain.PhaseStarted{Subject: a.Worktree, Phase: domain.PhaseFastForward})
	// Onto the commit planned, which the fingerprint covers: origin's branch may have moved on since.
	forwarded := u.Provision.FastForward(shielded, provision.FastForwardParams{Subject: a.Worktree, CheckedOutAt: a.Path, Force: plan.Force, Onto: a.To})
	report(domain.PhaseFinished{Subject: a.Worktree, Phase: domain.PhaseFastForward, Failed: forwarded.Failed()})

	if forwarded.Failed() {
		item.Status, item.Err = domain.StatusFailed, failure(a, forwarded)
		return item
	}
	item.Status, item.Detail = domain.StatusDone, &Detail{Path: a.Path, From: a.From, To: a.To}
	return item
}

// failure is why the worktree of a did not move, as one fixed line per code and, when git refused, its own words in the params.
func failure(a Advance, forwarded provision.FastForwarded) *domain.Error {
	params := domain.Params{"branch": a.Worktree, "path": a.Path}
	switch {
	case forwarded.Dirty:
		return &domain.Error{Kind: domain.KindConflict, Code: CodeDirty, Message: messageDirty, Params: params}
	case forwarded.Busy:
		return &domain.Error{Kind: domain.KindConflict, Code: CodeBusy, Message: messageBusy, Params: params}
	}

	params["cause"] = forwarded.Err.Error()
	return domain.AsError(domain.AsErrorParams{Err: forwarded.Err, Code: CodeFailed, Message: messageFailed, Params: params})
}
