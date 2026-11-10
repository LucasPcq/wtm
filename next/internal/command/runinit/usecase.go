// Package runinit sets the run module up in a repository: from its docker-compose.yml, its package scripts and its .env files, it settles what runs, what is shared, each job's port and address, the .env keys that follow them and the profiles, and writes run.toml, docker-compose.yml's rewrite and config.toml's .env files at once.
package runinit

import (
	"context"
	"errors"
	"path/filepath"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/dotenv"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell; its command line is `run init`.
const Name = domain.CommandRunInit

// CodeConfigNotFound refuses a repository with no wtm config: `wtm init` has not run there.
const CodeConfigNotFound = Name + domain.ConfigNotFound

// CodeConfigInvalid refuses a repository whose config.toml does not parse or validate.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// A UseCase sets the run module up in one repository: the files it reads and writes, each through its port, and the services it shares with the other commands.
type UseCase struct {
	RunConfig  RunConfig
	Settings   Settings
	Compose    Compose
	Detector   Detector
	EnvFiles   EnvFiles
	Meta       Meta
	Repository repository.Service
	Operations operation.Service
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied.
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: one Item per file, in the order written, and what follows.
type Result = domain.Result[Detail]

// An Item is what became of one file: its status, the error it failed on, and its Detail.
type Item = domain.Unit[Detail]

// A Detail is, as the JSON envelope reports it, one file the apply wrote: where, and whether git tracks it.
type Detail struct {
	Path    string `json:"path"`
	Tracked bool   `json:"tracked"`
}

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: it plans (under the repository's lock, none on a dry run), fingerprints the plan, stops on a dry run, refuses a stale plan and writes its files between the operation's started and finished events. An error returned is a refusal before the apply.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Begin the operation; a dry run takes no lock.
	op, err := u.Operations.Begin(Name, req.Locks(), opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Plan.
	plan, warnings, err := operation.PlanUnderLocks(ctx, op, func(ctx context.Context) (Plan, []domain.Warning, error) {
		return u.Plan(ctx, req)
	})
	if err != nil {
		return Outcome{Op: op.ID}, err
	}

	// 3. Fingerprint what would be written; a dry run ends here with the plan, its preview and its fingerprint.
	outcome := Outcome{Op: op.ID, Plan: plan, Warnings: warnings}
	outcome.Fingerprint, err = operation.Fingerprint(plan.Fingerprinted())
	if err != nil || opts.DryRun {
		return outcome, err
	}

	// 4. A preview made on other facts is refused rather than applied.
	if err := op.Expect(outcome.Fingerprint); err != nil {
		return outcome, err
	}

	// 5. Write between the started and finished events other processes listen to.
	result := u.Apply(ctx, plan, op.Started())
	op.Finished(result.Failure())

	outcome.Result = &result
	outcome.Warnings = op.Warnings(append(outcome.Warnings, result.Warnings...))
	return outcome, nil
}

// ----------------------------------------------------------------------------
// Plan

// Plan decides what the request writes as the repository stands, writing nothing: the plan and its warnings, or the refusal that stops the request (a *domain.Error, its field errors included).
func (u UseCase) Plan(ctx context.Context, req Request) (Plan, []domain.Warning, error) {
	facts, err := u.Observe(ctx)
	if err != nil {
		return Plan{}, nil, err
	}

	decision, err := Resolve(facts, req)
	if err != nil {
		return Plan{}, nil, err
	}

	return Decide(facts, decision), append(facts.warnings(), decision.toCheck(facts)...), nil
}

// ----------------------------------------------------------------------------
// Apply

// CodeFileChanged fails a run init whose file changed on disk since it was read: nothing is written, and the same answers planned again read it anew.
const CodeFileChanged = Name + ".file_changed"

// Apply writes the plan's files, each a unit between its started and finished events: run.toml is checked as it would be written and each file against the digest it was read with before any is written, then docker-compose.yml, run.toml and config.toml in turn, the plan's order. A file changed since, or a run.toml refused, fails its item and leaves every file as it was. A write that fails after another stops the ones after it: run.toml changed on disk between that check and its write leaves docker-compose.yml rewritten, which reads the variables run.toml declares or will declare, and the same answers write it again.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	items := make([]Item, 0, len(plan.Files))
	for _, file := range plan.Files {
		items = append(items, Item{Subject: file.Path, Status: domain.StatusSkipped, Detail: &Detail{Path: file.Path, Tracked: file.Tracked}})
	}
	// run.toml is checked as SaveRun would before any file is written: a plan it refuses never leaves compose rewritten
	if slices.ContainsFunc(plan.Files, func(file File) bool { return file.Path == runFile }) {
		if err := u.RunConfig.CheckRun(plan.Run); err != nil {
			for i := range items {
				if items[i].Subject == runFile {
					items[i].Status, items[i].Err = domain.StatusFailed, &domain.Error{Kind: domain.KindInternal, Code: CodeWriteFailed, Message: "could not write run.toml", Params: domain.Params{"file": runFile}, Cause: err}
				}
			}
			return Result{Items: items}
		}
	}
	if changed := u.changed(plan); changed != "" {
		for i := range items {
			if items[i].Subject == changed {
				items[i].Status, items[i].Err = domain.StatusFailed, &domain.Error{Kind: domain.KindConflict, Code: CodeFileChanged, Message: changed + " changed on disk since it was read: nothing was written", Params: domain.Params{"file": changed}, FollowUp: &domain.FollowUp{Command: Name, Code: CodeFileChanged, Message: "read it again, the same answers kept"}}
			}
		}
		return Result{Items: items}
	}

	for i, file := range plan.Files {
		if err := ctx.Err(); err != nil {
			return Result{Items: items}
		}
		report(domain.UnitStarted{Subject: file.Path})
		items[i].Status = domain.StatusDone
		if err := u.write(ctx, plan, file); err != nil {
			items[i].Status, items[i].Err = domain.StatusFailed, err
		}
		report(domain.UnitFinished{Subject: file.Path, Status: items[i].Status})
		if items[i].Err != nil {
			return Result{Items: items}
		}
	}
	return Result{Items: items, FollowUps: plan.next()}
}

// changed is the first file of the plan that no longer reads as it did, "" when none.
func (u UseCase) changed(plan Plan) string {
	if saved, err := u.RunConfig.SavedRun(); err != nil || saved.Digest != plan.replaces {
		return runFile
	}
	if plan.compose != nil {
		if compose, err := u.Compose.Compose(); err != nil || compose.Digest != plan.compose.Replaces {
			return plan.compose.File
		}
	}
	if plan.config != nil {
		if saved, err := u.Settings.Saved(); err != nil || saved.Digest != plan.config.Replaces {
			return "config.toml"
		}
	}
	return ""
}

// CodeWriteFailed fails the write of a file of the plan.
const CodeWriteFailed = Name + ".write_failed"

// write writes one file of the plan.
func (u UseCase) write(ctx context.Context, plan Plan, file File) *domain.Error {
	var err error
	switch {
	case file.Path == runFile:
		_, err = u.RunConfig.SaveRun(ctx, project.RunRewrite{Run: plan.Run, Replaces: plan.replaces})
	case file.Tracked:
		_, err = u.Compose.SaveCompose(ctx, *plan.compose)
	default:
		_, err = u.Settings.Save(ctx, *plan.config)
	}
	if errors.Is(err, project.ErrRunChanged) || errors.Is(err, project.ErrComposeChanged) || errors.Is(err, project.ErrConfigChanged) {
		return &domain.Error{Kind: domain.KindConflict, Code: CodeFileChanged, Message: file.Path + " changed on disk since it was read", Params: domain.Params{"file": file.Path}, Cause: err}
	}
	return domain.AsError(domain.AsErrorParams{Err: err, Code: CodeWriteFailed, Message: "could not write " + file.Path, Params: domain.Params{"file": file.Path}})
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form a surface asks for the request, part by part, settled on the repository as it stands, with the request's problems.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx)
	if err != nil {
		return Questions{}, err
	}
	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// CodeRunInvalid refuses a run.toml that does not load: run init rewrites it whole, and would drop what it cannot read.
const CodeRunInvalid = Name + ".run_invalid"

// Observe reads what run init decides from: the repository and its config, run.toml and config.toml as written, docker-compose.yml, the package manifests, every .env file config.toml copies or the repository holds with its lines and its template's, and the ordinal of each worktree. A repository with no wtm config is refused with CodeConfigNotFound.
func (u UseCase) Observe(ctx context.Context) (Facts, error) {
	repo, err := u.Repository.Observe(ctx, Name)
	if err != nil {
		return Facts{}, err
	}
	saved, err := u.RunConfig.SavedRun()
	if err != nil {
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeRunInvalid, Message: "invalid run.toml: " + err.Error(), Cause: err}
	}
	config, err := u.Settings.Saved()
	if err != nil {
		return Facts{}, err
	}
	compose, err := u.Compose.Compose()
	if err != nil {
		return Facts{}, err
	}

	facts := Facts{Project: filepath.Base(repo.MainPath), ProxyPort: repo.Global.ProxyPort(), Run: saved, Config: &config, Compose: compose, Workspace: u.Detector.Workspace(), EnvLines: map[string][]dotenv.Line{}}
	facts.EnvFiles = slices.Clone(config.Config.Env.Files)
	for _, file := range u.Detector.Detect().EnvFiles {
		if !slices.ContainsFunc(facts.EnvFiles, func(known project.EnvFile) bool { return known.Target == file.Target }) {
			facts.EnvFiles = append(facts.EnvFiles, file)
		}
	}
	for _, file := range facts.EnvFiles {
		for _, at := range []string{file.Target, file.Template} {
			// A file that is not there reads no line: its template speaks for it.
			if lines, err := u.EnvFiles.Read(filepath.Join(repo.MainPath, at)); at != "" && err == nil && lines != nil {
				facts.EnvLines[at] = lines
			}
		}
	}
	for _, w := range repo.Snapshot.Worktrees {
		previewable := Previewable{Branch: w.Branch(), Main: w.Main}
		if record, err := u.Meta.Load(previewable.Branch); err == nil && !w.Main {
			previewable.Ordinal = record.Ordinal
		}
		if previewable.Branch != "" {
			facts.Worktrees = append(facts.Worktrees, previewable)
		}
	}
	facts.Taken = u.taken(repo.Snapshot.Worktrees)
	return facts, nil
}

// taken are the ordinals the records hold, as run up counts them: the linked worktrees' and, when one is detached and so names no branch, every record no worktree lists, since it may be the detached one's.
func (u UseCase) taken(worktrees []worktree.Worktree) []int {
	var holders []string
	detached := false
	for _, w := range worktrees {
		_, isDetached := w.State.(worktree.Detached)
		detached = detached || !w.Main && isDetached
		if !w.Main && w.Branch() != "" {
			holders = append(holders, w.Branch())
		}
	}
	if recorded, err := u.Meta.List(); detached && err == nil {
		holders = append(holders, slices.DeleteFunc(recorded, func(branch string) bool { return slices.Contains(holders, branch) })...)
	}
	var taken []int
	for _, branch := range holders {
		if record, err := u.Meta.Load(branch); err == nil {
			taken = append(taken, record.Ordinal)
		}
	}
	return taken
}
