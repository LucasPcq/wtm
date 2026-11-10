// Package configset changes config.toml's settings: each one given replaces the file's, the others and the remembered answers kept unless given, the file written whole and refused when it changed on disk since the preview.
package configset

import (
	"context"
	"errors"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/settings"
)

// ----------------------------------------------------------------------------
// The use case

// Name is the command's name, the one its surfaces and its codes spell; its command line is `config set`.
const Name = "config-set"

// The codes of a repository whose config.toml is missing or does not parse.
const (
	CodeConfigNotFound = Name + domain.ConfigNotFound
	CodeConfigInvalid  = Name + domain.ConfigInvalid
)

// A UseCase changes config.toml through Settings, under Operations; Init is the command that sets wtm up, offered when there is no config.toml, and Edit the one that opens it in an editor, offered when it does not parse.
type UseCase struct {
	Settings   settings.Service
	Git        Git
	Detect     Detect
	Operations operation.Service
	Init       *domain.FollowUp
	Edit       *domain.FollowUp
}

// An Outcome is what Run did: the operation it ran under, the plan and its fingerprint, the warnings, and the Result of the apply, nil when nothing was applied.
type Outcome = domain.Outcome[Plan, Detail]

// A Result is what the apply did: one Item, config.toml, and the follow-up that puts back what it changed.
type Result = domain.Result[Detail]

// An Item is what became of config.toml.
type Item = domain.Unit[Detail]

// ----------------------------------------------------------------------------
// Run

// Run carries out the request under its operation: it plans, fingerprints the plan, stops on a dry run, refuses a stale plan and applies it between the operation's started and finished events. An error returned is a refusal before the apply; a write that fails is in the Outcome's Result.
func (u UseCase) Run(ctx context.Context, req Request, opts domain.RunOptions) (Outcome, error) {
	// 1. Begin the operation: config.toml has its own lock, so no worktree is locked.
	op, err := u.Operations.Begin(Name, nil, opts)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}
	defer op.End()

	// 2. Plan.
	plan, err := u.Plan(ctx, req)
	if err != nil {
		return Outcome{Op: op.ID}, err
	}

	// 3. Fingerprint what would be written, the file it replaces included; a dry run ends here.
	outcome := Outcome{Op: op.ID, Plan: plan, Warnings: plan.warnings(offered(u.Edit, Name+".edit", "fix it in your editor"))}
	outcome.Fingerprint, err = operation.Fingerprint(plan.Fingerprinted())
	if err != nil || opts.DryRun {
		return outcome, err
	}

	// 4. A preview of another file, changed on disk since, is refused rather than written over.
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

// Plan decides what config.toml becomes, changing nothing.
func (u UseCase) Plan(ctx context.Context, req Request) (Plan, error) {
	facts, err := u.Observe(ctx)
	if err != nil {
		return Plan{}, err
	}

	return Decide(facts, req)
}

// ----------------------------------------------------------------------------
// Apply

// CodeWriteFailed fails a write of config.toml.
const CodeWriteFailed = Name + ".write_failed"

// Apply writes config.toml whole, between its unit's events; a plan that changes nothing writes nothing.
func (u UseCase) Apply(ctx context.Context, plan Plan, report domain.Report) Result {
	if plan.Empty {
		return Result{}
	}

	report(domain.UnitStarted{Subject: subject})
	item := Item{Subject: subject, Status: domain.StatusDone, Detail: &Detail{}}
	written, err := u.Settings.Write(ctx, plan.rewrite)
	var newer *domain.SchemaNewer
	switch {
	case errors.Is(err, project.ErrConfigChanged):
		item.Status, item.Err = domain.StatusFailed, &domain.Error{Kind: domain.KindConflict, Code: CodeChangedOnDisk, Message: "config.toml changed on disk since it was read: nothing written", Cause: err}
	case errors.As(err, &newer):
		item.Status, item.Err = domain.StatusFailed, &domain.Error{Kind: domain.KindPrecondition, Code: CodeConfigNewer, Message: newer.Error(), Cause: err}
	case err != nil:
		item.Status, item.Err = domain.StatusFailed, domain.AsError(domain.AsErrorParams{Err: err, Code: CodeWriteFailed, Message: "couldn't write config.toml: " + err.Error()})
	case written.Unlocked:
		item.Warnings = append(item.Warnings, domain.LocksUnavailable)
	}
	report(domain.UnitFinished{Subject: subject, Status: item.Status})

	result := Result{Items: []Item{item}}
	if undo := plan.undo(); item.Status == domain.StatusDone && undo != nil {
		// shortcut: the file is read back after its lock is released, so an edit landing in between is undone over; the store returning the digest it wrote closes it (adapter/config, LUC-332's)
		// an undo the file cannot be read back for is none: unheld, it would write over whatever is there
		if saved, err := u.Settings.Read(); err == nil && saved != nil {
			undo.Answers = append(undo.Answers, domain.Answered{Path: PathReplaces, Values: []string{saved.Digest}})
			result.FollowUps = []domain.FollowUp{*undo}
		}
	}
	return result
}

// ----------------------------------------------------------------------------
// Questions

// Questions are config.toml's settings as the request leaves them, each with the options the repository offers and the problems it would hold.
func (u UseCase) Questions(ctx context.Context, req Request) (Questions, error) {
	facts, err := u.Observe(ctx)
	if err != nil {
		return Questions{}, err
	}

	return NewQuestions(facts, req), nil
}

// ----------------------------------------------------------------------------
// Observe

// Observe reads config.toml as written, refusing none or one that does not parse, and what the repository knows of its settings.
func (u UseCase) Observe(ctx context.Context) (Facts, error) {
	saved, err := u.Settings.Read()
	if invalid, ok := errors.AsType[*project.InvalidConfigError](err); ok {
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeConfigInvalid, Message: "invalid config.toml: " + invalid.Reason.Error(), FollowUp: offered(u.Edit, Name+".edit", "fix it in your editor"), Cause: err}
	}
	if err != nil {
		return Facts{}, err
	}
	if saved == nil {
		return Facts{}, &domain.Error{Kind: domain.KindPrecondition, Code: CodeConfigNotFound, Message: "no wtm config found", FollowUp: offered(u.Init, "repository.init", "set wtm up in this repository"), Cause: project.ErrConfigNotFound}
	}

	branches, err := u.Git.Branches(ctx)
	if err != nil {
		return Facts{}, err
	}
	return Facts{Settings: settings.Facts{Saved: saved, Detected: u.Detect(), Branches: names(branches)}}, nil
}

// names are the branches the base branch can be: local then origin's, each once.
func names(branches worktree.Branches) []string {
	var found []string
	for _, branch := range slices.Concat(branches.Local, branches.Remote) {
		if !slices.Contains(found, branch.Name) {
			found = append(found, branch.Name)
		}
	}

	return found
}

// offered is the follow-up injected, coded and worded by this command; none when nil.
func offered(followUp *domain.FollowUp, code, message string) *domain.FollowUp {
	if followUp == nil {
		return nil
	}
	offer := *followUp
	offer.Code, offer.Message = code, message
	return &offer
}
