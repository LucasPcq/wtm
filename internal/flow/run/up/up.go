// Package up runs the `wtm run up` flow.
package up

import (
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/addressing"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/runconfig"
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
	// Profiles are the profiles to start, in the order they were named. A job
	// several of them list starts once.
	Profiles []string
	// Exclusive and Parallel override the project's standing preference for one
	// run. They are the concurrency step's Resolve, not a second axis.
	Exclusive bool
	Parallel  bool
	NoProbe   bool
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

func Run(params Params) (Outcome, error) {
	f := &upFlow{
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
	}
	return f.run()
}

type upFlow struct {
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter

	// named are the worktrees the positionals designated, nil when there were none.
	named []target.Resolved
	// jobs and running are one reading of the daemon's index: what runs where,
	// for the worktree badges and for the concurrency question.
	jobs    []domain.JobInfo
	running map[string]int
	service runlogs.Service
}

func (f *upFlow) run() (Outcome, error) {
	named, err := target.NamedAll(target.ResolveAllParams{ProjectDir: f.ctx.ProjectDir, Queries: f.request.Worktrees})
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

	answers, err := f.prompter.Ask(f.session())
	if errors.Is(err, domain.ErrUserAborted) {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}
	if err != nil {
		return Outcome{}, err
	}

	if err := f.remember(answers); err != nil {
		return Outcome{}, err
	}
	f.noticeOverridden(answers)
	if err := f.clearOthers(answers); err != nil {
		return Outcome{}, err
	}

	return f.start(answers)
}

// connect wakes the daemon and reads its index once. Both the worktree badges
// and the concurrency question are about what is already running, so neither
// can be built before this.
func (f *upFlow) connect() error {
	return f.presenter.Stage(flow.StageParams{
		Message: domain.RunDaemonConnecting,
		Work: func() error {
			if err := process.EnsureDaemon(process.DaemonParams{
				SocketPath: process.SocketPath(),
				ProxyPort:  rules.ProxyPort(f.ctx.Config.Global),
			}); err != nil {
				return fmt.Errorf("ensure daemon: %w", err)
			}
			f.service = runlogs.NewService(runlogs.ServiceParams{SocketPath: process.SocketPath()})
			// A daemon that cannot list is not a reason to refuse the run: the
			// counts decorate a picker and the question defaults to stopping
			// nothing.
			f.jobs, _ = f.service.List("")
			f.running = rules.RunningJobsByWorktree(f.jobs)
			return nil
		},
	})
}

// remember writes the answer to run.toml when the user asked for it to stand.
// It is never silent: a file changed without a word is a file nobody knows to
// change back.
func (f *upFlow) remember(answers flow.Answers) error {
	answer := answers.Value(KeyConcurrency)
	if !remembers(answer) {
		return nil
	}

	cfg := f.request.Config
	cfg.Concurrency = concurrencyOf(answer)
	if err := runconfig.Save(runconfig.SaveParams{StateDir: f.ctx.StateDir, Config: cfg}); err != nil {
		return fmt.Errorf("remember concurrency: %w", err)
	}
	f.request.Config = cfg
	f.presenter.Status(flow.Notice{
		Kind: flow.NoticeMessage,
		Text: fmt.Sprintf(domain.RunConcurrencyRememberedFmt, cfg.Concurrency),
	})
	return nil
}

// noticeOverridden says the project's settled answer could not be applied to
// this run. It is only ever reached where nobody could be asked: the safe
// default destroys nothing, and a default that goes unsaid is a default nobody
// can correct.
func (f *upFlow) noticeOverridden(answers flow.Answers) {
	if answers.Answered(KeyConcurrency) || !f.decideConcurrency(answers).Contradiction {
		return
	}
	f.presenter.Status(warning(fmt.Sprintf(domain.RunConcurrencyOverriddenFmt,
		f.request.Config.Concurrency, len(f.workDirs(answers)))))
}

// clearOthers stops the other worktrees' jobs when that is what was decided.
// A worktree that refuses to stop is reported and the run carries on: the
// answer was about this machine's load, not about a dependency.
func (f *upFlow) clearOthers(answers flow.Answers) error {
	if f.concurrency(answers) != domain.ConcurrencyExclusive {
		return nil
	}
	others := f.otherWorktrees(answers)
	if len(others) == 0 {
		return nil
	}

	dirs := make([]string, 0, len(others))
	for dir := range others {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	client := process.NewClient(process.SocketPath())
	return f.presenter.Stage(flow.StageParams{
		Message: domain.RunStoppingOthers,
		Work: func() error {
			for _, dir := range dirs {
				resp, err := client.Send(process.Request{Action: process.ActionStopAll, WorkDir: dir})
				switch {
				case err != nil:
					f.presenter.Status(warning(fmt.Sprintf(domain.RunStopOtherFailFmt, filepath.Base(dir), err)))
				case resp.Status == process.StatusError:
					f.presenter.Status(warning(fmt.Sprintf(domain.RunStopOtherFailFmt, filepath.Base(dir), resp.Message)))
				default:
					f.presenter.Status(flow.Notice{
						Kind: flow.NoticeSuccess,
						Text: fmt.Sprintf(domain.RunStoppedOtherFmt, filepath.Base(dir)),
					})
				}
			}
			return nil
		},
	})
}

func warning(text string) flow.Notice {
	return flow.Notice{Kind: flow.NoticeWarning, Text: text}
}

func (f *upFlow) start(answers flow.Answers) (Outcome, error) {
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

	addresses := addressing.Read(addressing.Params{Context: f.ctx, WorkDirs: workDirs})
	set := seam.OpenSet(seam.SetParams{
		ProjectDir:    f.ctx.ProjectDir,
		StateDir:      f.ctx.StateDir,
		WorkDirs:      workDirs,
		Jobs:          profile.Jobs,
		Declared:      f.request.Config.Jobs,
		PortAddressed: addresses.PortAddressed,
		ProbeBudget:   rules.PortProbeBudget(f.request.Config),
		NoProbe:       f.request.NoProbe,
		ProxyPort:     rules.ProxyPort(f.ctx.Config.Global),
		PublicPort:    process.PublicProxyPort(rules.ProxyPort(f.ctx.Config.Global)),
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
		Warnings:  addresses.Warnings,
		Start:     set.Starter(seam.StartParams{Profile: profile.Name, Jobs: rules.JobsWithEffectivePorts(f.request.Config, profile.Jobs)}),
	})
	if err != nil {
		return Outcome{}, err
	}

	if err := f.offerToSilenceProbes(results); err != nil {
		return Outcome{}, err
	}

	return Outcome{
		WorkDirs: workDirs,
		Profile:  profile.Name,
		Results:  results,
		Aborted:  results.Aborted(),
	}, nil
}

// offerToSilenceProbes asks once about the warnings that will otherwise come
// back identical at every run: a job binding the base port because its command
// never reads the variable. A warning about a port another worktree holds is
// not offered — there is nothing to acknowledge, the run said whose it is.
func (f *upFlow) offerToSilenceProbes(results runlogs.Outcomes) error {
	// Never after an abort: the reader stopped the run or a job failed, and the
	// question to answer then is why — not whether to hear less about it.
	if !f.prompter.Interactive() || results.Aborted() {
		return nil
	}

	var probes []domain.PortProbe
	for _, outcome := range results {
		probes = append(probes, outcome.Probes...)
	}
	names := rules.JobsToSilence(rules.JobsToSilenceParams{Probes: probes, Jobs: f.request.Config.Jobs})
	if len(names) == 0 {
		return nil
	}

	proceed, err := f.prompter.Confirm(flow.ConfirmParams{
		Title:       domain.ProbeSilenceTitle,
		Description: fmt.Sprintf(domain.ProbeSilenceDescFmt, strings.Join(names, domain.CmdListVarSep)),
		DefaultYes:  false,
	})
	if err != nil || !proceed {
		return nil
	}

	cfg := rules.SilenceProbes(rules.SilenceProbesParams{Config: f.request.Config, Jobs: names})
	if err := runconfig.Save(runconfig.SaveParams{StateDir: f.ctx.StateDir, Config: cfg}); err != nil {
		return fmt.Errorf("silence port probes: %w", err)
	}
	f.request.Config = cfg
	f.presenter.Status(flow.Notice{
		Kind: flow.NoticeMessage,
		Text: fmt.Sprintf(domain.ProbeSilencedFmt, strings.Join(names, domain.CmdListVarSep)),
	})
	return nil
}

// resolvedProfile is what this run settled on: a name for it and the jobs it
// starts. The name is empty for a config declaring no profile at all — dropping
// it left `run up` unable to say which of several it had brought up (LUC-208).
// Over several profiles it names them all, which is what the recap reads back.
type resolvedProfile struct {
	Name string
	Jobs []domain.JobConfig
}

func (f *upFlow) resolveProfile(answers flow.Answers) (resolvedProfile, error) {
	names := answers.Values(target.KeyProfile)
	if len(names) == 0 {
		profile, ok := rules.DefaultProfile(f.request.Config)
		if !ok {
			return resolvedProfile{Jobs: rules.JobsWithoutProfile(f.request.Config)}, nil
		}
		return f.profileRun(profile), nil
	}

	profiles := make([]domain.ProfileConfig, 0, len(names))
	for _, name := range names {
		profile, ok := rules.FindProfile(f.request.Config, name)
		if !ok {
			return resolvedProfile{}, fmt.Errorf("profile %q not found in config", name)
		}
		profiles = append(profiles, profile)
	}
	return f.profilesRun(profiles), nil
}

// profilesRun is the union of what the chosen profiles name, in the order they
// were chosen. A job two of them list is started once: the second mention is
// the same process, and starting it twice would be the collision the whole
// module exists to prevent.
func (f *upFlow) profilesRun(profiles []domain.ProfileConfig) resolvedProfile {
	if len(profiles) == 1 {
		return f.profileRun(profiles[0])
	}

	names := make([]string, 0, len(profiles))
	seen := map[string]bool{}
	var jobs []domain.JobConfig
	for _, profile := range profiles {
		names = append(names, profile.Name)
		for _, job := range rules.ProfileJobs(f.request.Config, profile) {
			if seen[job.Name] {
				continue
			}
			seen[job.Name] = true
			jobs = append(jobs, job)
		}
	}
	return resolvedProfile{Name: strings.Join(names, domain.CmdListVarSep), Jobs: jobs}
}

func (f *upFlow) profileRun(profile domain.ProfileConfig) resolvedProfile {
	return resolvedProfile{Name: profile.Name, Jobs: rules.ProfileJobs(f.request.Config, profile)}
}
