// Package logs runs the `wtm run logs` flow.
package logs

import (
	"errors"
	"fmt"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/flow"
	"github.com/LucasPcq/wtm/internal/flow/run/addressing"
	"github.com/LucasPcq/wtm/internal/flow/run/seam"
	"github.com/LucasPcq/wtm/internal/flow/run/target"
	"github.com/LucasPcq/wtm/internal/flow/runlogs"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

type Request struct {
	Worktrees []string
	Cwd       string
	// Precheck is what arrives ticked when the worktree step IS asked, as
	// worktree roots git spells them. A surface that already knows a likely
	// answer offers it; the selection stays exact.
	Precheck []string
	// Job narrows the output to one of them; empty takes every job the worktree
	// has. Unlike `run start`, there is a safe default here — all of them — so it
	// is never required.
	Job    string
	Config domain.RunConfig
}

type Outcome struct {
	// WorkDirs are the worktrees whose jobs the surface was shown, in selection
	// order.
	WorkDirs []string
	Aborted  bool
}

// ShowParams is what the surface reads from: the worktree's jobs, and which of
// them the reader asked about.
type ShowParams struct {
	Board runlogs.Board
	Job   string
	// Worktrees names what the board covers, in selection order: a surface reads
	// its length to know whether to say where a line came from at all.
	Worktrees []string
	// Warnings are what the board has to say about the addresses it lists; see
	// seam.SequenceParams.
	Warnings []string
}

type Presenter interface {
	flow.Presenter
	// Show reports the jobs' output. `run logs` starts nothing, so it hands over
	// a board rather than a start sequence.
	Show(ShowParams) error
}

type Params struct {
	Context   flow.Context
	Request   Request
	Prompter  flow.Prompter
	Presenter Presenter
}

func Run(params Params) (Outcome, error) {
	f := &logsFlow{
		ctx:       params.Context,
		request:   params.Request,
		prompter:  params.Prompter,
		presenter: params.Presenter,
	}
	return f.run()
}

type logsFlow struct {
	ctx       flow.Context
	request   Request
	prompter  flow.Prompter
	presenter Presenter

	named   []target.Resolved
	running map[string]int
}

func (f *logsFlow) run() (Outcome, error) {
	named, err := target.NamedAll(target.ResolveAllParams{ProjectDir: f.ctx.ProjectDir, Queries: f.request.Worktrees})
	if err != nil {
		return Outcome{}, err
	}
	f.named = named

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

	workDirs := target.WorkDirs(target.WorkDirsParams{Answers: answers, Named: f.named, Cwd: f.request.Cwd})
	addresses := addressing.Read(addressing.Params{Context: f.ctx, WorkDirs: workDirs})
	set := seam.OpenSet(seam.SetParams{
		ProjectDir:    f.ctx.ProjectDir,
		StateDir:      f.ctx.StateDir,
		WorkDirs:      workDirs,
		Jobs:          f.request.Config.Jobs,
		PortAddressed: addresses.PortAddressed,
		ProxyPort:     rules.ProxyPort(f.ctx.Config.Global),
		PublicPort:    process.PublicProxyPort(rules.ProxyPort(f.ctx.Config.Global)),
	})

	return Outcome{WorkDirs: workDirs}, f.presenter.Show(ShowParams{
		Board:     set.Board(),
		Job:       f.request.Job,
		Worktrees: set.Worktrees(),
		Warnings:  addresses.Warnings,
	})
}

// connect wakes the daemon: a job whose log is on disk is still read through it,
// and the worktree picker shows what each worktree is running.
func (f *logsFlow) connect() error {
	return f.presenter.Stage(flow.StageParams{
		Message: domain.RunDaemonConnecting,
		Work: func() error {
			if err := process.EnsureDaemon(process.DaemonParams{
				SocketPath: process.SocketPath(),
				ProxyPort:  rules.ProxyPort(f.ctx.Config.Global),
			}); err != nil {
				return fmt.Errorf("ensure daemon: %w", err)
			}
			f.running = target.RunningJobs(runlogs.NewService(runlogs.ServiceParams{SocketPath: process.SocketPath()}))
			return nil
		},
	})
}

// session asks for the worktree only. --job narrows what is shown, and showing
// every job is a safe default, so it is a preset rather than a question.
func (f *logsFlow) session() flow.Session {
	return flow.Session{
		ErrLabel: domain.CmdLogs,
		Presets:  target.Presets(target.PresetParams{Worktrees: target.Dirs(f.named), Job: f.request.Job}),
		Steps: []flow.Step{
			target.WorktreesStep(target.WorktreesParams{
				ProjectDir: f.ctx.ProjectDir,
				Current:    f.request.Cwd,
				Selected:   target.Preselected(target.PreselectedParams{Named: f.named, Precheck: f.request.Precheck}),
				Running:    f.running,
			}),
		},
	}
}

type ViewsParams struct {
	Board runlogs.Board
	// Job is the one asked for by name, empty for all of them.
	Job string
	// Persisted takes every job the worktree declares rather than only the ones a
	// stream can be opened on: reading log files back, a stopped job has as much
	// to show as a running one.
	Persisted bool
}

// Views is what one run of `run logs` reports on: the named job, or every job
// the worktree has anything to show for. It lives here rather than beside the
// rendering because choosing what to show is a decision, and output/ and tui/
// hold none.
func Views(params ViewsParams) ([]runlogs.JobView, error) {
	jobs := params.Board.Jobs()
	if params.Job != "" {
		// Every worktree's copy of the job, not the first: --job names a job, and
		// a board covering three worktrees holds three of them.
		named := make([]runlogs.JobView, 0, len(jobs))
		for _, view := range jobs {
			if view.Name == params.Job {
				named = append(named, view)
			}
		}
		if len(named) == 0 {
			return nil, fmt.Errorf("%w: %s", domain.ErrJobNotFound, params.Job)
		}
		return named, nil
	}

	views := make([]runlogs.JobView, 0, len(jobs))
	for _, view := range jobs {
		if params.Persisted || view.Attachable {
			views = append(views, view)
		}
	}
	return views, nil
}
