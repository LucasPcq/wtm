// Package sync rebases worktrees onto their parent, down the stack: one fetch of origin, the branches they are rebased onto fast-forwarded first, then each worktree in the cascade's order, parents before children, those it cannot rebase safely refused when named and left as they are when the command chose them.
package sync

import (
	"context"
	"errors"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = domain.CommandSync

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// A UseCase syncs worktrees of one repository: the ports it reaches the repository through and the services it shares with the other commands.
type UseCase struct {
	Git        Git
	Meta       Meta
	Shield     Shield
	Repository repository.Service
	Operations operation.Service
	Provision  provision.Service
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied (a dry run, or a refusal before the apply).
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: one Item per branch fast-forwarded first, then per worktree of the cascade, in its order, and the steps after it.
type Result = domain.Result[Detail]

// An Item is what became of one branch fast-forwarded first or one worktree of the cascade: its status and why, the error it failed on, and its Detail once it moved or stands on its parent.
type Item = domain.Unit[Detail]

// A Detail describes a worktree synced as the JSON envelope reports it: where it is, the parent it stands on, the commit it stood at before, its own commits replayed, and whether it was pushed. Of a branch fast-forwarded first, it has no parent, and Commits are those it took from origin.
type Detail struct {
	Path    string `json:"path"`
	Parent  string `json:"parent"`
	From    string `json:"from"`
	Commits int    `json:"commits"`
	Pushed  bool   `json:"pushed"`
}

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: it takes the locks (none on a dry run, which plans once), fetches origin, plans under them, fingerprints the plan, stops on a dry run, refuses a stale plan and applies it between the operation's started and finished events. An error returned is a refusal before the apply, an unsafe worktree named among them; a failure during the apply, a conflict included, is in the Outcome's Result.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Begin the operation, which takes the request's locks before anything is read, so the facts cannot change under the plan; a dry run takes none.
	op, err := u.Operations.Begin(Name, req.Locks(), opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Fetch origin once, so each branch stands where origin has it now, unless the request reads it as last fetched; a fetch that fails leaves the plan on origin as last fetched, said in a warning.
	var fetched error
	if !req.NoFetch {
		fetched = u.Git.Fetch(ctx)
	}
	if fetched != nil && ctx.Err() != nil {
		return Outcome{Op: op.ID}, ctx.Err()
	}

	// 3. Plan; a plan moving a branch the request did not name, one the command chose, is planned again once that lock is held.
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

	// 5. A preview made on other facts, a branch or origin moved since among them, is refused rather than applied.
	if err := op.Expect(outcome.Fingerprint); err != nil {
		return outcome, err
	}

	// 6. Apply between the started and finished events other processes listen to.
	report := op.Started()
	result := u.Apply(ctx, plan, report)
	op.Finished(result.Failure())

	outcome.Result = &result
	outcome.Warnings = op.Warnings(append(outcome.Warnings, result.Warnings...))
	return outcome, nil
}

// ----------------------------------------------------------------------------
// Plan

// Plan decides what the request does to the repository as origin was fetched last, fetched said why that fetch failed, nil when it did not: the plan and its warnings, or the refusal that stops the request (a *domain.Error, field errors, unsafe worktrees with CodeUnsafe and a cycle with CodeCycle included).
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

	// Where each worktree of the cascade forked from its parent, the commit its rebase replays after: read before any branch moves, so a parent rebased first still has its children replay their own commits alone.
	for _, r := range facts.decide(resolved).plan.Worktrees {
		if err := u.forked(ctx, facts, r.Worktree); err != nil {
			return Plan{}, nil, err
		}
	}

	planned, warnings, err := Decide(facts, resolved)
	planned.Recap, planned.Empty = planned.recap(!resolved.All), planned.empty()
	return planned, warnings, err
}

// ----------------------------------------------------------------------------
// Apply

// Apply carries out the plan, reporting each step: it fast-forwards the branches rebased onto, then syncs each worktree of the cascade in turn, parents before children, one Item for each in that order; a failure stops the worktrees under the one that failed, never the others, and the branches it leaves stand as they are. Once ctx is cancelled, the branches and worktrees not yet started are skipped.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	stands := map[string]string{} // the commit each branch rebased onto stands at when its fast-forward failed
	items := make([]Item, 0, len(plan.Onto)+len(plan.Worktrees))
	for _, o := range plan.Onto {
		if !o.moves() {
			continue
		}
		item := u.forward(ctx, plan, o, report)
		if item.Status == domain.StatusFailed {
			stands[o.Branch] = o.From
		}
		items = append(items, item)
	}

	stopped := map[string]bool{}
	for _, r := range plan.Worktrees {
		item := u.sync(ctx, plan, r, ontoOf(r, stands), stopped[r.Parent], report)
		stopped[r.Worktree] = item.Status == domain.StatusFailed || item.Reason == domain.ReasonBlocked || item.Reason == domain.ReasonInterrupted
		items = append(items, item)
	}

	return Result{Items: items, FollowUps: plan.next(items)}
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form the wizard asks for the request, on origin as last fetched: the worktrees it may sync, those that need it picked by default, whether the worktrees under the named come too, whether to force the unsafe ones, what a conflict does, whether to fast-forward the branches rebased onto and whether to push, and the request's problems as field errors. It never fetches: a form is asked again on each answer.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what sync decides from: the project's config, the repository's snapshot, the meta record of each branch and the stack they make, where each name asked as a path leads, and for each worktree on a branch whether its directory is gone and its uncommitted changes, then for each it may rebase its own commits and its parent's it lacks, and, diverged from origin, the commits of origin it never held. A repository with no wtm config is refused with CodeConfigNotFound.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	records, recordErrs := repository.Records(u.Meta, repo.Live())
	facts := Facts{Repository: repo, Records: records, RecordErrs: recordErrs, Stack: worktree.StackOf(repo.Worktrees, records, repo.Config.Worktrees.BaseBranch), Located: map[string]string{}, Checks: map[string]Check{}}
	for _, name := range req.Worktrees {
		if facts.Located[name], err = repository.Located(ctx, u.Git, name, req.Caller); err != nil {
			return Facts{}, err
		}
	}

	for _, w := range facts.MainFirst() {
		if w.Branch() == "" {
			continue
		}
		if facts.Checks[w.Branch()], err = u.check(ctx, facts, w); err != nil {
			return Facts{}, err
		}
	}

	return facts, nil
}

// check reads what the worktree w holds: whether its directory is gone, its uncommitted changes, the commits of origin it never held when it diverged from origin, and, for one sync may rebase onto a parent it can count against, the commits of its parent it lacks.
func (u UseCase) check(ctx context.Context, facts Facts, w worktree.Worktree) (Check, error) {
	var check Check
	var err error
	if check.Missing, err = repository.Gone(ctx, u.Git, w); err != nil || check.Missing {
		return check, err
	}
	changes, err := u.Git.Changes(ctx, w.Path)
	if err != nil {
		return check, err
	}
	check.Changes = len(changes)

	branch := w.Branch()
	if origin := facts.OriginOf(branch); origin != nil && origin.State == worktree.Diverged {
		if check.Never, err = u.Git.NeverHeld(ctx, branch); err != nil {
			return check, err
		}
	}

	parent, counted := facts.CountedAgainstParent(facts.Stack, w)
	if _, recorded := facts.Stack.Parent(branch); !counted || !recorded || branch == facts.Config.Worktrees.BaseBranch {
		return check, nil
	}
	gap, err := u.Git.AheadBehindOf(ctx, worktree.Span{Branch: branch, Base: parent})
	check.Behind = gap.Behind
	return check, err
}

// forked reads where the worktree of branch forked from its parent, for one sync may rebase onto a parent it counts against.
func (u UseCase) forked(ctx context.Context, facts Facts, branch string) error {
	check, ok := facts.Checks[branch]
	parent, recorded := facts.Stack.Parent(branch)
	if !ok || check.Missing || !recorded || !facts.Local(parent) || !facts.Local(branch) || facts.Stack.InCycle(branch) {
		return nil
	}

	fork, err := u.Git.ForkPoint(ctx, worktree.Span{Branch: branch, Base: parent})
	if errors.Is(err, worktree.ErrNoCommonHistory) {
		check.Unrelated, err = true, nil
	}
	check.Fork = fork
	facts.Checks[branch] = check
	return err
}

// ----------------------------------------------------------------------------
// The steps of Apply

// CodeFFFailed fails the item of a branch rebased onto that could not be fast-forwarded: the worktrees on it are rebased onto it as it stands.
const CodeFFFailed = Name + ".ff_failed"

// forward fast-forwards o, a branch rebased onto, to the commit of origin planned, between its phase events: in its worktree, or, held by none, its ref moved. Its item is done with the commits it took, or failed with why, its children rebased onto it as it stands. Once ctx is cancelled, it is not started.
func (u UseCase) forward(ctx context.Context, plan Plan, o Onto, report domain.Report) Item {
	item := Item{Subject: o.Branch}
	if ctx.Err() != nil {
		item.Status, item.Reason = domain.StatusSkipped, domain.ReasonInterrupted
		return item
	}
	shielded, release := u.Shield(ctx)
	defer release()

	report(domain.PhaseStarted{Subject: o.Branch, Phase: domain.PhaseFastForward})
	cause := u.forwardCause(shielded, plan, o)
	report(domain.PhaseFinished{Subject: o.Branch, Phase: domain.PhaseFastForward, Failed: cause != ""})
	if cause != "" {
		children := strings.Join(o.Children, ", ")
		item.Status, item.Err = domain.StatusFailed, &domain.Error{Kind: domain.KindConflict, Code: CodeFFFailed, Message: "couldn't fast-forward " + o.Branch + ": " + cause + ": " + children + " rebased onto it as it stands", Params: domain.Params{"branch": o.Branch, "cause": cause, "children": children}}
		return item
	}

	item.Status, item.Detail = domain.StatusDone, &Detail{Path: o.Path, From: o.From, Commits: o.Origin.Behind}
	return item
}

// forwardCause fast-forwards o and says why it could not, "" when it did. A branch no worktree holds is moved to the commit planned, not to origin's as it stands now, after checking it is a fast-forward of where it stands.
func (u UseCase) forwardCause(ctx context.Context, plan Plan, o Onto) string {
	if o.Path == "" {
		ahead, err := u.Git.IsAncestor(ctx, worktree.Ancestry{Ancestor: o.From, Descendant: o.To})
		switch {
		case err != nil:
			return err.Error()
		case !ahead:
			return "origin's branch is no fast-forward of it"
		}
		if err := u.Git.UpdateBranch(ctx, worktree.BranchUpdate{Branch: o.Branch, From: o.From, To: o.To}); err != nil {
			return err.Error()
		}
		return ""
	}

	forwarded := u.Provision.FastForward(ctx, provision.FastForwardParams{Subject: o.Branch, CheckedOutAt: o.Path, Force: plan.Force, Onto: o.To})
	switch {
	case !forwarded.Failed():
		return ""
	case forwarded.Busy:
		return "its worktree is in the middle of an operation"
	case forwarded.Err != nil:
		return forwarded.Err.Error()
	}
	return "its worktree has uncommitted changes"
}

// ontoOf is the commit r is rebased onto: its parent's as planned, unless that parent's fast-forward failed, which leaves it where it stood, or its parent as it stands once rebased in this run, when it rebases too.
func ontoOf(r Rebase, stands map[string]string) worktree.Ref {
	if tip, failed := stands[r.Parent]; failed {
		return worktree.Revision{Hash: tip}
	}
	if r.Onto != "" {
		return worktree.Revision{Hash: r.Onto}
	}
	return worktree.LocalBranch{Name: r.Parent}
}

// sync syncs one worktree between its unit events, each git step shielded from a cancel, as the plan settled it: its own origin's commits brought in, the rebase onto its parent, then the push. A worktree the plan leaves stands as it is, with no event; one whose parent stopped, or reached after a cancel, is skipped.
func (u UseCase) sync(ctx context.Context, plan Plan, r Rebase, onto worktree.Ref, parentStopped bool, report domain.Report) (item Item) {
	item.Subject = r.Worktree
	detail := &Detail{Path: r.Path, Parent: r.Parent, From: r.From}
	switch {
	case len(r.Phases) == 0 && r.Standing == StandingOnParent:
		item.Status, item.Reason, item.Detail = domain.StatusDone, domain.ReasonUnchanged, detail
		return item
	case len(r.Phases) == 0:
		item.Status, item.Reason = domain.StatusSkipped, domain.ReasonRuledOut
		return item
	case ctx.Err() != nil:
		item.Status, item.Reason = domain.StatusSkipped, domain.ReasonInterrupted
		return item
	case parentStopped:
		item.Status, item.Reason = domain.StatusSkipped, domain.ReasonBlocked
		return item
	}

	report(domain.UnitStarted{Subject: r.Worktree})
	defer func() { report(domain.UnitFinished{Subject: item.Subject, Status: item.Status}) }()
	// Shielded: a rebase or a merge stopped half-way would leave the worktree between two commits.
	shielded, release := u.Shield(ctx)
	defer release()

	item.Status = domain.StatusDone
	for i, phase := range r.Phases {
		// a first Ctrl+C lets the git step under way finish, and starts no other
		if i > 0 && ctx.Err() != nil {
			item.Status, item.Reason = domain.StatusCancelled, domain.ReasonInterrupted
			break
		}
		report(domain.PhaseStarted{Subject: r.Worktree, Phase: phase})
		failure, warning := u.step(shielded, plan, stepParams{Rebase: r, Phase: phase, Onto: onto, Detail: detail})
		report(domain.PhaseFinished{Subject: r.Worktree, Phase: phase, Failed: failure != nil})
		if warning != nil {
			item.Warnings = append(item.Warnings, *warning)
		}
		if failure != nil {
			item.Status, item.Reason, item.Err = domain.StatusFailed, "", failure
			break
		}
	}
	// a push refused leaves the rebase that went through before it
	if item.Err == nil || item.Err.Code == CodePushFailed {
		item.Detail = detail
	}
	return item
}

// stepParams are one phase of a worktree's sync: the worktree as planned, the phase, the commit its rebase goes onto, and the Detail the step records what it did in.
type stepParams struct {
	Rebase
	Phase  domain.Phase
	Onto   worktree.Ref
	Detail *Detail
}

// step runs one phase of a worktree's sync, recording in its detail what it did: the failure it stops on, and the warning it leaves.
func (u UseCase) step(ctx context.Context, plan Plan, s stepParams) (*domain.Error, *domain.Warning) {
	r, detail := s.Rebase, s.Detail
	params := domain.Params{"worktree": r.Worktree, "parent": r.Parent, "path": r.Path}
	switch s.Phase {
	case domain.PhaseFastForward:
		// Onto the commit planned, which the fingerprint covers: origin's branch may have moved on since.
		forwarded := u.Provision.FastForward(ctx, provision.FastForwardParams{Subject: r.Worktree, CheckedOutAt: r.Path, Force: plan.Force, Onto: r.Expect})
		return forwardFailure(forwarded, params), nil
	case domain.PhaseRebase:
		rebased, err := u.Git.Rebase(ctx, worktree.Rebase{Path: r.Path, Onto: s.Onto, Upstream: r.Fork, Keep: plan.KeepConflict, Autostash: plan.Force && len(r.Blockers) > 0})
		failure := rebaseFailure(rebased, err, params)
		if failure == nil {
			detail.Commits = r.Commits
		}
		return failure, stashWarning(rebased, params)
	case domain.PhasePush:
		if err := u.Git.Push(ctx, worktree.Push{Branch: r.Worktree, Expect: r.Expect}); err != nil {
			params["cause"] = err.Error()
			return domain.AsError(domain.AsErrorParams{Err: err, Code: CodePushFailed, Message: "git refused the push", Params: params}), nil
		}
		detail.Pushed = true
	}

	return nil, nil
}

// forwardFailure is why the worktree's branch did not catch up with its origin, nil when it did.
func forwardFailure(forwarded provision.FastForwarded, params domain.Params) *domain.Error {
	switch {
	case !forwarded.Failed():
		return nil
	case forwarded.Dirty:
		return &domain.Error{Kind: domain.KindConflict, Code: CodeDirty, Message: "the worktree has uncommitted changes", Params: params}
	case forwarded.Busy:
		return &domain.Error{Kind: domain.KindConflict, Code: CodeBusy, Message: "the worktree is in the middle of an operation", Params: params}
	}

	params["cause"] = forwarded.Err.Error()
	return domain.AsError(domain.AsErrorParams{Err: forwarded.Err, Code: CodeFailed, Message: "git refused to bring in origin's commits", Params: params})
}

// rebaseFailure is why the rebase did not go through, nil when it did: a conflict, its files and whether it is left in progress in the params, else git's refusal.
func rebaseFailure(rebased worktree.Rebased, err error, params domain.Params) *domain.Error {
	switch {
	case rebased.Conflicted():
		params["files"], params["count"], params["kept"] = strings.Join(rebased.Conflicts, ", "), strconv.Itoa(len(rebased.Conflicts)), strconv.FormatBool(rebased.Kept)
		message := "the rebase onto " + params["parent"] + " stopped on a conflict in " + params["files"] + ": aborted, the worktree left as it was"
		if rebased.Kept {
			message = "the rebase onto " + params["parent"] + " stopped on a conflict in " + params["files"] + ": left in progress, to resolve"
		}
		return &domain.Error{Kind: domain.KindConflict, Code: CodeConflict, Message: message, Params: params}
	case err != nil:
		params["cause"] = err.Error()
		return domain.AsError(domain.AsErrorParams{Err: err, Code: CodeFailed, Message: "git refused the rebase", Params: params})
	}
	return nil
}

func stashWarning(rebased worktree.Rebased, params domain.Params) *domain.Warning {
	if !rebased.Stashed {
		return nil
	}
	return &domain.Warning{Code: CodeStashed, Message: "its uncommitted changes did not apply back after the rebase: git keeps them in its stash", Params: domain.Params{"worktree": params["worktree"]}}
}
