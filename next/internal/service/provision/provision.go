// Package provision makes a new worktree whole: added with its env and its record, then numbered, its ports settled and its hooks run; and fast-forwards the source it starts from.
package provision

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

type Project struct {
	MainPath string
	StateDir string
	EnvFiles []project.EnvFile
	OnCreate []project.Hook
	Run      project.Run
	RunErr   error
	Global   project.Global
}

type Worktree struct {
	Branch     string
	Path       string
	Checkout   worktree.Checkout
	Source     string
	ParentPath string
	Strategy   project.EnvStrategy
	Isolation  project.Isolation
}

type Params struct {
	Project  Project
	Worktree Worktree
	Report   domain.Report
}

type Provisioned struct {
	Meta     worktree.Meta
	Ordinal  *int
	EnvPorts env.Redacted
}

type Outcome struct {
	Provisioned Provisioned
	Status      domain.Status
	Reason      domain.Reason
	Err         *domain.Error
	Warnings    []domain.Warning
}

func (s Service) Run(ctx context.Context, params Params) Outcome {
	params.Worktree.Isolation = params.Worktree.Isolation.Effective()
	p := &provisioning{Params: params, Service: s, hookLog: env.HookLogPath(env.HookLogPathParams{StateDir: params.Project.StateDir, Branch: params.Worktree.Branch})}
	p.kept, p.ignored = env.Targets(params.Project.Run, params.Project.EnvFiles)
	if err := ctx.Err(); err != nil {
		return p.outcome(Outcome{Status: domain.StatusCancelled, Reason: domain.ReasonInterrupted, Err: interrupted(err, nil)})
	}
	provisioned, failure := p.add(ctx)
	if failure != nil {
		return p.outcome(Outcome{Status: domain.StatusFailed, Err: failure})
	}
	p.Report(domain.WorktreeCreated{Worktree: p.identity(provisioned)})
	outcome := p.complete(ctx, provisioned)
	p.Report(p.provisionedEvent(outcome))
	return outcome
}

type provisioning struct {
	Params
	Service  Service
	kept     project.Run
	ignored  []env.Ignored
	hookLog  string
	warnings []domain.Warning
	unlocked bool
}

type phase struct {
	name string
	log  string
	run  func(ctx context.Context) error
}

func (p *provisioning) phases(provisioned *Provisioned) []phase {
	var phases []phase
	if p.needsOrdinal() {
		phases = append(phases, phase{name: "ordinal", run: func(ctx context.Context) error { return p.number(ctx, provisioned) }})
	}
	if p.settlesPorts() {
		phases = append(phases, phase{name: "ports", run: func(ctx context.Context) error { return p.settle(ctx, provisioned) }})
	}
	if len(p.Project.OnCreate) > 0 {
		phases = append(phases, phase{name: "on_create", log: p.hookLog, run: func(ctx context.Context) error { return p.runHooks(ctx, *provisioned) }})
	}
	return phases
}

func (p *provisioning) complete(ctx context.Context, provisioned Provisioned) Outcome {
	for _, next := range p.phases(&provisioned) {
		numbered := provisioned.Ordinal != nil
		err := ctx.Err()
		if err == nil {
			err = p.do(ctx, next)
		}
		if err != nil {
			return p.stopped(ctx, err, next.name, provisioned)
		}
		if !numbered && provisioned.Ordinal != nil {
			p.Report(domain.WorktreeNumbered{Worktree: p.identity(provisioned)})
		}
	}
	return p.outcome(Outcome{Status: domain.StatusDone, Provisioned: provisioned})
}

func (p *provisioning) do(ctx context.Context, next phase) error {
	p.Report(domain.PhaseStarted{Subject: p.Worktree.Branch, Phase: next.name, Log: next.log})
	err := next.run(ctx)
	p.Report(domain.PhaseFinished{Subject: p.Worktree.Branch, Phase: next.name, Failed: err != nil})
	return err
}

func (p *provisioning) stopped(ctx context.Context, err error, phase string, provisioned Provisioned) Outcome {
	params := domain.Params{"phase": phase}
	failure := domain.AsError(domain.AsErrorParams{Err: err, Code: "unit.phase_failed", Message: phase + " failed", Params: params})
	status := domain.StatusFailed
	if ctx.Err() != nil {
		failure, status = interrupted(err, params), domain.StatusCancelled
	}
	return p.outcome(Outcome{Status: status, Reason: domain.ReasonIncomplete, Err: failure, Provisioned: provisioned})
}

func interrupted(err error, params domain.Params) *domain.Error {
	return &domain.Error{Kind: domain.KindCancelled, Code: domain.CodeInterrupted, Message: "interrupted", Params: params, Cause: err}
}

func (p *provisioning) outcome(outcome Outcome) Outcome {
	outcome.Warnings = p.warnings
	if p.unlocked {
		outcome.Warnings = append(outcome.Warnings, domain.LocksUnavailable)
	}
	return outcome
}

func (p *provisioning) warn(warning domain.Warning) {
	p.warnings = append(p.warnings, warning)
}

func (p *provisioning) identity(provisioned Provisioned) worktree.Identity {
	return worktree.Identity{
		Branch:    p.Worktree.Branch,
		Path:      p.Worktree.Path,
		Parent:    provisioned.Meta.SourceBranch,
		Ordinal:   provisioned.Ordinal,
		Isolation: provisioned.Meta.Isolation,
		CreatedAt: provisioned.Meta.CreatedAt,
	}
}

func (p *provisioning) provisionedEvent(outcome Outcome) domain.WorktreeProvisioned {
	event := domain.WorktreeProvisioned{Worktree: p.identity(outcome.Provisioned), OK: outcome.Status == domain.StatusDone}
	if failure, ok := errors.AsType[*env.HookFailure](outcome.Err); ok {
		event.Hook, event.ExitCode = failure.Cmd, failure.ExitCode
	}
	return event
}

func (p *provisioning) add(ctx context.Context) (Provisioned, *domain.Error) {
	shielded, release := p.Service.Shield(ctx)
	defer release()
	if err := p.do(shielded, phase{name: "worktree", run: p.addWorktree}); err != nil {
		return Provisioned{}, stepFailed("worktree", err)
	}
	meta := p.meta()
	if err := p.do(shielded, phase{name: "env", run: p.copyEnv}); err != nil {
		return Provisioned{}, p.undo(shielded, stepFailed("env", err))
	}
	if err := p.do(shielded, phase{name: "meta", run: func(ctx context.Context) error { return p.record(ctx, meta) }}); err != nil {
		return Provisioned{}, p.undo(shielded, stepFailed("meta", err))
	}
	return Provisioned{Meta: meta}, nil
}

func stepFailed(step string, err error) *domain.Error {
	return domain.AsError(domain.AsErrorParams{Err: err, Code: "unit.step_failed", Message: step + " failed", Params: domain.Params{"step": step}})
}

func (p *provisioning) addWorktree(ctx context.Context) error {
	return p.Service.Git.AddWorktree(ctx, worktree.Addition{Path: p.Worktree.Path, Checkout: p.Worktree.Checkout})
}

func (p *provisioning) meta() worktree.Meta {
	return worktree.Meta{
		SourceBranch: p.Worktree.Source,
		CreatedAt:    p.Service.Now().UTC().Format(time.RFC3339),
		EnvStrategy:  p.Worktree.Strategy,
		Isolation:    p.Worktree.Isolation,
	}
}

func (p *provisioning) copyEnv(ctx context.Context) error {
	if len(p.Project.EnvFiles) == 0 {
		return nil
	}
	copied, err := p.Service.EnvFiles.Copy(ctx, env.CopyRequest{
		Strategy:   p.Worktree.Strategy,
		Files:      p.Project.EnvFiles,
		TargetDir:  p.Worktree.Path,
		MainPath:   p.Project.MainPath,
		ParentPath: p.Worktree.ParentPath,
	})
	if err != nil {
		return err
	}
	for _, warning := range copied.Warnings {
		p.warn(copyWarning(warning))
	}
	p.unlocked = p.unlocked || copied.Unlocked
	return nil
}

func copyWarning(warning env.CopyWarning) domain.Warning {
	switch w := warning.(type) {
	case env.NoTemplate:
		return domain.Warning{Code: "provision.env_no_template", Message: "no template found for " + w.Target + ", skipping", Params: domain.Params{"target": w.Target}}
	case env.SourceMissing:
		return domain.Warning{Code: "provision.env_source_missing", Message: "file " + w.Target + " not found in " + w.Dir + ", skipping", Params: domain.Params{"target": w.Target, "dir": w.Dir}}
	case env.ParentFallback:
		return domain.Warning{Code: "provision.env_parent_fallback", Message: "file " + w.Target + " not found in the parent worktree, copied from main", Params: domain.Params{"target": w.Target}}
	}
	return domain.Warning{}
}

func (p *provisioning) record(ctx context.Context, meta worktree.Meta) error {
	written, err := p.Service.Meta.Update(ctx, worktree.MetaUpdate{
		Branch: p.Worktree.Branch,
		Change: func(stored *worktree.Meta) error {
			*stored = meta
			return nil
		},
	})
	p.unlocked = p.unlocked || written.Unlocked
	return err
}

func (p *provisioning) undo(ctx context.Context, failure *domain.Error) *domain.Error {
	if err := p.Service.Git.RemoveWorktree(ctx, p.Worktree.Path); err != nil {
		return p.leftBehind(leftBehind{Failure: failure, Err: err, Worktree: true})
	}
	name, created := worktree.CreatedBranch(p.Worktree.Checkout)
	if !created {
		return failure
	}
	if err := p.Service.Git.DeleteBranch(ctx, name); err != nil {
		return p.leftBehind(leftBehind{Failure: failure, Err: err})
	}
	return failure
}

type leftBehind struct {
	Failure  *domain.Error
	Err      error
	Worktree bool
}

func (p *provisioning) leftBehind(left leftBehind) *domain.Error {
	branch, path := p.Worktree.Branch, p.Worktree.Path
	request, _ := json.Marshal(domain.CleanRequest{Branches: []string{branch}, Force: true})
	clean := "remove it with `wtm clean " + branch + " --force`"
	message, kept := "could not delete the branch, left behind: branch", "branch"
	followUp := &domain.FollowUp{Message: "branch " + branch + " is left behind: " + clean, Params: domain.Params{"branch": branch}}
	if left.Worktree {
		message, kept = "could not remove the worktree, left behind: worktree", "worktree"
		followUp = &domain.FollowUp{Message: "branch " + branch + " is left half-created at " + path + ": " + clean, Params: domain.Params{"branch": branch, "path": path}}
	}
	followUp.Command, followUp.Request, followUp.Code = "clean", request, "provision.left_behind"
	return &domain.Error{
		Kind:     domain.KindInternal,
		Code:     "unit.undo_failed",
		Message:  message,
		Params:   domain.Params{"left": kept},
		FollowUp: followUp,
		Cause:    errors.Join(left.Failure, left.Err),
	}
}
