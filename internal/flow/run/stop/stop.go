// Package stop runs the `wtm run stop` flow.
package stop

import (
	"errors"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/service/process"
)

type Request struct {
	Worktrees []string
	Cwd       string
	Job       string
	// Config is run.toml: the job is resolved against it so a typo'd name fails
	// with a precise error instead of silently no-opping at the daemon.
	Config domain.RunConfig
}

type Outcome struct {
	// WorkDirs are the worktrees the job was stopped in, in selection order.
	WorkDirs []string
	Job      string
	// Results is one entry per worktree, each holding the single job this command
	// acts on. It is what lets the surfaces follow the arity (LUC-198).
	Results []domain.WorktreeJobResults
	// NoDaemon says nothing was listening. The job is stopped either way, which
	// is why it is an outcome and not an error.
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

func Run(params Params) (Outcome, error) {
	f := &stopFlow{
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
	}
	return f.run()
}

type stopFlow struct {
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter

	named []target.Resolved
}

func (f *stopFlow) run() (Outcome, error) {
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

	job, err := target.DeclaredJob(f.request.Config, answers.Value(target.KeyJob))
	if err != nil {
		return Outcome{}, err
	}
	outcome := Outcome{
		WorkDirs: target.WorkDirs(target.WorkDirsParams{Answers: answers, Named: f.named, Cwd: f.request.Cwd}),
		Job:      job.Name,
	}

	socket := process.SocketPath()
	if !process.IsDaemonRunning(socket) {
		outcome.NoDaemon = true
		return outcome, f.presenter.Stopped(outcome)
	}

	// The same job is stopped in each worktree in turn. A worktree that refuses
	// ends the run: unlike a start, there is nothing partial to leave standing —
	// the caller asked for the job to be down and it is not.
	for _, workDir := range outcome.WorkDirs {
		status, err := f.stop(socket, outcome.Job, workDir)
		if err != nil {
			return Outcome{}, err
		}
		outcome.Results = append(outcome.Results, domain.WorktreeJobResults{
			Worktree: f.branchOf(workDir),
			Path:     workDir,
			Jobs:     []domain.JobActionResult{{Name: outcome.Job, Status: status}},
		})
	}
	return outcome, f.presenter.Stopped(outcome)
}

// branchOf names a worktree the way a reader recognises it, falling back to the
// path git could not name.
func (f *stopFlow) branchOf(workDir string) string {
	for _, named := range f.named {
		if named.Dir == workDir && named.Branch != "" {
			return named.Branch
		}
	}
	return target.BranchOf(workDir)
}

func (f *stopFlow) stop(socket, job, workDir string) (string, error) {
	client := process.NewClient(socket)
	var resp process.Response
	if err := f.presenter.Stage(flow.StageParams{
		Message: fmt.Sprintf(domain.RunStoppingFmt, job),
		Work: func() error {
			var sendErr error
			resp, sendErr = client.Send(process.Request{
				Action:  process.ActionStop,
				Name:    job,
				WorkDir: workDir,
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
			target.WorktreesStep(target.WorktreesParams{
				ProjectDir: f.ctx.ProjectDir,
				Current:    f.request.Cwd,
				Selected:   target.Dirs(f.named),
				Running:    f.running(),
			}),
			target.JobStep(target.JobParams{Jobs: f.request.Config.Jobs, Flag: domain.FlagJob}),
		},
	}
}

func (f *stopFlow) running() map[string]int {
	socket := process.SocketPath()
	if !process.IsDaemonRunning(socket) {
		return nil
	}
	return target.RunningJobs(runlogs.NewService(runlogs.ServiceParams{SocketPath: socket}))
}
