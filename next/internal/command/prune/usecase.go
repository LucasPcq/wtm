// Package prune removes the worktrees done with: their pull request merged or closed while they stood at its head, or the branch they track deleted on origin. What it finds, what it keeps for being unsafe, what refuses a worktree named unsafe, then each removed in turn, its on_clean hooks first, and the children it leaves behind reparented or named.
package prune

import (
	"cmp"
	"context"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = "prune"

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// A UseCase removes the worktrees done with from one repository: the ports it reaches the repository through and the services it shares with the other commands.
type UseCase struct {
	Git          Git
	Meta         Meta
	Jobs         Jobs
	PullRequests PullRequests
	Teardown     teardown.Service
	Repository   repository.Service
	Operations   operation.Service
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied (a dry run, or a refusal before the apply).
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: one Item per worktree of the plan, in its order, and the warnings of the whole run.
type Result = domain.Result[Detail]

// An Item is what became of one worktree: its status and why, the error it failed on, its warnings, and its Detail once it is removed.
type Item = domain.Unit[Detail]

// A Detail describes a worktree removed as the JSON envelope reports it: where it was, and why it was done with.
type Detail struct {
	Path   string `json:"path"`
	Reason Reason `json:"reason"`
}

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: it takes the locks (none on a dry run, which plans once), plans under them, fingerprints the plan, stops on a dry run, refuses a stale plan and applies it between the operation's started and finished events. An error returned is a refusal before the apply, an unsafe worktree named among them; a failure during the apply is in the Outcome's Result.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Begin the operation, which takes the locks of the worktrees named before anything is read; a dry run takes none.
	op, err := u.Operations.Begin(Name, req.Locks(), opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Plan; a plan naming a worktree the request did not, each one done with when none is named and each child it reparents, is planned again once those locks are held.
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
	outcome.Warnings = op.Warnings(append(teardown.OfRemoved(outcome.Warnings, result.Items), result.Warnings...))
	return outcome, nil
}

// ----------------------------------------------------------------------------
// Plan

// Plan decides what the request does to the repository as it stands, changing nothing but origin's branches fetched: the plan and its warnings, or the refusal that stops the request (a *domain.Error, field errors and unsafe worktrees named with CodeUnsafe included).
func (u UseCase) Plan(ctx context.Context, req Request) (Plan, []domain.Warning, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Plan{}, nil, err
	}

	resolved, err := Resolve(facts, req)
	if err != nil {
		return Plan{}, nil, err
	}

	planned, warnings, err := Decide(facts, resolved)
	planned.Recap = planned.recap()
	return planned, warnings, err
}

// ----------------------------------------------------------------------------
// Apply

// Apply carries out the plan, reporting each step: it removes each worktree in turn, a failure stopping none of the others, those it leaves as they are reported ruled out, then reparents the children of the worktrees removed onto the nearest one that stayed, or warns of each one left orphaned; it ends on the step out of a worktree removed under the caller's feet. Once ctx is cancelled, the worktrees not yet started are skipped.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	items := make([]Item, 0, len(plan.Worktrees))
	for _, c := range plan.Worktrees {
		if c.ruledOut() {
			items = append(items, Item{Subject: c.Worktree, Status: domain.StatusSkipped, Reason: domain.ReasonRuledOut})
			continue
		}
		removed := u.Teardown.Remove(ctx, teardown.RemoveParams{Command: Name, Facts: plan.facts.Facts, Removal: c.Removal, Report: report})
		items = append(items, itemOf(c, removed))
	}

	// The moves are decided again on what went: a worktree that failed to go is a parent that stays.
	moves := plan.facts.Moves(teardown.Went(removals(plan.Worktrees), items))
	warnings := teardown.OrphanedBy(Name, moves)
	if plan.ReparentChildren {
		warnings = u.reparent(ctx, plan, moves, report)
	}

	return Result{Items: items, Warnings: warnings, FollowUps: plan.next(items)}
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form the wizard asks for the request: the worktrees done with it may remove, whether to reparent the children they leave behind and whether to force the unsafe ones picked, and the request's problems as field errors.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what prune decides from:
//  1. the project's config and the repository's snapshot, and the meta record of each worktree;
//  2. origin's branch of each worktree fetched again, the gone ones dropped, when gone branches are asked for and the fetch is not skipped;
//  3. at once: the jobs of the run daemon, the pull request of each worktree's branch, and the branches whose upstream origin deleted;
//  4. which branches stand behind their finished pull request's head;
//  5. what makes each worktree done with unsafe to remove, each one named when the request names some: its uncommitted changes, the commits no other branch holds but its merged pull request's head, its open pull request and its jobs up.
//
// A repository with no wtm config is refused with CodeConfigNotFound.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	facts := Facts{Facts: teardown.Facts{Repository: repo, Records: map[string]worktree.Meta{}, Checks: map[string]teardown.Check{}}}
	cwd, err := repository.Located(ctx, u.Git, req.Caller.Cwd, domain.Caller{})
	if err != nil {
		return Facts{}, err
	}
	facts.Cwd = cmp.Or(cwd, req.Caller.Cwd)
	for _, w := range repo.Worktrees {
		if w.Branch() == "" {
			continue
		}
		// A record that cannot be read is no parent to its children and no data kept: the worktree is removed all the same.
		if record, err := u.Meta.Load(w.Branch()); err == nil {
			facts.Records[w.Branch()] = record
		}
	}

	linked := slices.DeleteFunc(slices.Clone(repo.Worktrees), func(w worktree.Worktree) bool { return w.Main })

	branches := teardown.Branches(linked)
	if req.asks(ReasonGone) && !req.NoFetch && len(branches) > 0 {
		facts.FetchErr = u.Git.RefreshOrigin(ctx, branches)
		if ctx.Err() != nil {
			return Facts{}, ctx.Err()
		}
	}

	var goneErr error
	answered := repository.Ask(ctx, repository.AskParams{Jobs: u.Jobs, PullRequests: u.PullRequests, Branches: branches, WithPRs: len(branches) > 0}, func() {
		if req.asks(ReasonGone) {
			facts.Gone, goneErr = u.Git.Gone(ctx)
		}
	})
	if goneErr != nil {
		return Facts{}, goneErr
	}
	if answered.Interrupted {
		return Facts{}, ctx.Err()
	}
	facts.JobsErr, facts.PRsErr, facts.PullRequests = answered.JobsErr, answered.PRsErr, answered.PullRequests
	facts.Behind = repository.Behind(ctx, u.Git, repo, facts.PullRequests, linked)

	// Worktrees removed together hold nothing for each other: a request naming some checks them against every other worktree, which stays.
	checked := facts.Candidates(req)
	if len(req.Worktrees) > 0 {
		checked = slices.DeleteFunc(checked, func(c Candidate) bool { return !slices.Contains(req.Worktrees, c.Worktree) })
	}
	done := worktreesOf(repo, checked)
	if facts.Checks, err = u.Teardown.Read(ctx, done); err != nil {
		return Facts{}, err
	}
	facts.Settle(done, answered.Jobs, answered.PullRequests)
	return facts, nil
}

// worktreesOf are the worktrees of candidates, as git lists them.
func worktreesOf(repo worktree.Repository, candidates []Candidate) []worktree.Worktree {
	return slices.DeleteFunc(slices.Clone(repo.Worktrees), func(w worktree.Worktree) bool {
		return !slices.ContainsFunc(candidates, func(c Candidate) bool { return c.Worktree == w.Name() })
	})
}

// ----------------------------------------------------------------------------
// The steps of Apply

// The codes of what becomes of a removal: an on_clean hook that failed, the hooks skipped or run without the worktree's environment, a worktree git could not remove or whose files stay on disk, a branch or a record that could not be deleted.
const (
	CodeHookFailed         = Name + teardown.HookFailed
	CodeHooksSkipped       = Name + teardown.HooksSkipped
	CodeHookEnvUnavailable = Name + teardown.HookEnvUnavailable
	CodeRemoveFailed       = Name + teardown.RemoveFailed
	CodeLeftOnDisk         = Name + teardown.LeftOnDisk
	CodeBranchKept         = Name + teardown.BranchKept
	CodeMetaKept           = Name + teardown.MetaKept
)

// itemOf is what became of the candidate c as prune reports it: where it was and why it was done with, once it went.
func itemOf(c Candidate, removed teardown.Removed) Item {
	item := Item{Subject: c.Worktree, Status: removed.Status, Reason: removed.Reason, Err: removed.Err, Warnings: removed.Warnings}
	if removed.Gone {
		item.Detail = &Detail{Path: c.Path, Reason: c.Reason}
	}

	return item
}

// reparent records each child's new parent, reporting it updated; a child whose record cannot be written stays orphaned, with a warning.
func (u UseCase) reparent(ctx context.Context, plan Plan, moves []teardown.Move, report domain.Report) []domain.Warning {
	var warnings []domain.Warning
	for _, move := range moves {
		err := repository.Reparent(ctx, repository.ReparentParams{Meta: u.Meta, Branch: move.Branch, Path: plan.facts.ChildPath(move), To: move.To, Report: report})
		if err != nil {
			warnings = append(warnings, teardown.NotReparented(Name, move, err))
		}
	}

	return warnings
}
