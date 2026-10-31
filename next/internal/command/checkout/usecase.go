// Package checkout gives a pull request's branch its worktree: the pull request read, its branch fetched from origin, then the branch provisioned as create provisions one, its parent the branch the pull request merges into.
package checkout

import (
	"context"
	"errors"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = "checkout"

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// CodeMetaNewer refuses a request that would number the new worktree past a record a newer wtm wrote.
const CodeMetaNewer = Name + domain.MetaNewer

// CodeGitHubUnavailable refuses a request whose pull request GitHub could not be asked of: gh missing or signed out, or a repository GitHub does not hold.
const CodeGitHubUnavailable = Name + ".github_unavailable"

// A UseCase checks pull requests out in one repository: the ports it reaches the repository and GitHub through and the services it shares with the other commands.
type UseCase struct {
	Git          Git
	PullRequest  PullRequest
	PullRequests PullRequests
	Repository   repository.Service
	Operations   operation.Service
	Provision    provision.Service
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied (a dry run, or a refusal before the apply).
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: the Item of the pull request's branch, and the warnings of the whole run.
type Result = domain.Result[Detail]

// An Item is what became of the pull request's branch: its status and why, the error it failed on, its warnings, and its Detail once its worktree stands.
type Item = domain.Unit[Detail]

// A Detail describes the branch's worktree as the JSON envelope reports it: where it is, its meta record, where the branch stood against origin and the env ports it was given.
type Detail struct {
	Path     string        `json:"path"`
	Metadata worktree.Meta `json:"metadata"`
	worktree.Standing
	EnvPorts env.Redacted `json:"env_ports,omitzero"`
}

// Worktree is where the worktree of the Detail is and the env ports it was given.
func (d Detail) Worktree() (string, env.Redacted) {
	return d.Path, d.EnvPorts
}

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: it takes the locks (none on a dry run, which plans once), plans under them, fingerprints the plan, stops on a dry run, refuses a stale plan and applies it between the operation's started and finished events. An error returned is a refusal before the apply; a failure during the apply is in the Outcome's Result.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Begin the operation, which takes the request's locks before anything is read; the branch is not known before the pull request is read, so only the repository's is taken here.
	op, err := u.Operations.Begin(Name, req.Locks(), opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Plan; the plan names the pull request's branch, so it is planned again once that lock is held.
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
	outcome.Plan = plan.applied(result)
	outcome.Warnings = op.Warnings(append(outcome.Warnings, result.Warnings...))
	return outcome, nil
}

// ----------------------------------------------------------------------------
// Plan

// Plan decides what the request does to the repository as it stands, changing nothing but origin's copy of the pull request's branch, fetched: the plan and its warnings, or the refusal that stops the request (a *domain.Error, field errors included).
func (u UseCase) Plan(ctx context.Context, req Request) (Plan, []domain.Warning, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Plan{}, nil, err
	}

	questions, err := resolve(facts, req)
	if err != nil {
		return Plan{}, nil, err
	}

	planned, warnings := Decide(facts, questions.Request())
	if refused := u.Provision.NewerNeighbours(provision.NewerNeighboursParams{Command: Name, Undone: "nothing created", Repository: facts.Repository, Planned: planned.provisioned()}); refused != nil {
		return Plan{}, nil, refused
	}
	planned.Recalled = questions.Recalled()
	planned.Recap = planned.previewRecap()
	return planned, warnings, nil
}

// ----------------------------------------------------------------------------
// Apply

// Apply carries out the plan, reporting each step: it fast-forwards the branch when asked, provisions its worktree, and remembers the answers asked once the worktree is created; it ends on the step into the worktree once it stands. Once ctx is cancelled before the branch is started, it is skipped.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	advanced, warnings := u.Provision.FastForwardEach(ctx, provision.FastForwardEachParams{Command: Name, Subjects: plan.FastForward, Snapshot: plan.facts.Snapshot, Fallback: "it starts from your local branch"}, report)

	item := u.item(ctx, plan, report)
	if item.Detail != nil && len(advanced) > 0 {
		item.Detail.Standing = item.Detail.Standing.FastForwarded()
	}
	if item.Detail != nil && item.Detail.OriginState == worktree.Behind {
		item.Warnings = append([]domain.Warning{behindOrigin(item.Detail.Standing)}, item.Warnings...)
	}

	remembered, failed := u.remember(ctx, plan, item)
	return Result{Items: []Item{item}, Warnings: append(warnings, failed...), Remembered: remembered, FollowUps: plan.next(item)}
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form the wizard asks for the request: each question with its options, the open pull requests among them, its answer so far and where that answer comes from, and the request's problems as field errors.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	facts.Open, err = u.PullRequests(ctx, req.filter())
	if err != nil {
		return Questions{}, unavailable(err)
	}

	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what checkout decides from: the project's config and the repository's snapshot, the pull request asked with its branch fetched from origin first, and what lies at the path that branch would take. A repository with no wtm config is refused with CodeConfigNotFound, a pull request GitHub cannot be asked of with CodeGitHubUnavailable.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	facts := Facts{Repository: repo}
	if req.PR == 0 {
		return facts, nil
	}

	pr, err := u.PullRequest(ctx, req.PR)
	switch {
	case errors.Is(err, worktree.ErrPullRequestNotFound):
		return facts, nil
	case err != nil:
		return Facts{}, unavailable(err)
	}
	facts.PR = &pr
	if pr.Fork {
		return facts, nil
	}

	// Fetched before the branches are read again, so where the branch stands against origin, and whether a fast-forward is asked, is what origin holds now, not as last fetched.
	fetched := u.Git.FetchRef(ctx, pr.Branch)
	if facts.Snapshot, err = u.Git.Snapshot(ctx); err != nil {
		return Facts{}, err
	}
	switch {
	case errors.Is(fetched, worktree.ErrNoRemoteBranch):
		// origin deleted the branch, a merged pull request's often: its remote-tracking branch, left as last fetched, is no branch to track nor to stand against
		facts.Branches.Remote = slices.DeleteFunc(slices.Clone(facts.Branches.Remote), func(b worktree.Branch) bool { return b.Name == pr.Branch })
	default:
		facts.FetchErr = fetched
	}

	facts.Folder, err = u.Git.Locate(ctx, facts.PathOf(pr.Branch))
	if err != nil {
		return Facts{}, err
	}

	return facts, nil
}

// unavailable is err, a pull request GitHub could not be asked of, as checkout's refusal; an interrupted read stays an interruption.
func unavailable(err error) error {
	if !errors.Is(err, worktree.ErrPullRequestsUnavailable) {
		return domain.AsError(domain.AsErrorParams{Err: err, Code: CodeGitHubUnavailable, Message: err.Error()})
	}

	return &domain.Error{Kind: domain.KindPrecondition, Code: CodeGitHubUnavailable, Message: err.Error()}
}

// ----------------------------------------------------------------------------
// The steps of Apply

// item provisions the planned branch between its unit events; a branch reached after a cancel is skipped.
func (u UseCase) item(ctx context.Context, plan Plan, report domain.Report) Item {
	branch := plan.PullRequest.Branch
	if ctx.Err() != nil {
		return Item{Subject: branch, Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted}
	}

	report(domain.UnitStarted{Subject: branch})
	outcome := u.Provision.Run(ctx, provision.ParamsOf(plan.facts.Repository, plan.provisioned(), report))

	item := Item{Subject: branch, Status: outcome.Status, Reason: outcome.Reason, Err: outcome.Err, Warnings: outcome.Warnings}
	if outcome.Provisioned.Stands() {
		item.Detail = &Detail{
			Path:     plan.Path,
			Metadata: outcome.Provisioned.Meta,
			Standing: plan.facts.StandingOf(branch, plan.placement()),
			EnvPorts: outcome.Provisioned.EnvPorts,
		}
	}

	report(domain.UnitFinished{Subject: branch, Status: item.Status})
	return item
}

// remember writes the answers the plan asks to remember once the worktree was created, and returns the fields it remembered.
func (u UseCase) remember(ctx context.Context, plan Plan, item Item) ([]string, []domain.Warning) {
	if item.Status != domain.StatusDone {
		return nil, nil
	}

	return u.Provision.Remember(ctx, Name, plan.settled, plan.Remember)
}
