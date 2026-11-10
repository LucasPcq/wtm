// Package initialize is init: it sets wtm up in a repository, or sets it up again, writing config.toml whole from its settings, each filled from what the repository suggests on a first init and from config.toml on a re-init.
package initialize

import (
	"cmp"
	"context"
	"errors"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/settings"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell.
const Name = domain.CommandInit

// CodeConfigInvalid refuses a re-init of a config.toml that does not parse, or that remembers an answer init cannot keep: it is edited by hand first.
const CodeConfigInvalid = Name + domain.ConfigInvalid

// A UseCase sets wtm up in one repository: the ports it reads the repository through, the one that registers it, and the services it shares with the other commands. MainPath is the main worktree, ConfigPath where config.toml is.
type UseCase struct {
	Git        Git
	Detect     Detect
	RunConfig  RunConfig
	Register   Register
	Settings   settings.Service
	Operations operation.Service
	MainPath   string
	ConfigPath string
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied (a dry run, or a refusal before the apply).
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: one Item, config.toml, and the steps after it.
type Result = domain.Result[Detail]

// An Item is what became of config.toml: written, unchanged, or the error it failed on.
type Item = domain.Unit[Detail]

// A Detail describes config.toml as the JSON envelope reports it: where it is, and whether this run created it.
type Detail struct {
	Path    string `json:"path"`
	Created bool   `json:"created"`
}

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: it takes the repository's lock (none on a dry run, which plans once), plans under it, fingerprints the plan, stops on a dry run, refuses a stale plan and applies it between the operation's started and finished events. An error returned is a refusal before the apply; a failure during the apply is in the Outcome's Result.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Begin the operation, which takes the repository's lock before anything is read; a dry run takes none.
	op, err := u.Operations.Begin(Name, req.Locks(), opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Plan under the lock.
	plan, warnings, err := operation.PlanUnderLocks(ctx, op, func(ctx context.Context) (Plan, []domain.Warning, error) {
		return u.Plan(ctx, req)
	})
	if err != nil {
		return Outcome{Op: op.ID}, err
	}

	// 3. Fingerprint what would be written, config.toml as it stands included; a dry run ends here with the plan and its fingerprint.
	outcome := Outcome{Op: op.ID, Plan: plan, Warnings: warnings}
	outcome.Fingerprint, err = operation.Fingerprint(plan.Fingerprinted())
	if err != nil || opts.DryRun {
		return outcome, err
	}

	// 4. A preview made on another config.toml is refused rather than applied.
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

// Plan decides what the request writes, changing nothing: the plan, or the refusal that stops the request (a *domain.Error, field errors included).
func (u UseCase) Plan(ctx context.Context, req Request) (Plan, []domain.Warning, error) {
	facts, err := u.Observe(ctx)
	if err != nil {
		return Plan{}, nil, err
	}

	resolved, err := Resolve(facts, req)
	if err != nil {
		return Plan{}, nil, err
	}

	planned, warnings := Decide(facts, resolved)
	planned.Recap, planned.Empty = planned.recap(), planned.empty()
	return planned, warnings, nil
}

// ----------------------------------------------------------------------------
// Apply

// The failures of the write; config.toml is left as it was.
const (
	CodeChangedOnDisk = Name + ".changed_on_disk"
	CodeWriteFailed   = Name + ".write_failed"
)

// Apply writes config.toml whole, then registers the repository among those wtm knows; a plan that changes nothing writes nothing. It ends on the steps after: create a worktree, relocate those made elsewhere, set up run.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	item := Item{Subject: plan.Path, Status: domain.StatusDone, Detail: &Detail{Path: plan.Path, Created: plan.Created}}
	if plan.Empty {
		item.Reason = domain.ReasonUnchanged
		return Result{Items: []Item{item}, FollowUps: plan.next()}
	}
	if ctx.Err() != nil {
		return Result{Items: []Item{{Subject: plan.Path, Status: domain.StatusSkipped, Reason: domain.ReasonInterrupted}}}
	}

	report(domain.UnitStarted{Subject: plan.Path})
	if _, err := u.Settings.Write(ctx, plan.rewrite); err != nil {
		item = Item{Subject: plan.Path, Status: domain.StatusFailed, Err: writeFailure(plan, err)}
		report(domain.UnitFinished{Subject: plan.Path, Status: item.Status})
		return Result{Items: []Item{item}}
	}
	report(domain.UnitFinished{Subject: plan.Path, Status: item.Status})

	u.Register(ctx)
	return Result{Items: []Item{item}, FollowUps: plan.next()}
}

// writeFailure is why config.toml could not be written: changed on disk since it was read, a folder the user may not write (named as they find it, from the main worktree), or the error the write failed on (a newer wtm's file among them, its cause the *domain.SchemaNewer that exits 22).
func writeFailure(plan Plan, err error) *domain.Error {
	path := plan.Path
	params := domain.Params{"path": path, "cause": err.Error()}
	if errors.Is(err, fs.ErrPermission) {
		message := "permission denied on " + plan.shownPath + ": check who owns " + plan.shown(deniedIn(plan.Path, err)) + ", then set it up again"
		return &domain.Error{Kind: domain.KindPrecondition, Code: CodeWriteFailed, Message: message, Params: params, Cause: err}
	}
	if errors.Is(err, project.ErrConfigChanged) {
		return &domain.Error{Kind: domain.KindConflict, Code: CodeChangedOnDisk, Message: path + " changed since it was read: run init again", Params: params, FollowUp: &domain.FollowUp{Command: Name, Code: Name + ".again", Message: "set it up from the file as it is now"}, Cause: err}
	}

	return domain.AsError(domain.AsErrorParams{Err: err, Code: CodeWriteFailed, Message: "couldn't write " + path, Params: params})
}

// deniedIn is the folder a write of path was denied in: the one it could not create a folder in (.git, while .git/wtm does not exist yet), else the one it could not create its file in.
func deniedIn(path string, err error) string {
	if denied, ok := errors.AsType[*fs.PathError](err); ok {
		return filepath.Dir(denied.Path)
	}

	return filepath.Dir(path)
}

// ----------------------------------------------------------------------------
// Questions

// Questions is the form the wizard asks for the request: the settings, each filled as the repository and config.toml stand, and their problems.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// CodeConfigNewer refuses to set up again a config.toml a newer wtm wrote: this one does not rewrite it.
const CodeConfigNewer = Name + ".config_newer"

// CodeEditConfig is the step that opens a config.toml init cannot read in an editor.
const CodeEditConfig = Name + ".edit_config"

func editConfig() *domain.FollowUp {
	return &domain.FollowUp{Command: "config edit", Code: CodeEditConfig, Message: "fix it by hand"}
}

// Observe reads what init decides from: config.toml as written, refused with CodeConfigInvalid when it does not parse or remembers an answer init cannot keep, CodeConfigNewer when a newer wtm wrote it; the repository's worktrees and branches, origin's default branch (else the main worktree's), what its files suggest, and whether run.toml declares anything (an unreadable one counts as declared: run init is not what it needs).
func (u UseCase) Observe(ctx context.Context) (Facts, error) {
	saved, err := u.Settings.Read()
	if invalid, ok := errors.AsType[*project.InvalidConfigError](err); ok {
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeConfigInvalid, Message: "invalid config.toml: " + invalid.Reason.Error(), FollowUp: editConfig(), Cause: err}
	}
	if err != nil {
		return Facts{}, err
	}
	if saved != nil && saved.Newer() {
		newer := &domain.SchemaNewer{Path: u.ConfigPath, Found: saved.Schema, Supported: project.ConfigSchema}
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeConfigNewer, Message: newer.Error(), Cause: newer}
	}
	// init writes the remembered answers back as they are: one it cannot keep is fixed by hand first
	if problems := settings.RememberedProblems(Name, saved); len(problems) > 0 {
		messages := make([]string, 0, len(problems))
		for _, problem := range problems {
			messages = append(messages, problem.Message(problem.Path))
		}
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeConfigInvalid, Message: "invalid config.toml: " + strings.Join(messages, "; "), FollowUp: editConfig()}
	}

	snapshot, err := u.Git.Snapshot(ctx)
	if err != nil {
		return Facts{}, err
	}
	head, err := u.Git.OriginHead(ctx)
	if err != nil {
		return Facts{}, err
	}

	var branches []string
	for _, branch := range snapshot.Branches.Local {
		branches = append(branches, branch.Name)
	}
	for _, branch := range snapshot.Branches.Remote {
		if !snapshot.Local(branch.Name) {
			branches = append(branches, branch.Name)
		}
	}
	for _, w := range snapshot.Worktrees {
		if w.Path == u.MainPath {
			head = cmp.Or(head, w.Branch())
		}
	}

	run, runErr := u.RunConfig.LoadRun()
	return Facts{
		Settings:    settings.Facts{Saved: saved, Detected: u.Detect(), Branches: branches, Head: head},
		MainPath:    u.MainPath,
		ConfigPath:  u.ConfigPath,
		Worktrees:   snapshot.Worktrees,
		RunDeclared: runErr != nil || run.Declared,
	}, nil
}
