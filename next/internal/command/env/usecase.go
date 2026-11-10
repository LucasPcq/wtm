// Package env reconciles worktrees' .env files with their sources and with what wtm writes: each key a source gives and a file lacks added, a key holding another value than its source's overwritten and a key in no source dropped when asked, a missing file rebuilt, then the ports and addresses run.toml links and the values wtm owns settled for the worktree.
package env

import (
	"context"
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	envs "github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = domain.CommandEnv

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// A UseCase reconciles the .env files of worktrees of one repository: the ports it reaches them through and the services it shares with the other commands, provision numbering a worktree and telling the port a named address announces.
type UseCase struct {
	Git        Git
	Meta       Meta
	EnvFiles   Files
	Jobs       Jobs
	Repository repository.Service
	Operations operation.Service
	Provision  provision.Service
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied (a dry run, or a refusal before the apply).
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: one Item per worktree asked, in the plan's order.
type Result = domain.Result[Detail]

// An Item is what became of one worktree asked: its status and why, the error it failed on, its warnings (the jobs to restart among them), and its Detail once reconciled.
type Item = domain.Unit[Detail]

// A Detail describes a worktree reconciled as the JSON envelope reports it: where it is, the files written, and what wtm settled in them, its values withheld unless shown.
type Detail struct {
	Path    string        `json:"path"`
	Written []string      `json:"written"`
	Ports   envs.Redacted `json:"ports"`
}

// Worktree is where the worktree reconciled is and the ports settled in it.
func (d Detail) Worktree() (string, envs.Redacted) {
	return d.Path, d.Ports
}

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: it takes the locks (none on a dry run, which plans once), plans under them, fingerprints the plan, stops on a dry run, refuses a stale plan and applies it between the operation's started and finished events. An error returned is a refusal before the apply; a worktree that fails during the apply is in the Outcome's Result.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Begin the operation, which takes the request's locks before anything is read, so the files cannot change under the plan; a dry run takes none.
	op, err := u.Operations.Begin(Name, req.Locks(), opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Plan; a plan writing a worktree the request did not name, one the command chose, is planned again once that lock is held.
	plan, warnings, err := operation.PlanUnderLocks(ctx, op, func(ctx context.Context) (Plan, []domain.Warning, error) {
		return u.Plan(ctx, req)
	})
	if err != nil {
		return Outcome{Op: op.ID}, err
	}

	// 3. Fingerprint what would be applied, the files as they stand among it; a dry run ends here with the plan, its fingerprint and the jobs its apply would leave on the old values (the apply warns of those it did on their worktree).
	outcome := Outcome{Op: op.ID, Plan: plan, Warnings: warnings}
	outcome.Fingerprint, err = operation.Fingerprint(plan.Fingerprinted())
	if opts.DryRun {
		outcome.Warnings = append(outcome.Warnings, plan.restarts()...)
	}
	if err != nil || opts.DryRun {
		return outcome, err
	}

	// 4. A preview made on other files, one edited since among them, is refused rather than applied.
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

// Plan decides what the request does to the worktrees' .env files: the plan and its warnings, or the refusal that stops the request (a *domain.Error, field errors included).
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
	planned.Recap, planned.Empty = planned.recap(), planned.empty()
	return planned, warnings, err
}

// ----------------------------------------------------------------------------
// Apply

// Apply carries out the plan, reporting each step: it reconciles each worktree that changes in turn, a failure stopping none of the others, and reports the others as they stand. Once ctx is cancelled, the worktrees not yet started are skipped. Those that failed or were skipped are offered again (CodeRetry).
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	items := make([]Item, 0, len(plan.Worktrees))
	for _, r := range plan.Worktrees {
		items = append(items, u.reconcile(ctx, r, plan.ShowValues, report))
	}

	result := Result{Items: items}
	if retry := plan.retry(items); retry != nil {
		for _, item := range items {
			if item.Err != nil {
				item.Err.FollowUp = retry
			}
		}
		result.FollowUps = []domain.FollowUp{*retry}
	}
	return result
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form the wizard asks for the request: the worktrees it may reconcile, the one the caller stands in picked by default and those out of step offered beside it, the switches of the run, and the request's problems as field errors.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx, req)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads what env decides from: the project's config, run.toml and the repository's snapshot, where each name asked as a path leads, the record of each live branch, the port a named address announces, the jobs wtm's daemon lists, and for each worktree on a branch, which the picker shows, whether its directory is gone and its .env files with their template, its parent's copies and the main worktree's. A repository with no wtm config is refused with CodeConfigNotFound.
func (u UseCase) Observe(ctx context.Context, req Request) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}

	records, _ := repository.Records(u.Meta, repo.Live())
	facts := Facts{Repository: repo, Located: map[string]string{}, Records: records, Reads: map[string][]envs.FileRead{}, ReadErrs: map[string]error{}, Missing: map[string]bool{}}
	for _, name := range req.Worktrees {
		if facts.Located[name], err = repository.Located(ctx, u.Git, name, req.Caller); err != nil {
			return Facts{}, err
		}
	}
	if u.Provision.PublicPort != nil {
		facts.PublicPort = u.Provision.PublicPort(ctx, repo.Global.ProxyPort())
	}
	facts.Jobs, facts.JobsErr = u.Jobs(ctx)

	for _, w := range facts.onBranch() {
		if facts.Missing[w.Name()], err = repository.Gone(ctx, u.Git, w); err != nil {
			return Facts{}, err
		}
		if facts.Missing[w.Name()] {
			continue
		}
		parent := ""
		if source := records[w.Branch()].SourceBranch; source != "" {
			parent = repo.CheckedOutAt(source)
		}
		facts.Reads[w.Name()], facts.ReadErrs[w.Name()] = repository.EnvFiles(u.EnvFiles, repository.EnvFilesParams{Files: repo.Config.Env.Files, Dir: w.Path, ParentDir: parent, MainDir: repo.MainPath})
	}

	return facts, nil
}

// ----------------------------------------------------------------------------
// The steps of Apply

// reconcile writes one worktree's files between its unit events, as the plan settled them over the files as they stand, then records the isolation it switched to, once its files are in line with it: an unnumbered worktree is numbered first, its ports then settled on its ordinal; a worktree the plan leaves stands as it is, with no event, and one reached after a cancel is skipped. The jobs up in it that read a file written are warned of, with their restart.
func (u UseCase) reconcile(ctx context.Context, r Reconcile, show bool, report domain.Report) (item Item) {
	item.Subject = r.Worktree
	switch {
	case r.Missing || r.Unreadable != "":
		item.Status, item.Reason = domain.StatusSkipped, domain.ReasonRuledOut
		return item
	case !r.writes():
		item.Status, item.Reason = domain.StatusDone, domain.ReasonUnchanged
		return item
	case ctx.Err() != nil:
		item.Status, item.Reason = domain.StatusSkipped, domain.ReasonInterrupted
		return item
	}

	report(domain.UnitStarted{Subject: r.Worktree})
	defer func() { report(domain.UnitFinished{Subject: item.Subject, Status: item.Status}) }()

	params := r.settle.reconcile
	if !r.Numbered {
		report(domain.PhaseStarted{Subject: r.Worktree, Phase: domain.PhaseOrdinal})
		ordinal, err := u.Provision.Number(ctx, r.Worktree)
		report(domain.PhaseFinished{Subject: r.Worktree, Phase: domain.PhaseOrdinal, Failed: err != nil})
		if err != nil {
			item.Status, item.Err = domain.StatusFailed, domain.AsError(domain.AsErrorParams{Err: err, Code: CodeFailed, Message: "couldn't number " + r.Worktree + ": its ports are unknown", Params: domain.Params{"cause": err.Error()}})
			return item
		}
		resolve := r.settle.resolve
		resolve.Ordinal = ordinal
		params = envs.Settle(envs.SettleParams{Resolve: resolve, Reconcile: params, Numbered: true}).Params
	}

	report(domain.PhaseStarted{Subject: r.Worktree, Phase: domain.PhaseEnv})
	written, reads, err := u.write(ctx, r, params)
	report(domain.PhaseFinished{Subject: r.Worktree, Phase: domain.PhaseEnv, Failed: err != nil})
	if err != nil {
		item.Status, item.Err = domain.StatusFailed, domain.AsError(domain.AsErrorParams{Err: err, Code: CodeFailed, Message: "couldn't write " + r.Worktree + "'s .env files", Params: domain.Params{"cause": err.Error()}})
		return item
	}

	if r.Records != "" {
		_, err := u.Meta.Update(ctx, worktree.MetaUpdate{Branch: r.Worktree, Change: func(meta *worktree.Meta) error {
			meta.Isolation = r.Records
			return nil
		}})
		if err != nil {
			item.Status, item.Err = domain.StatusFailed, domain.AsError(domain.AsErrorParams{Err: err, Code: CodeFailed, Message: "couldn't record " + r.Worktree + "'s isolation", Params: domain.Params{"cause": err.Error()}})
			return item
		}
	}

	params.Reads = reads
	ports := shown(envs.Reconcile(params).Ports, show)
	item.Status, item.Detail = domain.StatusDone, &Detail{Path: r.Path, Written: written, Ports: ports}
	if len(written) == 0 && r.Records == "" {
		item.Reason = domain.ReasonUnchanged
	}
	if len(r.Restart) > 0 && len(written) > 0 {
		item.Warnings = append(item.Warnings, restartWarning(r))
	}
	return item
}

// write edits each file of the worktree as params reconcile it over its lines on disk, and returns the files written and each file as read, its lines on disk when edited.
func (u UseCase) write(ctx context.Context, r Reconcile, params envs.ReconcileParams) ([]string, []envs.FileRead, error) {
	written := []string{}
	reads := make([]envs.FileRead, 0, len(params.Reads))
	for i, read := range params.Reads {
		one := params
		edit := dotenv.Edit{Path: filepath.Join(r.Path, read.File.Target), Create: r.settle.planned.Files[i].Created, Edit: func(current []dotenv.Line) []dotenv.Line {
			read.Child = current
			one.Reads = []envs.FileRead{read}
			file := envs.Reconcile(one).Files[0]
			if !file.Changed() {
				return current
			}
			return file.After
		}}
		changed, err := u.EnvFiles.Edit(ctx, edit)
		if err != nil {
			return nil, nil, err
		}
		if changed.Changed {
			written = append(written, read.File.Target)
		}
		reads = append(reads, read)
	}

	return written, reads, nil
}
