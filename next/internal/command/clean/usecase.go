// Package clean removes worktrees, their branches and their meta records: what the request settles on, what refuses an unsafe one, then each removed in turn, its on_clean hooks first, and the children it leaves behind reparented or named.
package clean

import (
	"context"
	"path/filepath"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
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
	EnvFiles     EnvFiles
	Jobs         Jobs
	PullRequests PullRequests
	RunHooks     RunHooks
	Shield       Shield
	StateDir     string // wtm's state, where each worktree's hook log goes
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
	outcome.Plan = plan.applied()
	outcome.Warnings = op.Warnings(append(outcome.Warnings, result.Warnings...))
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

// Apply carries out the plan, reporting each step: it removes each worktree in turn, a failure stopping none of the others, reports each name that is no worktree as already absent, then reparents the children of the worktrees removed, or warns of each one left orphaned; it ends on the step out of a worktree removed under the caller's feet. Once ctx is cancelled, the worktrees not yet started are skipped.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	items := make([]Item, 0, len(plan.Worktrees)+len(plan.Absent))
	for _, removal := range plan.Worktrees {
		items = append(items, u.remove(ctx, plan, removal, report))
	}
	for _, name := range plan.Absent {
		items = append(items, Item{Subject: name, Status: domain.StatusDone, Reason: domain.ReasonUnchanged})
	}

	moves := slices.DeleteFunc(slices.Clone(plan.Moves), func(move Move) bool { return !removedIn(items, move.From) })
	if !plan.ReparentChildren {
		return Result{Items: items, Warnings: orphaned(moves), FollowUps: plan.next(items)}
	}

	return Result{Items: items, Warnings: u.reparent(ctx, plan, moves, report), FollowUps: plan.next(items)}
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

// Observe reads what clean decides from: the project's config, the repository's snapshot, the meta record of each worktree, and, for each worktree req asks, what makes it unsafe to remove: its uncommitted changes, the commits no other branch holds, its open pull request (the forge asked only of the branches origin has) and its jobs up, the forge and the run daemon asked while git reads. A repository with no wtm config is refused with CodeConfigNotFound.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	facts := Facts{Repository: repo, Records: map[string]worktree.Meta{}, Checks: map[string]Check{}}
	if facts.Located, err = u.locate(ctx, facts, req); err != nil {
		return Facts{}, err
	}
	for _, w := range repo.Worktrees {
		// A record that cannot be read is no parent to its children and no data kept: the worktree is removed all the same.
		if record, err := u.Meta.Load(w.Branch()); w.Branch() != "" && err == nil {
			facts.Records[w.Branch()] = record
		}
	}

	asked := facts.asked(req.Worktrees)
	if len(asked) == 0 {
		return facts, nil
	}

	var gitErr error
	pushed := facts.pushed(asked)
	answered := repository.Ask(ctx, repository.AskParams{Jobs: u.Jobs, PullRequests: u.PullRequests, Branches: pushed, WithPRs: len(pushed) > 0}, func() {
		gitErr = u.check(ctx, asked, facts.Checks)
	})
	if gitErr != nil {
		return Facts{}, repository.Interrupted(ctx, gitErr)
	}
	if answered.Interrupted {
		return Facts{}, ctx.Err()
	}

	facts.JobsErr, facts.PRsErr = answered.JobsErr, answered.PRsErr
	for _, w := range asked {
		check := facts.Checks[w.Name()]
		check.Jobs = run.UpIn(answered.Jobs, w.Path)
		check.PullRequest = openPullRequest(answered.PullRequests, w.Branch())
		facts.Checks[w.Name()] = check
	}

	return facts, nil
}

// locate resolves each name req asks that is no worktree's name as a path, from where the caller stands, through its symbolic links as git spells a worktree's path.
func (u UseCase) locate(ctx context.Context, facts Facts, req Request) (map[string]string, error) {
	located := map[string]string{}
	for _, name := range req.Worktrees {
		if _, named := facts.named(name); named || strings.TrimSpace(name) == "" {
			continue
		}

		path := name
		if !filepath.IsAbs(path) && req.Caller.Cwd != "" {
			path = filepath.Join(req.Caller.Cwd, path)
		}
		location, err := u.Git.Locate(ctx, path)
		if err != nil {
			return nil, err
		}
		if location.Exists {
			located[name] = location.Path
		}
	}

	return located, nil
}

// check reads, for each worktree, its uncommitted changes, none once its directory is gone, and the commits of its branch no other branch holds.
func (u UseCase) check(ctx context.Context, worktrees []worktree.Worktree, checks map[string]Check) error {
	for _, w := range worktrees {
		var check Check
		var err error
		if !w.Prunable {
			if check.Dirty, err = u.Git.Dirty(ctx, w.Path); err != nil {
				return err
			}
		}
		if w.Branch() != "" {
			if check.Unpushed, err = u.Git.Unpushed(ctx, w.Branch()); err != nil {
				return err
			}
		}
		checks[w.Name()] = check
	}

	return nil
}

// ----------------------------------------------------------------------------
// The steps of Apply

// remove removes one worktree between its unit events: its on_clean hooks, which refuse its removal when one fails, then the worktree, its branch and its meta record, shielded from a cancel, then the event that names it gone. A worktree reached after a cancel is skipped.
func (u UseCase) remove(ctx context.Context, plan Plan, removal Removal, report domain.Report) (item Item) {
	item.Subject = removal.Worktree
	if ctx.Err() != nil {
		item.Status, item.Reason = domain.StatusSkipped, domain.ReasonInterrupted
		return item
	}

	report(domain.UnitStarted{Subject: removal.Worktree})
	defer func() { report(domain.UnitFinished{Subject: item.Subject, Status: item.Status}) }()

	warnings, err := u.hooks(ctx, plan, removal, report)
	item.Warnings = warnings
	if err != nil {
		item.Status, item.Err = domain.StatusFailed, failure("clean.hook_failed", "on_clean hook failed", err)
		if ctx.Err() != nil {
			item.Status, item.Reason = domain.StatusCancelled, domain.ReasonInterrupted
		}
		return item
	}

	// Shielded: a cancel half-way would strand a branch and a record with no worktree, and an event nobody publishes.
	shielded, release := u.Shield(ctx)
	defer release()
	report(domain.PhaseStarted{Subject: removal.Worktree, Phase: domain.PhaseRemove})
	removed := u.removeWorktree(shielded, plan, removal, report)
	report(domain.PhaseFinished{Subject: removal.Worktree, Phase: domain.PhaseRemove, Failed: removed.Status != domain.StatusDone})

	removed.Warnings = append(item.Warnings, removed.Warnings...)
	item = removed
	return item
}

// hooks runs the on_clean hooks in the worktree, their variables expanded and the worktree's run environment set, between their phase's events; a worktree whose directory is gone runs none, with a warning.
func (u UseCase) hooks(ctx context.Context, plan Plan, removal Removal, report domain.Report) ([]domain.Warning, error) {
	configured := plan.facts.Config.OnClean
	switch {
	case len(configured) == 0:
		return nil, nil
	case removal.Missing:
		return []domain.Warning{{Code: "clean.hooks_skipped", Message: "its directory is gone: the on_clean hooks did not run", Params: domain.Params{"path": removal.Path}}}, nil
	}

	record := plan.facts.Records[removal.Branch]
	vars := env.HookVars{Worktree: removal.Path, Branch: removal.Branch, Root: plan.facts.MainPath, FromBranch: record.SourceBranch}
	hooks := make([]project.Hook, 0, len(configured))
	for _, hook := range configured {
		hooks = append(hooks, vars.Hook(hook))
	}

	log := env.HookLogPath(env.HookLogPathParams{StateDir: u.StateDir, Branch: removal.Branch, Hooks: env.OnCleanHooks})
	hookEnv, warnings := u.hookEnv(plan, removal, record)
	report(domain.PhaseStarted{Subject: removal.Worktree, Phase: domain.PhaseOnClean, Log: log})
	err := u.RunHooks(ctx, env.HookChain{
		Hooks:   hooks,
		WorkDir: removal.Path,
		Env:     hookEnv,
		Unset:   env.JobKeys(),
		Output:  func(line string) { report(domain.Output{Subject: removal.Worktree, Line: line}) },
		Log:     log,
	})
	report(domain.PhaseFinished{Subject: removal.Worktree, Phase: domain.PhaseOnClean, Failed: err != nil})
	return warnings, err
}

// hookEnv is the run environment the hooks get, as the worktree's own jobs get it (its Compose project, its ports): nil when they read none, or with a warning when a compose env file cannot be read.
func (u UseCase) hookEnv(plan Plan, removal Removal, record worktree.Meta) (map[string]string, []domain.Warning) {
	isolation := record.Isolation.Effective()
	if !env.HooksReadRun(plan.facts.Run, isolation) {
		return nil, nil
	}

	var compose [][]dotenv.Line
	for _, path := range env.ComposeEnvPaths(plan.facts.Run) {
		lines, err := u.EnvFiles.Read(filepath.Join(removal.Path, path))
		if err != nil {
			return nil, []domain.Warning{{Code: "clean.hook_env_unavailable", Message: err.Error() + " — on_clean hooks run without the worktree's environment", Params: domain.Params{"cause": err.Error()}}}
		}
		compose = append(compose, lines)
	}

	return env.HookEnv(env.JobParams{Branch: removal.Branch, ProjectDir: plan.facts.MainPath, Ordinal: record.Ordinal, Isolation: isolation, Run: plan.facts.Run}, compose), nil
}

// removeWorktree removes the worktree, then its branch and its meta record, and reports it removed. A worktree git dropped while files it could not delete stay on disk (a container's, owned by root) is removed all the same, those files named in a warning.
func (u UseCase) removeWorktree(ctx context.Context, plan Plan, removal Removal, report domain.Report) Item {
	item := Item{Subject: removal.Worktree}
	if err := u.Git.RemoveWorktree(ctx, removal.Path); err != nil {
		if u.listed(ctx, removal.Path) {
			item.Status, item.Err = domain.StatusFailed, failure("clean.remove_failed", "couldn't remove the worktree", err)
			return item
		}
		item.Warnings = append(item.Warnings, domain.Warning{Code: "clean.left_on_disk", Message: removal.Path + " is still on disk (" + err.Error() + "): delete what is left with sudo rm -rf " + removal.Path, Params: domain.Params{"path": removal.Path}})
	}

	report(domain.WorktreeRemoved{Worktree: identity(removal, plan.facts.Records[removal.Branch])})
	item.Status, item.Detail = domain.StatusDone, &Detail{Path: removal.Path}
	if removal.Branch == "" {
		return item
	}

	if err := u.Git.DeleteBranch(ctx, removal.Branch); err != nil {
		item.Status, item.Reason, item.Err = domain.StatusFailed, domain.ReasonIncomplete, failure("clean.branch_kept", "the worktree is removed but its branch "+removal.Branch+" is kept", err)
		return item
	}
	if err := u.Meta.Delete(removal.Branch); err != nil {
		item.Warnings = append(item.Warnings, domain.Warning{Code: "clean.meta_kept", Message: "couldn't delete the meta record of " + removal.Branch + ": " + err.Error(), Params: domain.Params{"branch": removal.Branch}})
	}

	return item
}

// listed reports whether git still lists a worktree at path; a list git cannot give counts as listed, so nothing is taken as removed that may not be.
func (u UseCase) listed(ctx context.Context, path string) bool {
	worktrees, err := u.Git.Worktrees(ctx)
	return err != nil || slices.ContainsFunc(worktrees, func(w worktree.Worktree) bool { return w.Path == path })
}

// reparent records each child's new parent, reporting it updated; a child whose record cannot be written stays orphaned, with a warning.
func (u UseCase) reparent(ctx context.Context, plan Plan, moves []Move, report domain.Report) []domain.Warning {
	var warnings []domain.Warning
	for _, move := range moves {
		var record worktree.Meta
		_, err := u.Meta.Update(ctx, worktree.MetaUpdate{Branch: move.Branch, Change: func(meta *worktree.Meta) error {
			meta.SourceBranch = move.To
			record = *meta
			return nil
		}})
		if err != nil {
			warning := orphaned([]Move{move})[0]
			warning.Message += ": couldn't reparent it (" + err.Error() + ")"
			warnings = append(warnings, warning)
			continue
		}

		child, _ := plan.facts.named(move.Branch)
		report(domain.WorktreeUpdated{Worktree: identity(Removal{Branch: move.Branch, Path: child.Path}, record), Changed: []worktree.IdentityField{worktree.IdentityParent}})
	}

	return warnings
}

// ----------------------------------------------------------------------------
// Helpers

// identity is the worktree of removal as its events name it, from its record.
func identity(removal Removal, record worktree.Meta) worktree.Identity {
	identity := worktree.Identity{Branch: removal.Branch, Path: removal.Path, Parent: record.SourceBranch, Isolation: record.Isolation.Effective(), CreatedAt: record.CreatedAt}
	if record.Ordinal > 0 {
		identity.Ordinal = &record.Ordinal
	}

	return identity
}

// openPullRequest is the page of the open pull request of branch among prs, empty when it has none.
func openPullRequest(prs []worktree.PullRequest, branch string) string {
	at := slices.IndexFunc(prs, func(pr worktree.PullRequest) bool {
		return branch != "" && pr.Branch == branch && pr.State == worktree.PullRequestOpen
	})
	if at < 0 {
		return ""
	}

	return prs[at].URL
}

// removedIn reports whether the worktree of branch was removed among items, its branch deleted or not.
func removedIn(items []Item, branch string) bool {
	return slices.ContainsFunc(items, func(item Item) bool { return item.Subject == branch && item.Detail != nil })
}
