// Package stop runs the `wtm run stop` flow.
package stop

import (
	"context"
	"errors"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

type Request struct {
	Worktrees []string
	Cwd       string
	Job       string
	// Config is run.toml: the job is resolved against it so a typo'd name fails
	// with a precise error instead of silently no-opping at the daemon.
	Config domain.RunConfig
	// ByName says run.toml could not be read: the job is stopped by the name
	// given, and the picker offers what the daemon runs, since stopping must
	// never depend on the file.
	ByName bool
}

type Outcome struct {
	// WorkDirs are the worktrees the job was stopped in, in selection order.
	WorkDirs []string
	Job      string
	// Results is one entry per worktree, each holding the single job this command
	// acts on.
	Results []domain.WorktreeJobResults
	// NoDaemon says nothing was listening and the index held nothing for these
	// worktrees: every result is not_running.
	NoDaemon bool
	Aborted  bool
}

type Presenter interface {
	flow.Presenter
	Stopped(Outcome) error
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

// Operation declares how a surface schedules a stop: it holds the worktree it
// is stopping a job in, and gives the surface back.
func Operation() flow.Operation {
	return flow.Operation{
		Kind:      domain.OpKindRunStop,
		Mode:      flow.ModeBackground,
		TargetKey: target.KeyWorktree,
	}
}

func Run(ctx context.Context, params Params) (Outcome, error) {
	f := &stopFlow{
		runCtx:    ctx,
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
	}
	return f.run()
}

type stopFlow struct {
	runCtx    context.Context
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter

	named []target.Resolved
}

func (f *stopFlow) run() (Outcome, error) {
	if !f.request.ByName {
		if err := target.RequireDeclared(target.DeclaredParams{Config: f.request.Config, Job: f.request.Job}); err != nil {
			return Outcome{}, err
		}
	}
	named, err := target.NamedAll(f.runCtx, target.ResolveAllParams{ProjectDir: f.ctx.ProjectDir, Queries: f.request.Worktrees})
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

	job, err := f.job(answers)
	if err != nil {
		return Outcome{}, err
	}
	outcome := Outcome{
		WorkDirs: target.WorkDirs(f.runCtx, target.WorkDirsParams{Answers: answers, Named: f.named, Cwd: f.request.Cwd}),
		Job:      job,
	}

	socket := process.SocketPath()
	if err := f.wake(outcome.WorkDirs); err != nil {
		return Outcome{}, err
	}
	if !process.IsDaemonRunning(socket) {
		outcome.NoDaemon = true
		outcome.Results = f.notRunning(outcome)
		return outcome, f.presenter.Stopped(outcome)
	}

	running, err := process.NewClient(socket).Send(process.Request{Action: process.ActionList})
	if err != nil {
		return Outcome{}, fmt.Errorf("stop %s: %w", outcome.Job, err)
	}
	// The same job is stopped in each worktree in turn. A worktree that refuses
	// ends the run: unlike a start, there is nothing partial to leave standing —
	// the caller asked for the job to be down and it is not.
	for _, workDir := range outcome.WorkDirs {
		status := domain.JobActionNotRunning
		if rules.JobUpIn(rules.JobUpInParams{Jobs: running.Jobs, Name: outcome.Job, WorkDir: workDir}) {
			status, err = f.stop(stopParams{Socket: socket, Job: outcome.Job, WorkDir: workDir})
			if err != nil {
				return Outcome{}, err
			}
		}
		outcome.Results = append(outcome.Results, domain.WorktreeJobResults{
			Branch: f.branchOf(workDir),
			Path:   workDir,
			Jobs:   []domain.JobActionResult{{Name: outcome.Job, Status: status}},
		})
	}
	return outcome, f.presenter.Stopped(outcome)
}

// wake starts a daemon when the index still holds jobs for these worktrees: a
// detached stack outlives the daemon that launched it, and `run stop` is one of
// the commands that must reach it.
func (f *stopFlow) wake(workDirs []string) error {
	if process.IsDaemonRunning(process.SocketPath()) {
		return nil
	}
	indexed := false
	for _, workDir := range workDirs {
		indexed = indexed || process.HasIndexedJobs(workDir)
	}
	if !indexed {
		return nil
	}
	return f.presenter.Stage(f.runCtx, flow.StageParams{
		Message: domain.RunDaemonConnecting,
		Work: func(ctx context.Context) error {
			return process.EnsureDaemon(process.DaemonParams{
				SocketPath: process.SocketPath(),
				ProxyPort:  rules.ProxyPort(f.ctx.Config.Global),
			})
		},
	})
}

func (f *stopFlow) notRunning(outcome Outcome) []domain.WorktreeJobResults {
	results := make([]domain.WorktreeJobResults, 0, len(outcome.WorkDirs))
	for _, workDir := range outcome.WorkDirs {
		results = append(results, domain.WorktreeJobResults{
			Branch: f.branchOf(workDir),
			Path:   workDir,
			Jobs:   []domain.JobActionResult{{Name: outcome.Job, Status: domain.JobActionNotRunning}},
		})
	}
	return results
}

func (f *stopFlow) job(answers flow.Answers) (string, error) {
	name := answers.Value(target.KeyJob)
	if f.request.ByName && name != "" {
		return name, nil
	}
	job, err := target.DeclaredJob(f.request.Config, name)
	return job.Name, err
}

func (f *stopFlow) branchOf(workDir string) string {
	return target.NamedBranch(f.runCtx, target.NamedBranchParams{Named: f.named, Dir: workDir})
}

type stopParams struct {
	Socket  string
	Job     string
	WorkDir string
}

func (f *stopFlow) stop(params stopParams) (string, error) {
	client := process.NewClient(params.Socket)
	job := params.Job
	var resp process.Response
	if err := f.presenter.Stage(f.runCtx, flow.StageParams{
		Message: fmt.Sprintf(domain.RunStoppingFmt, job),
		Work: func(ctx context.Context) error {
			var sendErr error
			resp, sendErr = client.Send(process.Request{
				Action:  process.ActionStop,
				Name:    job,
				WorkDir: params.WorkDir,
			})
			return sendErr
		},
	}); err != nil {
		return "", fmt.Errorf("stop %s: %w", job, err)
	}
	if resp.Status == process.StatusError {
		return "", fmt.Errorf("stop %s: %s", job, resp.Message)
	}
	if resp.Released {
		return domain.JobActionReleased, nil
	}
	return domain.JobActionStopped, nil
}

// session never wakes a daemon to decorate its picker: `run stop` is the one
// command that must work when nothing is listening, and starting a daemon in
// order to ask which job to stop would be its own kind of absurd.
func (f *stopFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.CmdStop,
		Presets:  target.Presets(target.PresetParams{Worktrees: target.Dirs(f.named), Job: f.request.Job}),
		Steps: []flow.Step{
			target.WorktreesStep(f.runCtx, target.WorktreesParams{
				ProjectDir: f.ctx.ProjectDir,
				Current:    f.request.Cwd,
				Selected:   target.Dirs(f.named),
				Running:    f.running(),
			}),
			target.JobStep(target.JobParams{Jobs: f.pickable(), Flag: domain.FlagJob}),
		},
	}
}

func (f *stopFlow) pickable() []domain.JobConfig {
	if !f.request.ByName {
		return f.request.Config.Jobs
	}
	socket := process.SocketPath()
	if !process.IsDaemonRunning(socket) {
		return nil
	}
	infos, _ := runlogs.NewService(runlogs.ServiceParams{SocketPath: socket}).List("")
	seen := map[string]bool{}
	var jobs []domain.JobConfig
	for _, info := range infos {
		if seen[info.Name] {
			continue
		}
		seen[info.Name] = true
		jobs = append(jobs, domain.JobConfig{Name: info.Name, Kind: info.Kind})
	}
	return jobs
}

func (f *stopFlow) running() map[string]int {
	socket := process.SocketPath()
	if !process.IsDaemonRunning(socket) {
		return nil
	}
	return target.RunningJobs(runlogs.NewService(runlogs.ServiceParams{SocketPath: socket}))
}
