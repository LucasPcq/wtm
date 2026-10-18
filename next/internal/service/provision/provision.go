// Package provision makes a new worktree whole: added with its env and its record, then numbered, its ports settled and its hooks run; and fast-forwards the source it starts from.
package provision

import (
	"context"
	"errors"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// The service

// Service provisions new worktrees and fast-forwards their sources for the
// commands that create one. Every field must be set before Run.
type Service struct {
	Git      Git
	Meta     Meta
	EnvFiles EnvFiles
	Answers  AnswerStore
	StateDir string // wtm's state, where each worktree's hook log goes

	RunHooks   func(ctx context.Context, chain env.HookChain) error // runs the on_create hooks, failing with an *env.HookFailure
	PublicPort func(ctx context.Context, configured int) int        // the port a named URL announces, given the proxy's configured one; 0 with no proxy
	Shield     func(ctx context.Context) (context.Context, func())  // a context the first cancel does not reach, and its release
	Now        func() time.Time
	LeftBehind func(branch string) []string // the command line that removes a branch an undo left behind, and its worktree
}

// ----------------------------------------------------------------------------
// Request, outcome and Run

// Project is what Run needs of the project a worktree is added to.
type Project struct {
	MainPath string
	EnvFiles []project.EnvFile
	OnCreate []project.Hook
	Run      project.Run
	RunErr   error // why run.toml could not be read; its ports are then left unsettled with a warning
	Global   project.Global
}

// ProjectOf is what Run needs of the project, as repo holds it.
func ProjectOf(repo worktree.Repository) Project {
	return Project{
		MainPath: repo.MainPath,
		EnvFiles: repo.Config.Env.Files,
		OnCreate: repo.Config.OnCreate,
		Run:      repo.Run,
		RunErr:   repo.RunErr,
		Global:   repo.Global,
	}
}

// Worktree is the worktree Run adds: its branch, where it goes, what it checks
// out, and the answers to the questions of a new worktree it is made with.
type Worktree struct {
	Branch     string
	Path       string
	Checkout   worktree.Checkout
	Source     string // the branch it starts from, recorded as its parent
	ParentPath string // the parent's worktree, where a parent env strategy copies from; empty when it has none
	Answers    Answers
}

// Params is one worktree to provision and where its events go.
type Params struct {
	Project  Project
	Worktree Worktree
	Report   domain.Report
}

// Planned is a new worktree as a command's plan places it: its branch, its path, how it starts, the branch it starts from and the answers it is made with.
type Planned struct {
	Branch    string
	Path      string
	Placement worktree.Placed
	From      string
	Answers   Answers
}

// ParamsOf is the Params that provision planned in repo, its events going to report: what it checks out, and the worktree of the branch it starts from.
func ParamsOf(repo worktree.Repository, planned Planned, report domain.Report) Params {
	return Params{
		Project: ProjectOf(repo),
		Worktree: Worktree{
			Branch:     planned.Branch,
			Path:       planned.Path,
			Checkout:   planned.Placement.Checkout(planned.Branch, repo.Ref(planned.From)),
			Source:     planned.From,
			ParentPath: repo.CheckedOutAt(planned.From),
			Answers:    planned.Answers,
		},
		Report: report,
	}
}

// Provisioned is what a worktree was given: its record, its ordinal when it was
// numbered (nil otherwise) and the values settled in its .env files.
type Provisioned struct {
	Meta     worktree.Meta
	Ordinal  *int
	EnvPorts env.Redacted
}

// Stands reports whether the worktree stands: its meta record was written, even when a later phase failed.
func (p Provisioned) Stands() bool {
	return p.Meta.CreatedAt != ""
}

// Outcome is how provisioning a worktree ended. Status is done, failed or
// cancelled; Err is set unless it is done, and Provisioned holds what was
// given before a later phase stopped.
type Outcome struct {
	Provisioned Provisioned
	Status      domain.Status
	Reason      domain.Reason
	Err         *domain.Error
	Warnings    []domain.Warning
}

// Run adds the worktree with its env and its meta record, then numbers it,
// settles its ports and runs its on_create hooks, reporting each phase. The
// add is all or nothing: a failure removes what it made, and only a failed
// removal leaves something behind, named in the error's follow-up. A later
// phase that fails stops the rest and keeps the worktree. Run never returns
// an error of its own: the Outcome carries it.
func (s Service) Run(ctx context.Context, params Params) Outcome {
	if !params.Worktree.Answers.asked {
		return Outcome{Status: domain.StatusFailed, Err: &domain.Error{Kind: domain.KindInternal, Code: "provision.unasked", Message: "the questions of a new worktree were not asked: Answers come from Questions.Answers", Params: domain.Params{"branch": params.Worktree.Branch}}}
	}

	params.Worktree.Answers.isolation = params.Worktree.Answers.isolation.Effective()
	p := &provisioning{Params: params, Service: s, hookLog: env.HookLogPath(env.HookLogPathParams{StateDir: s.StateDir, Branch: params.Worktree.Branch})}
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

// ----------------------------------------------------------------------------
// The fingerprint and the remembered answers

// Fingerprinted is what the fingerprint of a plan that provisions worktrees covers: the plan, the isolation as answers resolve it, and all Run reads of repo's project (its env, its on_create hooks, its run.toml and the global settings), so the apply of a preview made on another project is refused.
func Fingerprinted(plan any, repo worktree.Repository, answers Answers) any {
	return struct {
		Plan      any               `json:"plan"`
		Isolation project.Isolation `json:"isolation"`
		Env       project.Env       `json:"env"`
		OnCreate  []project.Hook    `json:"on_create"`
		Run       project.Run       `json:"run"`
		RunErr    string            `json:"run_err,omitempty"`
		Global    project.Global    `json:"global"`
	}{Plan: plan, Isolation: answers.Isolation(), Env: repo.Config.Env, OnCreate: repo.Config.OnCreate, Run: repo.Run, RunErr: errorText(repo.RunErr), Global: repo.Global}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}

// Remember writes the answers to the fields asked to be remembered, as answers settles them; a write that fails is command's warning <command>.remember_failed, what was made standing all the same.
func (s Service) Remember(ctx context.Context, command string, answers Answers, fields []string) []domain.Warning {
	remembered := answers.Remembered(fields)
	if len(remembered) == 0 {
		return nil
	}

	written, err := s.Answers.Remember(ctx, remembered)
	if err != nil {
		return []domain.Warning{{Code: command + ".remember_failed", Message: "could not remember the answer: " + err.Error(), Params: domain.Params{"cause": err.Error()}}}
	}
	if written.Unlocked {
		return []domain.Warning{domain.LocksUnavailable}
	}

	return nil
}

// ----------------------------------------------------------------------------
// A run and its phases

// provisioning is one run of Run: its params, the run.toml links that apply
// to the project's env files and those that name none, and the warnings and
// lock state it gathers on the way.
type provisioning struct {
	Params
	Service  Service
	kept     project.Run
	ignored  []env.Ignored
	hookLog  string
	warnings []domain.Warning
	unlocked bool
}

// A phase is one step after the add, reported as it starts and finishes; log
// is the file its output goes to, if any.
type phase struct {
	name string
	log  string
	run  func(ctx context.Context) error
}

// phases returns the phases this worktree needs, in order; each one records
// what it gives into provisioned.
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

// complete runs the phases in order and stops at the first that fails or finds
// ctx cancelled.
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

// do runs next between its started and finished events.
func (p *provisioning) do(ctx context.Context, next phase) error {
	p.Report(domain.PhaseStarted{Subject: p.Worktree.Branch, Phase: next.name, Log: next.log})
	err := next.run(ctx)
	p.Report(domain.PhaseFinished{Subject: p.Worktree.Branch, Phase: next.name, Failed: err != nil})
	return err
}

// stopped is the outcome of a phase that failed with err: cancelled when ctx
// was, failed otherwise, incomplete either way.
func (p *provisioning) stopped(ctx context.Context, err error, phase string, provisioned Provisioned) Outcome {
	params := domain.Params{"phase": phase}
	failure := domain.AsError(domain.AsErrorParams{Err: err, Code: "unit.phase_failed", Message: phase + " failed", Params: params})

	status := domain.StatusFailed
	if ctx.Err() != nil {
		failure, status = interrupted(err, params), domain.StatusCancelled
	}
	return p.outcome(Outcome{Status: status, Reason: domain.ReasonIncomplete, Err: failure, Provisioned: provisioned})
}

// interrupted is the cancellation error of a run cut short by err.
func interrupted(err error, params domain.Params) *domain.Error {
	return &domain.Error{Kind: domain.KindCancelled, Code: domain.CodeInterrupted, Message: "interrupted", Params: params, Cause: err}
}

// outcome returns outcome with the run's warnings, and domain.LocksUnavailable
// when a write went unlocked.
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

// identity is the worktree as its events name it.
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

// provisionedEvent is the event that ends a worktree's provisioning, naming the
// hook that failed, if one did.
func (p *provisioning) provisionedEvent(outcome Outcome) domain.WorktreeProvisioned {
	event := domain.WorktreeProvisioned{Worktree: p.identity(outcome.Provisioned), OK: outcome.Status == domain.StatusDone}

	if failure, ok := errors.AsType[*env.HookFailure](outcome.Err); ok {
		event.Hook, event.ExitCode = failure.Cmd, failure.ExitCode
	}
	return event
}

// ----------------------------------------------------------------------------
// The add: worktree, env, record

// add adds the worktree, copies its env and writes its record under a shield,
// so the first Ctrl+C lets the three finish together; a failed step undoes the
// steps before it.
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

// stepFailed is the error of a step of the add that failed with err.
func stepFailed(step string, err error) *domain.Error {
	return domain.AsError(domain.AsErrorParams{Err: err, Code: "unit.step_failed", Message: step + " failed", Params: domain.Params{"step": step}})
}

func (p *provisioning) addWorktree(ctx context.Context) error {
	return p.Service.Git.AddWorktree(ctx, worktree.Addition{Path: p.Worktree.Path, Checkout: p.Worktree.Checkout})
}

// meta is the record of the new worktree, created now.
func (p *provisioning) meta() worktree.Meta {
	return worktree.Meta{
		SourceBranch: p.Worktree.Source,
		CreatedAt:    p.Service.Now().UTC().Format(time.RFC3339),
		EnvStrategy:  p.Worktree.Answers.strategy,
		Isolation:    p.Worktree.Answers.isolation,
	}
}

// copyEnv copies the project's env files into the worktree by its strategy,
// keeping what the copy skipped as warnings.
func (p *provisioning) copyEnv(ctx context.Context) error {
	if len(p.Project.EnvFiles) == 0 {
		return nil
	}

	copied, err := p.Service.EnvFiles.Copy(ctx, env.CopyRequest{
		Strategy:   p.Worktree.Answers.strategy,
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

// copyWarning is the warning a skipped or redirected env copy reports.
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

// record writes meta as the worktree's meta.json, replacing what was there.
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

// undo removes the worktree the add made, and the branch when the add created
// it, and returns failure; when either removal fails it returns the error that
// says what is left behind.
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

// leftBehind is an undo that failed: the add's failure, the removal's error,
// and whether the worktree (rather than only the branch) is left.
type leftBehind struct {
	Failure  *domain.Error
	Err      error
	Worktree bool
}

// leftBehind is the internal error of an undo that failed, with the follow-up
// that cleans what it left.
func (p *provisioning) leftBehind(left leftBehind) *domain.Error {
	branch, path := p.Worktree.Branch, p.Worktree.Path
	message, kept := "could not delete the branch, left behind: branch", "branch"
	followUp := &domain.FollowUp{Message: "branch " + branch + " is left behind", Params: domain.Params{"branch": branch}}
	if left.Worktree {
		message, kept = "could not remove the worktree, left behind: worktree", "worktree"
		followUp = &domain.FollowUp{Message: "branch " + branch + " is left half-created at " + path, Params: domain.Params{"branch": branch, "path": path}}
	}
	followUp.Argv, followUp.Code = p.Service.LeftBehind(branch), "provision.left_behind"

	return &domain.Error{
		Kind:     domain.KindInternal,
		Code:     "unit.undo_failed",
		Message:  message,
		Params:   domain.Params{"left": kept},
		FollowUp: followUp,
		Cause:    errors.Join(left.Failure, left.Err),
	}
}
