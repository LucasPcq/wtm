// Package down runs the `wtm run down` flow.
package down

import (
	"errors"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
	"github.com/LucasPcq/wtm/internal/service/worktree"
)

type Request struct {
	Worktrees []string
	Cwd       string
	// Precheck is what arrives ticked when the worktree step IS asked, as
	// worktree roots git spells them. A surface that already knows a likely
	// answer offers it; the selection stays exact.
	Precheck []string
	// Profile narrows the stop to one profile's jobs. Empty means everything the
	// worktree has up, which is the command's safe default — so it is never asked.
	Profile string
	// All reaches across every worktree of this project — never another
	// repository — which is why it takes no worktree and no profile: it is a
	// different question, not a wider answer to this one.
	All    bool
	Config domain.RunConfig
}

type Outcome struct {
	// WorkDirs are the worktrees this run emptied, in selection order. Empty
	// with --all, whose worktrees are those Results names.
	WorkDirs []string
	Profile  string
	All      bool
	// Results is one entry per worktree, each holding the jobs it stopped.
	Results []domain.WorktreeJobResults
	// NoDaemon says nothing was listening, so nothing was running to stop.
	NoDaemon bool
	Aborted  bool
}

// Failed reports a job left standing. Every run command exits non-zero on what
// it could not do (LUC-198).
func (o Outcome) Failed() bool {
	for _, worktree := range o.Results {
		for _, result := range worktree.Jobs {
			if result.Status == domain.JobActionError {
				return true
			}
		}
	}
	return false
}

// Stopped is every job this run acted on, across the worktrees: a job it found
// not running is not one of them.
func (o Outcome) Stopped() []domain.JobActionResult {
	var jobs []domain.JobActionResult
	for _, worktree := range o.Results {
		for _, job := range worktree.Jobs {
			if job.Status != domain.JobActionNotRunning {
				jobs = append(jobs, job)
			}
		}
	}
	return jobs
}

type Presenter interface {
	flow.Presenter
	Downed(Outcome) error
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

// Operation declares how a surface schedules a down: it holds the worktree it
// is emptying, and gives the surface back.
func Operation() flow.Operation {
	return flow.Operation{
		Kind:      domain.OpKindRunDown,
		Mode:      flow.ModeBackground,
		TargetKey: target.KeyWorktree,
	}
}

func Run(params Params) (Outcome, error) {
	f := &downFlow{
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
	}
	return f.run()
}

type downFlow struct {
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter

	named []target.Resolved
}

func (f *downFlow) run() (Outcome, error) {
	if err := target.RequireDeclared(target.DeclaredParams{Config: f.request.Config, Profile: f.request.Profile}); err != nil {
		return Outcome{}, err
	}
	named, err := target.NamedAll(target.ResolveAllParams{ProjectDir: f.ctx.ProjectDir, Queries: f.request.Worktrees})
	if err != nil {
		return Outcome{}, err
	}
	f.named = named

	answers, err := f.prompter.Ask(f.session())
	if errors.Is(err, domain.ErrUserAborted) {
		f.presenter.Notice(flow.AbortedNotice)
		return Outcome{Aborted: true}, nil
	}
	if err != nil {
		return Outcome{}, err
	}

	outcome := Outcome{
		Profile: f.request.Profile,
		All:     f.request.All,
	}
	if !f.request.All {
		outcome.WorkDirs = target.WorkDirs(target.WorkDirsParams{Answers: answers, Named: f.named, Cwd: f.request.Cwd})
	}

	if err := f.wake(outcome.WorkDirs); err != nil {
		return Outcome{}, err
	}
	if !process.IsDaemonRunning(process.SocketPath()) {
		outcome.NoDaemon = true
		outcome.Results = f.nothingRunning(outcome)
		return outcome, f.presenter.Downed(outcome)
	}

	results, err := f.stop(outcome)
	if err != nil {
		return Outcome{}, err
	}
	outcome.Results = results
	return outcome, f.presenter.Downed(outcome)
}

// wake starts a daemon when the index still holds jobs for what is being
// stopped. A daemon exits once no foreground job is left, so after a reboot —
// or simply half an hour later — nothing is listening while detached stacks are
// very much up, and `run down` is exactly the command that must reach them.
func (f *downFlow) wake(workDirs []string) error {
	if process.IsDaemonRunning(process.SocketPath()) {
		return nil
	}
	indexed := false
	for _, workDir := range workDirs {
		indexed = indexed || process.HasIndexedJobs(workDir)
	}
	if f.request.All {
		indexed = process.HasAnyIndexedJob()
	}
	if !indexed {
		return nil
	}
	return f.presenter.Stage(flow.StageParams{
		Message: domain.RunDaemonConnecting,
		Work: func() error {
			return process.EnsureDaemon(process.DaemonParams{
				SocketPath: process.SocketPath(),
				ProxyPort:  rules.ProxyPort(f.ctx.Config.Global),
			})
		},
	})
}

// stop empties each worktree in turn. Sequentially, unlike `run up`: stopping
// is a round-trip to the daemon per job rather than a stack coming up, and the
// daemon is one server — the concurrency would buy nothing and interleave the
// stages a surface shows.
func (f *downFlow) stop(outcome Outcome) ([]domain.WorktreeJobResults, error) {
	if outcome.All {
		return f.stopEverywhere()
	}

	results := make([]domain.WorktreeJobResults, 0, len(outcome.WorkDirs))
	for _, workDir := range outcome.WorkDirs {
		jobs, err := f.stopIn(outcome, workDir)
		if err != nil {
			return nil, err
		}
		results = append(results, domain.WorktreeJobResults{
			Branch: f.branchOf(workDir),
			Path:   workDir,
			Jobs:   jobs,
		})
	}
	return results, nil
}

func (f *downFlow) stopIn(outcome Outcome, workDir string) ([]domain.JobActionResult, error) {
	if outcome.Profile != "" {
		return f.stopProfile(outcome, workDir)
	}
	return f.stopAll(workDir)
}

func (f *downFlow) branchOf(workDir string) string {
	return target.NamedBranch(target.NamedBranchParams{Named: f.named, Dir: workDir})
}

// stopProfile stops the profile's jobs one by one, so a job that refuses is
// named rather than lost inside a single failure for the whole set.
func (f *downFlow) stopProfile(outcome Outcome, workDir string) ([]domain.JobActionResult, error) {
	profile, ok := rules.FindProfile(f.request.Config, outcome.Profile)
	if !ok {
		return nil, fmt.Errorf(domain.RunProfileNotFoundFmt, domain.ErrProfileNotFound, outcome.Profile)
	}

	client := process.NewClient(process.SocketPath())
	running, err := client.Send(process.Request{Action: process.ActionList})
	if err != nil {
		return nil, fmt.Errorf("stop profile %s: %w", outcome.Profile, err)
	}
	jobs := rules.ProfileJobs(f.request.Config, profile)
	results := make([]domain.JobActionResult, 0, len(jobs))
	for _, job := range jobs {
		if !rules.JobUpIn(rules.JobUpInParams{Jobs: running.Jobs, Name: job.Name, WorkDir: workDir}) {
			results = append(results, domain.JobActionResult{Name: job.Name, Status: domain.JobActionNotRunning})
			continue
		}
		var resp process.Response
		err := f.presenter.Stage(flow.StageParams{
			Message: fmt.Sprintf(domain.RunStoppingFmt, job.Name),
			Work: func() error {
				var sendErr error
				resp, sendErr = client.Send(process.Request{
					Action:  process.ActionStop,
					Name:    job.Name,
					WorkDir: workDir,
				})
				return sendErr
			},
		})
		switch {
		case err != nil:
			results = append(results, domain.JobActionResult{Name: job.Name, Status: domain.JobActionError, Message: err.Error()})
		case resp.Status == process.StatusError:
			results = append(results, domain.JobActionResult{Name: job.Name, Status: domain.JobActionError, Message: resp.Message})
		default:
			results = append(results, domain.JobActionResult{Name: job.Name, Status: stoppedStatus(resp.Released)})
		}
	}
	return results, nil
}

// stopEverywhere empties every worktree of this project the daemon holds jobs
// in, one worktree at a time: the daemon is machine-wide, and --all never
// reaches into another repository.
func (f *downFlow) stopEverywhere() ([]domain.WorktreeJobResults, error) {
	worktrees, err := worktree.ListAll(worktree.ListAllParams{ProjectDir: f.ctx.ProjectDir})
	if err != nil {
		return nil, fmt.Errorf("stop all jobs: %w", err)
	}
	running, err := client().Send(process.Request{Action: process.ActionList})
	if err != nil {
		return nil, fmt.Errorf("stop all jobs: %w", err)
	}
	dirs := make([]string, 0, len(worktrees))
	for _, wt := range worktrees {
		dirs = append(dirs, wt.Path)
	}

	var results []domain.WorktreeJobResults
	for _, workDir := range rules.WorkDirsWithJobsUp(rules.WorkDirsWithJobsUpParams{Jobs: running.Jobs, Within: dirs}) {
		jobs, err := f.stopAll(workDir)
		if err != nil {
			return nil, err
		}
		results = append(results, domain.WorktreeJobResults{
			Branch: f.branchOf(workDir),
			Path:   workDir,
			Jobs:   jobs,
		})
	}
	return results, nil
}

func (f *downFlow) stopAll(workDir string) ([]domain.JobActionResult, error) {
	jobs, err := f.stoppedJobs(workDir)
	if err != nil {
		return nil, err
	}
	stopped := make([]domain.JobActionResult, 0, len(jobs))
	for _, job := range jobs {
		stopped = append(stopped, domain.JobActionResult{Name: job.Name, Status: stoppedStatus(job.Released)})
	}
	return stopped, nil
}

// stoppedJobs asks the daemon to empty one worktree, and answers with what it
// reported.
func (f *downFlow) stoppedJobs(workDir string) ([]domain.JobInfo, error) {
	request := process.Request{Action: process.ActionStopAll, WorkDir: workDir}

	var resp process.Response
	if err := f.presenter.Stage(flow.StageParams{
		Message: domain.RunStoppingJobs,
		Work: func() error {
			var sendErr error
			resp, sendErr = client().Send(request)
			return sendErr
		},
	}); err != nil {
		return nil, fmt.Errorf("stop all jobs: %w", err)
	}
	if resp.Status == process.StatusError {
		return nil, fmt.Errorf("stop all: %s", resp.Message)
	}
	return resp.Jobs, nil
}

// nothingRunning is what a down reports with no daemon to ask: every worktree
// it targeted, holding nothing — or, under --profile, each of its jobs
// not_running.
func (f *downFlow) nothingRunning(outcome Outcome) []domain.WorktreeJobResults {
	var jobs []domain.JobActionResult
	if profile, ok := rules.FindProfile(f.request.Config, outcome.Profile); ok {
		for _, job := range rules.ProfileJobs(f.request.Config, profile) {
			jobs = append(jobs, domain.JobActionResult{Name: job.Name, Status: domain.JobActionNotRunning})
		}
	}
	results := make([]domain.WorktreeJobResults, 0, len(outcome.WorkDirs))
	for _, workDir := range outcome.WorkDirs {
		results = append(results, domain.WorktreeJobResults{
			Branch: f.branchOf(workDir),
			Path:   workDir,
			Jobs:   append([]domain.JobActionResult{}, jobs...),
		})
	}
	return results
}

func client() *process.Client { return process.NewClient(process.SocketPath()) }

// session asks for the worktree and nothing else. `run down` with no --profile
// means "stop everything here", which is a safe default: asking would force a
// choice the command does not need and offers no "all" answer for. With --all
// there is no worktree to ask about either.
func (f *downFlow) session() flow.Session {
	if f.request.All {
		return flow.Session{ErrLabel: domain.CmdDown}
	}
	return flow.Session{
		ErrLabel: domain.CmdDown,
		Presets:  target.Presets(target.PresetParams{Worktrees: target.Dirs(f.named), Profile: f.request.Profile}),
		Steps: []flow.Step{
			target.WorktreesStep(target.WorktreesParams{
				ProjectDir: f.ctx.ProjectDir,
				Current:    f.request.Cwd,
				Selected:   target.Preselected(target.PreselectedParams{Named: f.named, Precheck: f.request.Precheck}),
				Running:    f.running(),
			}),
		},
	}
}

func (f *downFlow) running() map[string]int {
	socket := process.SocketPath()
	if !process.IsDaemonRunning(socket) {
		return nil
	}
	return target.RunningJobs(runlogs.NewService(runlogs.ServiceParams{SocketPath: socket}))
}

// stoppedStatus tells a shared job this worktree let go of apart from one that
// went down: another worktree may still hold it.
func stoppedStatus(released bool) string {
	if released {
		return domain.JobActionReleased
	}
	return domain.JobActionStopped
}
