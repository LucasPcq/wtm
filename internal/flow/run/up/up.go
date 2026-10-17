// Package up runs the `wtm run up` flow.
package up

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/addressing"
	"github.com/LucasPcq/wtm/internal/flow/run/concurrency"
	"github.com/LucasPcq/wtm/internal/flow/run/foreigndata"
	"github.com/LucasPcq/wtm/internal/flow/run/owed"
	"github.com/LucasPcq/wtm/internal/flow/run/probes"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

type Request struct {
	// Worktrees are the positionals as they were typed; empty leaves the step to
	// ask, or to answer with the current worktree.
	Worktrees []string
	// Cwd is where the command was launched, the worktree step's safe default.
	Cwd string
	// Precheck is what arrives ticked when the worktree step IS asked, as
	// worktree roots git spells them. A surface that already knows a likely
	// answer offers it; the selection stays exact.
	Precheck []string
	// Profile is the profile to start; empty leaves the step to ask, or to take
	// the default one.
	Profile string
	// Exclusive and Parallel override the project's standing preference for one
	// run. They are the concurrency step's Resolve, not a second axis.
	Exclusive bool
	Parallel  bool
	NoProbe   bool
	// Force starts jobs that change data the worktree does not own without
	// asking: the safety axis, never implied by --yes.
	Force bool
	// Config is run.toml, already validated by the surface that read it.
	Config domain.RunConfig
}

type Outcome struct {
	// WorkDirs are the worktrees this run acted on, in selection order.
	WorkDirs []string
	Profile  string
	// Results is one account per worktree. A run over one is a slice of one:
	// the surfaces read the arity rather than branching on a mode (LUC-198).
	Results runlogs.Outcomes
	Aborted bool
}

type Presenter interface {
	flow.Presenter
	seam.Watcher
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

// Operation declares how a surface must schedule a run up: it gives the surface
// back and holds the worktree it started jobs in, named by the worktree step.
func Operation() flow.Operation {
	return flow.Operation{
		Kind:      domain.OpKindRunUp,
		Mode:      flow.ModeBackground,
		TargetKey: target.KeyWorktree,
	}
}

func Run(ctx context.Context, params Params) (Outcome, error) {
	f := &upFlow{
		runCtx:    ctx,
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
	}
	return f.run(ctx)
}

type upFlow struct {
	runCtx    context.Context
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter

	// named are the worktrees the positionals designated, nil when there were none.
	named []target.Resolved
	// jobs and running are one reading of the daemon's index: what runs where,
	// for the worktree badges and for the concurrency question.
	jobs        []domain.JobInfo
	running     map[string]int
	service     runlogs.Service
	concurrency *concurrency.Question
}

func (f *upFlow) run(ctx context.Context) (Outcome, error) {
	if err := target.RequireDeclared(target.DeclaredParams{Config: f.request.Config, Profile: f.request.Profile}); err != nil {
		return Outcome{}, err
	}
	named, err := target.NamedAll(f.runCtx, target.ResolveAllParams{ProjectDir: f.ctx.ProjectDir, Queries: f.request.Worktrees})
	if err != nil {
		return Outcome{}, err
	}
	f.named = named

	// A flag that contradicts itself is not a decision to default: --exclusive
	// means one stack at a time, and it cannot be applied to a run that brings up
	// several. Refused here rather than after the wizard so a run that cannot
	// happen does not wake a daemon first; the picker refuses the same thing at
	// the tick, through the step's ValidateSet.
	if f.request.Exclusive && len(named) > 1 {
		return Outcome{}, domain.ErrExclusiveMultiWorktree
	}

	if err := f.connect(); err != nil {
		return Outcome{}, err
	}
	f.concurrency = f.question()

	answers, err := f.prompter.Ask(f.session(f.runCtx))
	if errors.Is(err, domain.ErrUserAborted) {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}
	if err != nil {
		return Outcome{}, err
	}
	if f.concurrency.Cancelled(answers) {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}
	if err := seam.RequireEnv(f.runCtx, seam.RequireEnvParams{ProjectDir: f.ctx.ProjectDir, StateDir: f.ctx.StateDir, WorkDirs: f.workDirs(answers), Publisher: f.ctx.Publisher}); err != nil {
		return Outcome{}, err
	}
	// Before anything is stopped: a selection that is its own conflict must not
	// cost the other worktrees their jobs first.
	if clashes := rules.SelfPortClashes(f.concurrency.StartingClaims(f.runCtx, answers)); len(clashes) > 0 {
		return Outcome{}, fmt.Errorf(domain.RunSelfPortClashFmt, strings.Join(rules.PortClashLines(clashes), "\n"))
	}
	if proceed, err := f.allowForeignData(answers); err != nil || !proceed {
		if err == nil {
			f.presenter.Notice(flow.AbortedNotice)
		}
		return Outcome{Aborted: err == nil}, err
	}

	cfg, err := f.concurrency.Apply(f.runCtx, answers)
	f.request.Config = cfg
	if err != nil {
		return Outcome{}, err
	}

	return f.start(ctx, answers)
}

// allowForeignData stops before a job rewrites data the worktree does not own —
// asked here, before any other worktree is stopped for this run's sake.
func (f *upFlow) allowForeignData(answers flow.Answers) (bool, error) {
	profile, err := f.resolveProfile(answers)
	if err != nil {
		return false, err
	}
	return foreigndata.Allow(f.runCtx, foreigndata.Params{
		Context:  f.ctx,
		Config:   f.request.Config,
		Jobs:     profile.Jobs,
		WorkDirs: f.workDirs(answers),
		Force:    f.request.Force,
		Prompter: f.prompter,
	})
}

// connect wakes the daemon and reads its index once. Both the worktree badges
// and the concurrency question are about what is already running, so neither
// can be built before this.
func (f *upFlow) connect() error {
	return f.presenter.Stage(f.runCtx, flow.StageParams{
		Message: domain.RunDaemonConnecting,
		Work: func(ctx context.Context) error {
			if err := process.EnsureCurrentDaemon(ctx, process.DaemonParams{
				SocketPath: process.SocketPath(),
				ProxyPort:  rules.ProxyPort(f.ctx.Config.Global),
			}); err != nil {
				return fmt.Errorf("ensure daemon: %w", err)
			}
			f.service = runlogs.NewService(f.runCtx, runlogs.ServiceParams{SocketPath: process.SocketPath()})
			// A daemon that cannot list is not a reason to refuse the run: the
			// counts decorate a picker and the question defaults to stopping
			// nothing.
			f.jobs, _ = f.service.List("")
			f.running = rules.RunningJobsByWorktree(f.jobs)
			return nil
		},
	})
}

func (f *upFlow) start(ctx context.Context, answers flow.Answers) (Outcome, error) {
	workDirs := f.workDirs(answers)
	profile, err := f.resolveProfile(answers)
	if err != nil {
		return Outcome{}, err
	}

	// Refused rather than started: a runner and one of its own children are the
	// same process twice on the same port, and the second one to bind fails in a
	// way that names neither.
	if conflicts := rules.StartConflicts(rules.StartConflictsParams{
		Config:   f.request.Config,
		Starting: append(rules.JobNames(profile.Jobs), rules.JobsUpIn(f.jobs, workDirs)...),
	}); len(conflicts) > 0 {
		return Outcome{}, fmt.Errorf("%s:\n%s", domain.JobConflictTitle, strings.Join(rules.JobConflictLines(conflicts), "\n"))
	}

	warnings := addressing.Lines(f.runCtx, addressing.Params{Context: f.ctx, WorkDirs: workDirs})
	proxy := seam.ProxyPortsFor(ctx, seam.ProxyPortsParams{Global: f.ctx.Config.Global, Run: f.request.Config})
	set := seam.OpenSet(f.runCtx, seam.SetParams{
		ProjectDir:  f.ctx.ProjectDir,
		StateDir:    f.ctx.StateDir,
		WorkDirs:    workDirs,
		Jobs:        profile.Jobs,
		Declared:    f.request.Config.Jobs,
		ProbeBudget: rules.PortProbeBudget(f.request.Config),
		NoProbe:     f.request.NoProbe,
		ProxyPort:   proxy.Bind,
		PublicPort:  proxy.Public,
		Publisher:   f.ctx.Publisher,
	})

	// Before anything starts: a run defines what its worktrees' log directories
	// hold, the way LUC-198 made each file hold one run. Otherwise the directory
	// only ever grew, and every surface reading it showed what the worktree ran
	// last fortnight beside what it is running now.
	set.PruneLogs(seam.PruneParams{Jobs: profile.Jobs, Running: f.jobs})

	results, err := f.presenter.Sequence(seam.SequenceParams{
		Board:     set.Board(),
		Profile:   profile.Name,
		Worktrees: set.Worktrees(),
		Warnings:  warnings,
		Start:     set.Starter(seam.StartParams{Profile: profile.Name, Jobs: rules.JobsWithEffectivePorts(f.request.Config, profile.Jobs)}),
	})
	if err != nil {
		return Outcome{}, err
	}

	cfg, err := probes.OfferToSilence(ctx, probes.Params{
		Context:   f.ctx,
		Prompter:  f.prompter,
		Presenter: f.presenter,
		Config:    f.request.Config,
		Results:   results,
	})
	f.request.Config = cfg
	if err != nil {
		return Outcome{}, err
	}
	// A shared service this run brought up is the moment to pay what a clean
	// owed it while it was down.
	owed.Settle(f.runCtx, owed.Params{Context: f.ctx, Presenter: f.presenter})

	return Outcome{
		WorkDirs: workDirs,
		Profile:  profile.Name,
		Results:  results,
		Aborted:  results.Aborted(),
	}, nil
}

// resolvedProfile is what this run settled on: a name for it and the jobs it
// starts. The name is empty for a config declaring no profile at all — dropping
// it left `run up` unable to say which of several it had brought up (LUC-208).
type resolvedProfile struct {
	Name string
	Jobs []domain.JobConfig
}

func (f *upFlow) resolveProfile(answers flow.Answers) (resolvedProfile, error) {
	name := answers.Value(target.KeyProfile)
	if name == "" {
		if len(f.request.Config.Profiles) == 0 {
			return resolvedProfile{Jobs: rules.JobsWithoutProfile(f.request.Config)}, nil
		}
		profile, ok := rules.DefaultProfile(f.request.Config)
		if !ok {
			return resolvedProfile{}, domain.ErrProfileRequired
		}
		return f.profileRun(profile), nil
	}

	profile, ok := rules.FindProfile(f.request.Config, name)
	if !ok {
		return resolvedProfile{}, fmt.Errorf(domain.RunProfileNotFoundFmt, domain.ErrProfileNotFound, name)
	}
	return f.profileRun(profile), nil
}

func (f *upFlow) profileRun(profile domain.ProfileConfig) resolvedProfile {
	return resolvedProfile{Name: profile.Name, Jobs: rules.ProfileJobs(f.request.Config, profile)}
}
