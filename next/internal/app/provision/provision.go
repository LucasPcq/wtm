// Package provision makes a new worktree whole: added with its env and its record, then numbered, its ports settled and its hooks run.
package provision

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/dotenv"
	"github.com/LucasPcq/wtm/next/internal/wtm/env"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
)

type Git interface {
	Worktrees(ctx context.Context) ([]worktree.Worktree, error)
	AddWorktree(ctx context.Context, addition worktree.Addition) error
	RemoveWorktree(ctx context.Context, path string) error
	DeleteBranch(ctx context.Context, name string) error
}

type Meta interface {
	Load(branch string) (worktree.Meta, error)
	Update(ctx context.Context, update worktree.MetaUpdate) (wtm.Written, error)
	LockOrdinals(ctx context.Context, do func() error) (bool, error)
}

type EnvFiles interface {
	Read(path string) ([]dotenv.Line, error)
	Copy(ctx context.Context, request env.CopyRequest) (env.Copied, error)
	Edit(ctx context.Context, edit dotenv.Edit) (wtm.Written, error)
}

type Deps struct {
	Git        Git
	Meta       Meta
	EnvFiles   EnvFiles
	RunHooks   func(ctx context.Context, chain env.HookChain) error
	PublicPort func(ctx context.Context, configured int) int
	Shield     func(ctx context.Context) (context.Context, func())
	Now        func() time.Time
}

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
	Deps     Deps
	Project  Project
	Worktree Worktree
	Report   wtm.Report
}

type Provisioned struct {
	Meta     worktree.Meta
	Ordinal  *int
	EnvPorts env.Redacted
}

type Outcome struct {
	Provisioned Provisioned
	Status      wtm.Status
	Reason      wtm.Reason
	Err         *wtm.Error
	Warnings    []wtm.Warning
}

func Run(ctx context.Context, params Params) Outcome {
	params.Worktree.Isolation = params.Worktree.Isolation.Effective()
	p := &provisioning{Params: params, hookLog: env.HookLogPath(env.HookLogPathParams{StateDir: params.Project.StateDir, Branch: params.Worktree.Branch})}
	p.kept, p.ignored = env.Targets(env.TargetsParams{Run: params.Project.Run, Files: params.Project.EnvFiles})
	if err := ctx.Err(); err != nil {
		return p.outcome(Outcome{Status: wtm.StatusCancelled, Reason: wtm.ReasonInterrupted, Err: interrupted(interruption{Err: err})})
	}
	provisioned, failure := p.add(ctx)
	if failure != nil {
		return p.outcome(Outcome{Status: wtm.StatusFailed, Err: failure})
	}
	p.Report(wtm.WorktreeCreated{Worktree: p.identity(provisioned)})
	outcome := p.complete(ctx, provisioned)
	p.Report(p.provisionedEvent(outcome))
	return outcome
}

type provisioning struct {
	Params
	kept     project.Run
	ignored  []env.Ignored
	hookLog  string
	warnings []wtm.Warning
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
			return p.stopped(ctx, stop{Err: err, Phase: next.name, Provisioned: provisioned})
		}
		if !numbered && provisioned.Ordinal != nil {
			p.Report(wtm.WorktreeNumbered{Worktree: p.identity(provisioned)})
		}
	}
	return p.outcome(Outcome{Status: wtm.StatusDone, Provisioned: provisioned})
}

func (p *provisioning) do(ctx context.Context, next phase) error {
	p.Report(wtm.PhaseStarted{Subject: p.Worktree.Branch, Phase: next.name, Log: next.log})
	err := next.run(ctx)
	p.Report(wtm.PhaseFinished{Subject: p.Worktree.Branch, Phase: next.name, Failed: err != nil})
	return err
}

type stop struct {
	Err         error
	Phase       string
	Provisioned Provisioned
}

func (p *provisioning) stopped(ctx context.Context, s stop) Outcome {
	params := wtm.Params{"phase": s.Phase}
	failure := wtm.AsError(wtm.AsErrorParams{Err: s.Err, Code: "unit.phase_failed", Message: s.Phase + " failed", Params: params})
	status := wtm.StatusFailed
	if ctx.Err() != nil {
		failure, status = interrupted(interruption{Err: s.Err, Params: params}), wtm.StatusCancelled
	}
	return p.outcome(Outcome{Status: status, Reason: wtm.ReasonIncomplete, Err: failure, Provisioned: s.Provisioned})
}

type interruption struct {
	Err    error
	Params wtm.Params
}

func interrupted(i interruption) *wtm.Error {
	return &wtm.Error{Kind: wtm.KindCancelled, Code: wtm.CodeInterrupted, Message: "interrupted", Params: i.Params, Cause: i.Err}
}

func (p *provisioning) outcome(outcome Outcome) Outcome {
	outcome.Warnings = p.warnings
	if p.unlocked {
		outcome.Warnings = append(outcome.Warnings, wtm.LocksUnavailable)
	}
	return outcome
}

func (p *provisioning) warn(warning wtm.Warning) {
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

func (p *provisioning) provisionedEvent(outcome Outcome) wtm.WorktreeProvisioned {
	event := wtm.WorktreeProvisioned{Worktree: p.identity(outcome.Provisioned), OK: outcome.Status == wtm.StatusDone}
	if failure, ok := errors.AsType[*env.HookFailure](outcome.Err); ok {
		event.Hook, event.ExitCode = failure.Cmd, failure.ExitCode
	}
	return event
}

func (p *provisioning) add(ctx context.Context) (Provisioned, *wtm.Error) {
	shielded, release := p.Deps.Shield(ctx)
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

func stepFailed(step string, err error) *wtm.Error {
	return wtm.AsError(wtm.AsErrorParams{Err: err, Code: "unit.step_failed", Message: step + " failed", Params: wtm.Params{"step": step}})
}

func (p *provisioning) addWorktree(ctx context.Context) error {
	return p.Deps.Git.AddWorktree(ctx, worktree.Addition{Path: p.Worktree.Path, Checkout: p.Worktree.Checkout})
}

func (p *provisioning) meta() worktree.Meta {
	return worktree.Meta{
		SourceBranch: p.Worktree.Source,
		CreatedAt:    p.Deps.Now().UTC().Format(time.RFC3339),
		EnvStrategy:  p.Worktree.Strategy,
		Isolation:    p.Worktree.Isolation,
	}
}

func (p *provisioning) copyEnv(ctx context.Context) error {
	if len(p.Project.EnvFiles) == 0 {
		return nil
	}
	copied, err := p.Deps.EnvFiles.Copy(ctx, env.CopyRequest{
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

func copyWarning(warning env.CopyWarning) wtm.Warning {
	switch w := warning.(type) {
	case env.NoTemplate:
		return wtm.Warning{Code: "provision.env_no_template", Message: "no template found for " + w.Target + ", skipping", Params: wtm.Params{"target": w.Target}}
	case env.SourceMissing:
		return wtm.Warning{Code: "provision.env_source_missing", Message: "file " + w.Target + " not found in " + w.Dir + ", skipping", Params: wtm.Params{"target": w.Target, "dir": w.Dir}}
	case env.ParentFallback:
		return wtm.Warning{Code: "provision.env_parent_fallback", Message: "file " + w.Target + " not found in the parent worktree, copied from main", Params: wtm.Params{"target": w.Target}}
	}
	return wtm.Warning{}
}

func (p *provisioning) record(ctx context.Context, meta worktree.Meta) error {
	written, err := p.Deps.Meta.Update(ctx, worktree.MetaUpdate{
		Branch: p.Worktree.Branch,
		Change: func(stored *worktree.Meta) error {
			*stored = meta
			return nil
		},
	})
	p.unlocked = p.unlocked || written.Unlocked
	return err
}

func (p *provisioning) undo(ctx context.Context, failure *wtm.Error) *wtm.Error {
	if err := p.Deps.Git.RemoveWorktree(ctx, p.Worktree.Path); err != nil {
		return p.leftBehind(leftBehind{Failure: failure, Err: err, Worktree: true})
	}
	name, created := worktree.CreatedBranch(p.Worktree.Checkout)
	if !created {
		return failure
	}
	if err := p.Deps.Git.DeleteBranch(ctx, name); err != nil {
		return p.leftBehind(leftBehind{Failure: failure, Err: err})
	}
	return failure
}

type leftBehind struct {
	Failure  *wtm.Error
	Err      error
	Worktree bool
}

type CleanRequest struct {
	Branches []string `json:"branches"`
	Force    bool     `json:"force"`
}

func (p *provisioning) leftBehind(left leftBehind) *wtm.Error {
	branch, path := p.Worktree.Branch, p.Worktree.Path
	request, _ := json.Marshal(CleanRequest{Branches: []string{branch}, Force: true})
	clean := "remove it with `wtm clean " + branch + " --force`"
	message, kept := "could not delete the branch, left behind: branch", "branch"
	followUp := &wtm.FollowUp{Message: "branch " + branch + " is left behind: " + clean, Params: wtm.Params{"branch": branch}}
	if left.Worktree {
		message, kept = "could not remove the worktree, left behind: worktree", "worktree"
		followUp = &wtm.FollowUp{Message: "branch " + branch + " is left half-created at " + path + ": " + clean, Params: wtm.Params{"branch": branch, "path": path}}
	}
	followUp.Command, followUp.Request, followUp.Code = "clean", request, "provision.left_behind"
	return &wtm.Error{
		Kind:     wtm.KindInternal,
		Code:     "unit.undo_failed",
		Message:  message,
		Params:   wtm.Params{"left": kept},
		FollowUp: followUp,
		Cause:    errors.Join(left.Failure, left.Err),
	}
}
