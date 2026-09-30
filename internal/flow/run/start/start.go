// Package start runs the `wtm run start` flow.
package start

import (
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
	// Worktree is the positional as it was typed; Cwd is what answers for it
	// when nobody is asked.
	Worktree string
	Cwd      string
	Job      string
	// Exclusive and Parallel override the project's standing preference for one
	// run, as they do for `run up`.
	Exclusive bool
	Parallel  bool
	NoProbe   bool
	// Force starts a job that changes data the worktree does not own without
	// asking.
	Force bool
	// Config is run.toml, so a job name matching nothing fails here rather than
	// at the daemon.
	Config domain.RunConfig
}

type Outcome struct {
	WorkDir string
	Job     domain.JobConfig
	Result  runlogs.Outcome
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

// Operation declares how a surface schedules a single start: like `run up` it
// gives the surface back and holds the worktree it started the job in.
func Operation() flow.Operation {
	return flow.Operation{
		Kind:      domain.OpKindRunStart,
		Mode:      flow.ModeBackground,
		TargetKey: target.KeyWorktree,
	}
}

func Run(params Params) (Outcome, error) {
	f := &startFlow{
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
	}
	return f.run()
}

type startFlow struct {
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter

	named *target.Resolved
	// jobs is one reading of the daemon's index; running is its per-worktree
	// tally, which the picker shows.
	jobs        []domain.JobInfo
	running     map[string]int
	concurrency *concurrency.Question
}

func (f *startFlow) run() (Outcome, error) {
	if err := target.RequireDeclared(target.DeclaredParams{Config: f.request.Config, Job: f.request.Job}); err != nil {
		return Outcome{}, err
	}
	named, err := target.Named(target.ResolveParams{ProjectDir: f.ctx.ProjectDir, Query: f.request.Worktree})
	if err != nil {
		return Outcome{}, err
	}
	f.named = named

	if err := f.connect(); err != nil {
		return Outcome{}, err
	}
	f.concurrency = f.question()

	answers, err := f.prompter.Ask(f.session())
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

	job, err := target.DeclaredJob(f.request.Config, answers.Value(target.KeyJob))
	if err != nil {
		return Outcome{}, err
	}

	workDir := f.workDirs(answers)[0]
	if err := seam.RequireEnv(seam.RequireEnvParams{ProjectDir: f.ctx.ProjectDir, StateDir: f.ctx.StateDir, WorkDirs: []string{workDir}}); err != nil {
		return Outcome{}, err
	}
	// Refused rather than started: a runner and one of its own children are the
	// same process twice on the same port, whether the other one was asked for
	// in this gesture or is already up.
	if conflicts := rules.StartConflicts(rules.StartConflictsParams{
		Config:   f.request.Config,
		Starting: append([]string{job.Name}, rules.JobsUpIn(f.jobs, []string{workDir})...),
	}); len(conflicts) > 0 {
		return Outcome{}, fmt.Errorf("%s:\n%s", domain.JobConflictTitle, strings.Join(rules.JobConflictLines(conflicts), "\n"))
	}

	proceed, err := foreigndata.Allow(foreigndata.Params{
		Context:  f.ctx,
		Config:   f.request.Config,
		Jobs:     []domain.JobConfig{job},
		WorkDirs: []string{workDir},
		Force:    f.request.Force,
		Prompter: f.prompter,
	})
	if err != nil {
		return Outcome{}, err
	}
	if !proceed {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}

	cfg, err := f.concurrency.Apply(answers)
	f.request.Config = cfg
	if err != nil {
		return Outcome{}, err
	}

	warnings := addressing.Lines(addressing.Params{Context: f.ctx, WorkDirs: []string{workDir}})
	runSeam := seam.Open(f.seamParams(workDir))

	result, err := f.presenter.Sequence(seam.SequenceParams{
		Board:    runSeam.Board(),
		Job:      job.Name,
		Inline:   job.Kind == domain.JobKindTask,
		Warnings: warnings,
		Start:    runSeam.Starter(seam.StartParams{Jobs: rules.JobsWithEffectivePorts(f.request.Config, []domain.JobConfig{job})}),
	})
	if err != nil {
		return Outcome{}, err
	}
	cfg, err = probes.OfferToSilence(probes.Params{
		Context:   f.ctx,
		Prompter:  f.prompter,
		Presenter: f.presenter,
		Config:    f.request.Config,
		Results:   result,
	})
	f.request.Config = cfg
	if err != nil {
		return Outcome{}, err
	}
	if rules.IsShared(job) {
		owed.Settle(owed.Params{Context: f.ctx, Presenter: f.presenter})
	}
	return Outcome{WorkDir: workDir, Job: job, Result: result.One(), Aborted: result.Aborted()}, nil
}

// connect wakes the daemon before anything is asked: the worktree picker shows
// what each worktree is already running, which only the daemon knows.
func (f *startFlow) connect() error {
	return f.presenter.Stage(flow.StageParams{
		Message: domain.RunDaemonConnecting,
		Work: func() error {
			if err := process.EnsureCurrentDaemon(process.DaemonParams{
				SocketPath: process.SocketPath(),
				ProxyPort:  rules.ProxyPort(f.ctx.Config.Global),
			}); err != nil {
				return fmt.Errorf("ensure daemon: %w", err)
			}
			// A daemon that cannot list is not a reason to refuse the run: the
			// counts decorate a picker, and the guard below only ever adds to
			// what this gesture already names.
			f.jobs, _ = runlogs.NewService(runlogs.ServiceParams{SocketPath: process.SocketPath()}).List("")
			f.running = rules.RunningJobsByWorktree(f.jobs)
			return nil
		},
	})
}

// seamParams lists every declared job on the board, not just this one: starting
// a job is no reason to hide the ones already up beside it.
func (f *startFlow) seamParams(workDir string) seam.Params {
	proxy := seam.ProxyPortsFor(seam.ProxyPortsParams{Global: f.ctx.Config.Global, Run: f.request.Config})
	return seam.Params{
		ProjectDir:  f.ctx.ProjectDir,
		StateDir:    f.ctx.StateDir,
		WorkDir:     workDir,
		Jobs:        f.request.Config.Jobs,
		ProxyPort:   proxy.Bind,
		PublicPort:  proxy.Public,
		ProbeBudget: rules.PortProbeBudget(f.request.Config),
		NoProbe:     f.request.NoProbe,
	}
}

func (f *startFlow) question() *concurrency.Question {
	return concurrency.New(concurrency.Params{
		Context:   f.ctx,
		Presenter: f.presenter,
		Exclusive: f.request.Exclusive,
		Parallel:  f.request.Parallel,
		Config:    f.request.Config,
		Running:   f.jobs,
		WorkDirs:  f.workDirs,
		Starting:  f.startingJobs,
	})
}

func (f *startFlow) workDirs(answers flow.Answers) []string {
	return []string{target.WorkDir(target.WorkDirParams{Answers: answers, Named: f.named, Cwd: f.request.Cwd})}
}

// startingJobs is empty while the job is unknown: the run refuses it later.
func (f *startFlow) startingJobs(answers flow.Answers) []domain.JobConfig {
	job, err := target.DeclaredJob(f.request.Config, answers.Value(target.KeyJob))
	if err != nil {
		return nil
	}
	return rules.JobsWithEffectivePorts(f.request.Config, []domain.JobConfig{job})
}

func (f *startFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.CmdStart,
		Presets:  target.Presets(target.PresetParams{Named: f.named, Job: f.request.Job}),
		Steps: []flow.Step{
			target.WorktreeStep(target.WorktreeParams{
				ProjectDir: f.ctx.ProjectDir,
				Current:    f.request.Cwd,
				Running:    f.running,
			}),
			target.JobStep(target.JobParams{Jobs: f.request.Config.Jobs, Flag: domain.FlagJob}),
			f.concurrency.Step(),
		},
	}
}
