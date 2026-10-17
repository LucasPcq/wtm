// Package create gives each branch asked its worktree, from one source: what the request settles on, what becomes of each branch, then each provisioned whole.
package create

import (
	"cmp"
	"context"
	"errors"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
)

// ----------------------------------------------------------------------------
// The use case

const name = "create"

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = "create.config_not_found"

// A UseCase creates worktrees in one repository: the ports it reaches the repository through, the services it shares with the other commands, and the paths of the main checkout and of wtm's state.
type UseCase struct {
	Git        Git
	Config     Config
	Answers    Answers
	Operations operation.Service
	Provision  provision.Service
	MainPath   string
	StateDir   string
}

// A Request is what the caller asks of create, as given: a field left empty takes its remembered answer, its configured value or its default when the request is resolved.
type Request struct {
	Branches    []string            `json:"branches"`
	From        string              `json:"from"`
	EnvFrom     project.EnvStrategy `json:"env_from"`
	Isolation   project.Isolation   `json:"isolation"`
	FastForward *bool               `json:"fast_forward"` // nil when not given, so a remembered answer can apply
	IfNotExists bool                `json:"if_not_exists"`
	Remember    []string            `json:"remember"` // the fields whose answer is remembered for the next runs
	Ask         bool                `json:"-"`        // ignore the remembered answers for this run
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied (a dry run, or a refusal before the apply).
type Outcome struct {
	Op          string
	Plan        Plan
	Fingerprint string
	Warnings    []domain.Warning
	Result      *Result
}

// Failure is the error the apply ended on, nil when it succeeded or did not run.
func (o Outcome) Failure() *domain.Error {
	if o.Result == nil {
		return nil
	}

	return o.Result.Failure()
}

// A Result is what the apply did: one Item per branch of the plan, in its order, and the warnings of the whole run.
type Result struct {
	Items    []Item
	Warnings []domain.Warning
}

// Failure is the error the run ends on: the first interruption, else the first failed branch, else nil.
func (r Result) Failure() *domain.Error {
	if at := slices.IndexFunc(r.Items, interrupted); at >= 0 {
		return cmp.Or(r.Items[at].Err, &domain.Error{Kind: domain.KindCancelled, Code: domain.CodeInterrupted, Message: "interrupted"})
	}

	if at := slices.IndexFunc(r.Items, func(item Item) bool { return item.Status == domain.StatusFailed }); at >= 0 {
		return r.Items[at].Err
	}

	return nil
}

// An Item is what became of one branch: its status and why, the error it failed on, its warnings, and its Detail once its worktree stands.
type Item struct {
	Subject  string
	Status   domain.Status
	Reason   domain.Reason
	Err      *domain.Error
	Warnings []domain.Warning
	Detail   *Detail
}

// A Detail describes a branch's worktree as the JSON envelope reports it: where it is, its meta record, where the branch stood against origin, its ordinal and the env ports it was given.
type Detail struct {
	Path     string        `json:"path"`
	Metadata worktree.Meta `json:"metadata"`
	Standing
	Ordinal  *int         `json:"ordinal,omitempty"`
	EnvPorts env.Redacted `json:"env_ports,omitzero"`
}

// Standing is where an existing branch the worktree checks out stood against origin, or the remote branch it resumes.
type Standing struct {
	ExistingBranch bool                `json:"existing_branch"`
	OriginState    worktree.Divergence `json:"origin_state,omitempty"`
	OriginAhead    int                 `json:"origin_ahead,omitempty"`
	OriginBehind   int                 `json:"origin_behind,omitempty"`
	Upstream       string              `json:"upstream,omitempty"`
}

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: it takes the locks, plans under them, fingerprints the plan, stops on a dry run, refuses a stale plan and applies it between the operation's started and finished events. An error returned is a refusal before the apply; a failure during the apply is in the Outcome's Result.
func (u UseCase) Run(ctx context.Context, req Request, opts operation.Options) (Outcome, error) {
	// 1. Take the operation's locks before reading anything, so the facts cannot change under the plan.
	op, err := u.Operations.Begin(name, req.Locks(), opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Plan; a plan naming a worktree the request did not is planned again once that lock is held.
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
	outcome.Warnings = op.Warnings(append(outcome.Warnings, result.Warnings...))
	return outcome, nil
}

// ----------------------------------------------------------------------------
// Plan

// Plan decides what the request does to the repository as it stands, changing nothing: the plan and its warnings, or the refusal that stops the request (a *domain.Error, field errors included).
func (u UseCase) Plan(ctx context.Context, req Request) (Plan, []domain.Warning, error) {
	facts, err := u.Observe(ctx, req.Branches)
	if err != nil {
		return Plan{}, nil, err
	}

	resolved, err := Resolve(facts, req)
	if err != nil {
		return Plan{}, nil, err
	}

	return Decide(facts, resolved)
}

// ----------------------------------------------------------------------------
// Apply

// Apply carries out the plan, reporting each step: it fast-forwards the sources asked, provisions each branch in turn, a failure stopping none of the others, and remembers the answers asked once a worktree is created. Once ctx is cancelled, the branches not yet started are skipped.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	warnings := u.fastForward(ctx, plan, report)

	items := make([]Item, 0, len(plan.Branches))
	for _, planned := range plan.Branches {
		items = append(items, u.item(ctx, plan, planned, report))
	}

	return Result{Items: items, Warnings: append(warnings, u.remember(ctx, plan, items)...)}
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form the wizard asks for the request: each question with its options, its answer so far and where that answer comes from, and the request's problems as field errors.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req.Branches)
	if err != nil {
		return Questions{}, err
	}

	return Form(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what create decides from: the project's config, the repository's snapshot and what lies at the path each of branches would take. A repository with no wtm config is refused with CodeConfigNotFound.
func (u UseCase) Observe(ctx context.Context, branches []string) (Facts, error) {
	config, global, err := u.Config.Load()
	if errors.Is(err, project.ErrConfigNotFound) {
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeConfigNotFound, Message: "no wtm config found — run `wtm init` first", Cause: err}
	}
	if err != nil {
		return Facts{}, err
	}

	snapshot, err := u.Git.Snapshot(ctx)
	if err != nil {
		return Facts{}, err
	}

	facts := Facts{MainPath: u.MainPath, Config: config, Global: global, Snapshot: snapshot}
	folders, err := u.locate(ctx, facts.pathsOf(branches))
	if err != nil {
		return Facts{}, err
	}
	facts.Folders = folders

	// A run.toml that fails to load does not stop the plan: its error travels with the facts to where run.toml is used.
	facts.Run, facts.RunErr = u.Config.LoadRun()
	return facts, nil
}

func (u UseCase) locate(ctx context.Context, paths []string) (map[string]worktree.Location, error) {
	folders := make(map[string]worktree.Location, len(paths))
	for _, path := range paths {
		location, err := u.Git.Locate(ctx, path)
		if err != nil {
			return nil, err
		}
		folders[path] = location
	}

	return folders, nil
}

// ----------------------------------------------------------------------------
// The steps of Apply

// fastForward fast-forwards each source the plan names; one that cannot be is a warning, its worktrees starting from the local branch.
func (u UseCase) fastForward(ctx context.Context, plan Plan, report domain.Report) []domain.Warning {
	var warnings []domain.Warning
	for _, subject := range plan.FastForward {
		if ctx.Err() != nil {
			return warnings
		}

		report(domain.PhaseStarted{Subject: subject, Phase: "fast_forward"})
		warning, failed := u.advance(ctx, plan, subject)
		report(domain.PhaseFinished{Subject: subject, Phase: "fast_forward", Failed: failed})

		// A fast-forward cut short by a cancel is not worth a warning: the run reports the interruption.
		if failed && ctx.Err() == nil {
			warnings = append(warnings, warning)
		}
	}

	return warnings
}

func (u UseCase) advance(ctx context.Context, plan Plan, subject string) (domain.Warning, bool) {
	forwarded := u.Provision.FastForward(ctx, provision.FastForwardParams{Subject: subject, CheckedOutAt: plan.facts.CheckedOutAt(subject)})
	if forwarded.Dirty {
		return domain.Warning{
			Code:    "create.ff_dirty",
			Message: "couldn't fast-forward " + subject + ": its worktree has uncommitted changes — it starts from your local branch",
			Params:  domain.Params{"branch": subject},
		}, true
	}

	return notAdvanced(subject, forwarded.Err)
}

func notAdvanced(subject string, err error) (domain.Warning, bool) {
	if err == nil {
		return domain.Warning{}, false
	}

	return domain.Warning{
		Code:    "create.ff_failed",
		Message: "couldn't fast-forward " + subject + ": " + err.Error() + " — it starts from your local branch",
		Params:  domain.Params{"branch": subject, "cause": err.Error()},
	}, true
}

// item provisions one planned branch between its unit events; a worktree already open is reported as it stands, and a branch reached after a cancel is skipped.
func (u UseCase) item(ctx context.Context, plan Plan, planned BranchPlan, report domain.Report) Item {
	switch {
	case planned.Action == ActionUnchanged:
		return u.unchanged(planned)
	case ctx.Err() != nil:
		return Item{Subject: planned.Branch, Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted}
	}

	report(domain.UnitStarted{Subject: planned.Branch})
	outcome := u.Provision.Run(ctx, u.provisionParams(plan, planned, report))

	item := Item{Subject: planned.Branch, Status: outcome.Status, Reason: outcome.Reason, Err: outcome.Err, Warnings: outcome.Warnings}
	// A worktree whose meta record was written stands, even when a later step of its provisioning failed.
	if outcome.Provisioned.Meta.CreatedAt != "" {
		item.Detail = &Detail{
			Path:     planned.Path,
			Metadata: outcome.Provisioned.Meta,
			Standing: plan.facts.standingOf(planned),
			Ordinal:  outcome.Provisioned.Ordinal,
			EnvPorts: outcome.Provisioned.EnvPorts,
		}
	}

	report(domain.UnitFinished{Subject: planned.Branch, Status: item.Status})
	return item
}

func (u UseCase) unchanged(planned BranchPlan) Item {
	// A record that cannot be read only leaves the detail's metadata empty: the worktree is there all the same.
	recorded, _ := u.Provision.Meta.Load(planned.Branch)
	return Item{Subject: planned.Branch, Status: domain.StatusDone, Reason: domain.ReasonUnchanged, Detail: &Detail{Path: planned.Path, Metadata: recorded}}
}

func (u UseCase) provisionParams(plan Plan, planned BranchPlan, report domain.Report) provision.Params {
	f := plan.facts
	return provision.Params{
		Project: provision.Project{
			MainPath: f.MainPath,
			StateDir: u.StateDir,
			EnvFiles: f.Config.Env.Files,
			OnCreate: f.Config.OnCreate,
			Run:      f.Run,
			RunErr:   f.RunErr,
			Global:   f.Global,
		},
		Worktree: provision.Worktree{
			Branch:     planned.Branch,
			Path:       planned.Path,
			Checkout:   plan.checkout(planned),
			Source:     plan.From,
			ParentPath: f.CheckedOutAt(plan.From),
			Strategy:   plan.EnvFrom,
			Isolation:  plan.isolation,
		},
		Report: report,
	}
}

// remember writes the answers the plan asks to remember, once at least one worktree was created; a write that fails is a warning, the worktrees standing all the same.
func (u UseCase) remember(ctx context.Context, plan Plan, items []Item) []domain.Warning {
	answers := plan.answers()
	if len(answers) == 0 || !slices.ContainsFunc(items, created) {
		return nil
	}

	written, err := u.Answers.Remember(ctx, answers)
	if err != nil {
		return []domain.Warning{{Code: "create.remember_failed", Message: "could not remember the answer: " + err.Error(), Params: domain.Params{"cause": err.Error()}}}
	}
	if written.Unlocked {
		return []domain.Warning{domain.LocksUnavailable}
	}

	return nil
}

// ----------------------------------------------------------------------------
// Helpers

func interrupted(item Item) bool {
	return item.Status == domain.StatusCancelled || item.Reason == domain.ReasonInterrupted
}

func created(item Item) bool {
	return item.Status == domain.StatusDone && item.Reason != domain.ReasonUnchanged
}
