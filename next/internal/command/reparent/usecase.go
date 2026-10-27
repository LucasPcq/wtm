// Package reparent changes the parent worktrees are rebased onto: what the request settles on, what refuses a parent that would close a cycle, then each worktree's meta record rewritten in turn. No git ref moves: the next sync rebases onto the new parent.
package reparent

import (
	"cmp"
	"context"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = domain.CommandReparent

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// A UseCase moves worktrees of one repository onto a new parent: the ports it reaches the repository through and the services it shares with the other commands.
type UseCase struct {
	Git        Git
	Meta       Meta
	Repository repository.Service
	Operations operation.Service
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied (a dry run, or a refusal before the apply).
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: one Item per worktree of the plan, in its order, and the steps after it.
type Result = domain.Result[Detail]

// An Item is what became of one worktree: its status and why, the error it failed on, and its Detail once it stands on the new parent.
type Item = domain.Unit[Detail]

// A Detail describes a worktree moved as the JSON envelope reports it: where it is, the parent it left (empty when it had none) and the one it stands on.
type Detail struct {
	Path string `json:"path"`
	From string `json:"from"`
	To   string `json:"to"`
}

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: it takes the locks (none on a dry run, which plans once), plans under them, fingerprints the plan, stops on a dry run, refuses a stale plan and applies it between the operation's started and finished events. An error returned is a refusal before the apply; a failure during the apply is in the Outcome's Result.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Begin the operation, which takes the locks of the worktrees named before anything is read; a dry run takes none.
	op, err := u.Operations.Begin(Name, req.Locks(), opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Plan; a plan naming a worktree the request did not, the caller's one by default and the new parent's chain, is planned again once those locks are held.
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
	outcome.Warnings = op.Warnings(outcome.Warnings)
	return outcome, nil
}

// ----------------------------------------------------------------------------
// Plan

// Plan decides what the request does to the repository as it stands, changing nothing: the plan, or the refusal that stops the request (a *domain.Error, field errors included).
func (u UseCase) Plan(ctx context.Context, req Request) (Plan, []domain.Warning, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Plan{}, nil, err
	}

	questions, err := Resolve(facts, req)
	if err != nil {
		return Plan{}, nil, err
	}

	planned := Decide(facts, questions)
	planned.Recap = planned.recap()
	return planned, nil, nil
}

// ----------------------------------------------------------------------------
// Apply

// Apply carries out the plan, reporting each step: it rewrites the parent in the record of each worktree that moves, in turn, a failure stopping none of the others, and leaves those already on the new parent as they are; it ends on the steps after: the sync of the worktrees moved, and the undo of each. Once ctx is cancelled, the worktrees not yet started are skipped.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	items := make([]Item, 0, len(plan.Moves))
	for _, move := range plan.Moves {
		items = append(items, u.move(ctx, move, report))
	}

	return Result{Items: items, FollowUps: plan.next(items)}
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form the wizard asks for the request: the worktrees it may move, the parents they may go to, and the request's problems as field errors.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what reparent decides from: the project's config, the repository's snapshot, the meta record of each worktree on a branch, the stack they make, and where the caller stands. A repository with no wtm config is refused with CodeConfigNotFound.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	branches := make([]string, 0, len(repo.Worktrees))
	for _, w := range repo.Worktrees {
		branches = append(branches, w.Branch())
	}
	records, recordErrs := repository.Records(u.Meta, branches)
	cwd, err := repository.Located(ctx, u.Git, req.Caller.Cwd, domain.Caller{})
	if err != nil {
		return Facts{}, err
	}

	return Facts{Repository: repo, Records: records, RecordErrs: recordErrs, Stack: worktree.StackOf(repo.Worktrees, records, repo.Config.Worktrees.BaseBranch), Cwd: cmp.Or(cwd, req.Caller.Cwd)}, nil
}

// ----------------------------------------------------------------------------
// The steps of Apply

// CodeWriteFailed is the failure of a worktree whose record could not be written: nothing else changed.
const CodeWriteFailed = Name + ".write_failed"

// move rewrites the parent in the record of move's worktree, reporting it updated; one already on the new parent is left as it is, one not started once ctx is cancelled skipped.
func (u UseCase) move(ctx context.Context, move Move, report domain.Report) Item {
	if ctx.Err() != nil {
		return Item{Subject: move.Branch, Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted}
	}
	item := Item{Subject: move.Branch, Status: domain.StatusDone, Detail: &Detail{Path: move.Path, From: move.From, To: move.To}}
	if move.Unchanged() {
		item.Reason = domain.ReasonUnchanged
		return item
	}

	report(domain.UnitStarted{Subject: move.Branch})
	err := repository.Reparent(ctx, repository.ReparentParams{Meta: u.Meta, Branch: move.Branch, Path: move.Path, To: move.To, Report: report})
	if err != nil {
		item = Item{Subject: move.Branch, Status: domain.StatusFailed, Err: domain.AsError(domain.AsErrorParams{Err: err, Code: CodeWriteFailed, Message: "couldn't write the record of " + move.Branch})}
	}
	report(domain.UnitFinished{Subject: move.Branch, Status: item.Status})
	return item
}
