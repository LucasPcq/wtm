// Package clean removes worktrees, their branches and their meta records: what the request settles on, what refuses an unsafe one, then each removed in turn, its on_clean hooks first, and the children it leaves behind reparented or named.
package clean

import (
	"cmp"
	"context"
	"errors"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = "clean"

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// A UseCase removes worktrees from one repository: the ports it reaches the repository through and the services it shares with the other commands.
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

// A Result is what the apply did: one Item per worktree asked, those removed in the plan's order then the names that are no worktree, and the warnings of the whole run.
type Result = domain.Result[Detail]

// An Item is what became of one worktree asked: its status and why, the error it failed on, its warnings, and its Detail once it is removed.
type Item = domain.Unit[Detail]

// A Detail describes a worktree removed as the JSON envelope reports it: where it was.
type Detail struct {
	Path string `json:"path"`
}

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: it takes the locks (none on a dry run, which plans once), plans under them, fingerprints the plan, stops on a dry run, refuses a stale plan and applies it between the operation's started and finished events. An error returned is a refusal before the apply, an unsafe worktree among them; a failure during the apply is in the Outcome's Result.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Begin the operation, which takes the request's locks before anything is read, so the facts cannot change under the plan; a dry run takes none.
	op, err := u.Operations.Begin(Name, req.Locks(), opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Plan; a plan naming a worktree the request did not, a child it reparents, is planned again once that lock is held.
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

// Plan decides what the request does to the repository as it stands, changing nothing: the plan and its warnings, or the refusal that stops the request (a *domain.Error, field errors and unsafe worktrees with CodeUnsafe included).
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

// Apply carries out the plan, reporting each step: it removes each worktree in turn, a failure stopping none of the others, reports each name that is no worktree as already absent, then reparents the children of the worktrees removed onto the nearest one that stayed, or warns of each one left orphaned; it ends on the step out of a worktree removed under the caller's feet. Once ctx is cancelled, the worktrees not yet started are skipped.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	items := make([]Item, 0, len(plan.Worktrees)+len(plan.Absent))
	for _, removal := range plan.Worktrees {
		removed := u.Teardown.Remove(ctx, teardown.RemoveParams{Command: Name, Facts: plan.facts.Facts, Removal: removal, Report: report})
		items = append(items, itemOf(removal, removed))
	}
	for _, name := range plan.Absent {
		items = append(items, Item{Subject: name, Status: domain.StatusDone, Reason: domain.ReasonUnchanged})
	}

	// The moves are decided again on what went: a worktree that failed to go is a parent that stays.
	moves := plan.facts.Moves(teardown.Went(plan.Worktrees, items))
	warnings := teardown.OrphanedBy(Name, moves)
	if plan.ReparentChildren {
		warnings = u.reparent(ctx, plan, moves, report)
	}

	return Result{Items: items, Warnings: warnings, FollowUps: plan.next(items)}
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form the wizard asks for the request: the worktrees it may remove, whether to reparent the children they leave behind and whether to force the unsafe ones, and the request's problems as field errors.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what clean decides from: the project's config, the repository's snapshot, the meta record of each worktree and of each branch asked no worktree holds, and, for each worktree req asks, what makes it unsafe to remove: its uncommitted changes, the commits no other branch holds but its merged pull request's head, its open pull request and its jobs up, the forge and the run daemon asked while git reads. A repository with no wtm config is refused with CodeConfigNotFound.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	facts := Facts{Facts: teardown.Facts{Repository: repo, Records: map[string]worktree.Meta{}, Newer: map[string]*domain.SchemaNewer{}, Checks: map[string]teardown.Check{}}}
	if facts.Located, err = u.locate(ctx, facts, req); err != nil {
		return Facts{}, err
	}
	cwd, err := repository.Located(ctx, u.Git, req.Caller.Cwd, domain.Caller{})
	if err != nil {
		return Facts{}, err
	}
	facts.Cwd = cmp.Or(cwd, req.Caller.Cwd)
	asked := facts.asked(req.Worktrees)
	for _, w := range append(slices.Clone(repo.Worktrees), asked...) {
		if w.Branch() == "" {
			continue
		}
		// A record that cannot be read is no parent to its children and no data kept: the worktree is removed all the same, unless a newer wtm wrote it.
		record, err := u.Meta.Load(w.Branch())
		if err == nil {
			facts.Records[w.Branch()] = record
		}
		if newer, ok := errors.AsType[*domain.SchemaNewer](err); ok {
			facts.Newer[w.Branch()] = newer
		}
	}
	if len(asked) == 0 {
		return facts, nil
	}

	var gitErr error
	asking := teardown.Branches(asked)
	answered := repository.Ask(ctx, repository.AskParams{Jobs: u.Jobs, Restart: u.Repository.Restart, PullRequests: u.PullRequests, Branches: asking, WithPRs: len(asking) > 0}, func() {
		// Worktrees removed together hold nothing for each other.
		facts.Checks, gitErr = u.Teardown.Read(ctx, teardown.ReadParams{Worktrees: asked, Going: teardown.Branches(asked)})
	})
	if gitErr != nil {
		return Facts{}, gitErr
	}
	if answered.Interrupted {
		return Facts{}, ctx.Err()
	}

	facts.JobsErr, facts.PRsErr = answered.JobsErr, answered.PRsErr
	if answered.Skewed != nil {
		skew := repository.SkewWarning(repository.SkewWarningParams{Command: Name, Skewed: answered.Skewed, Restart: u.Repository.Restart})
		facts.Skew = &skew
	}
	facts.Settle(asked, answered.Jobs, answered.PullRequests)
	return facts, nil
}

// locate resolves each name req asks that is no worktree's name as the path it stands for, from where the caller stands (repository.Located), spelled as git spells the worktree root it is.
func (u UseCase) locate(ctx context.Context, facts Facts, req Request) (map[string]string, error) {
	located := map[string]string{}
	for _, name := range req.Worktrees {
		if _, named := facts.named(name); named || strings.TrimSpace(name) == "" {
			continue
		}

		path, err := repository.Located(ctx, u.Git, name, req.Caller)
		if err != nil {
			return nil, err
		}
		if path == "" {
			continue
		}
		if located[name], err = u.root(ctx, facts.Worktrees, path); err != nil {
			return nil, err
		}
	}

	return located, nil
}

// root is path as git spells the worktree root it is, when it is one under another spelling (a disk that ignores case: git keeps a path as it was given), else path itself; only a path that matches no root as written is asked of the disk.
func (u UseCase) root(ctx context.Context, worktrees []worktree.Worktree, path string) (string, error) {
	if slices.ContainsFunc(worktrees, func(w worktree.Worktree) bool { return w.Path == path }) {
		return path, nil
	}
	for _, w := range worktrees {
		if w.Path == "" || w.Prunable {
			continue
		}
		same, err := u.Git.Same(ctx, path, w.Path)
		if err != nil {
			return "", err
		}
		if same {
			return w.Path, nil
		}
	}

	return path, nil
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

// itemOf is what became of removal as clean reports it: where it was, once it went.
func itemOf(removal teardown.Removal, removed teardown.Removed) Item {
	item := Item{Subject: removal.Worktree, Status: removed.Status, Reason: removed.Reason, Err: removed.Err, Warnings: removed.Warnings}
	if removed.Gone {
		item.Detail = &Detail{Path: removal.Path}
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
