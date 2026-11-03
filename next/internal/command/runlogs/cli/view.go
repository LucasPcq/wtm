package cli

import (
	"context"
	"errors"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/command/runlogs"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/LucasPcq/wtm/next/internal/ui/runview"
)

// View is where run logs shows the jobs it read: the run daemon its live view asks, and how it opens an address in the browser (nil opens none). A zero View shows none.
type View struct {
	Daemon runview.Daemon
	Open   func(url string) error
}

// show opens the live view on the jobs read, when a job ran in one of the worktrees; leaving it stops nothing, and the read is not drawn again under it.
func (c *logsCommand) show(ctx context.Context, read runlogs.Outcome, req runlogs.Request, _ *uicli.Run) (runlogs.Outcome, error) {
	params, ran := c.viewOf(read, req)
	if c.view.Daemon == nil || !ran {
		return read, nil
	}
	c.viewed = true
	if err := runview.Run(ctx, params); err != nil {
		output.WarningLine(c.io.Streams.Err, err.Error())
	}
	return read, nil
}

// viewOf is the live view of read: each worktree with the jobs read there, the main checkout the shared services run in, the job asked selected, the warnings as notices; and whether any job ran.
func (c *logsCommand) viewOf(read runlogs.Outcome, req runlogs.Request) (runview.Params, bool) {
	daemon := viewDaemon{Daemon: c.view.Daemon, logged: map[logKey]runlogs.Job{}}
	params := runview.Params{Daemon: daemon, Open: c.view.Open, Job: req.Job, In: c.io.Streams.In, Out: c.io.Streams.Err}
	ran := false
	for _, item := range read.Items {
		if item.Detail == nil {
			continue
		}
		wt := runview.Worktree{Branch: item.Subject, Path: item.Detail.Path}
		for _, job := range item.Detail.Jobs {
			wt.Jobs = append(wt.Jobs, job.Name)
			ran = ran || job.State != runlogs.JobNotStarted
			if job.SharedIn != "" {
				params.Main = job.SharedIn
			}
			daemon.logged[logKey{dir: item.Detail.Path, name: job.Name}] = job
		}
		params.Worktrees = append(params.Worktrees, wt)
	}
	for _, item := range read.Items {
		for _, warning := range item.Warnings {
			params.Notices = append(params.Notices, warning.Message)
		}
	}
	for _, warning := range read.Warnings {
		params.Notices = append(params.Notices, warning.Message)
	}
	return params, ran
}

type logKey struct {
	dir  string
	name string
}

// viewDaemon is the daemon as the live view of run logs asks it: a job only its log knows (the daemon no longer lists it) is listed stopped, and the lines of a job the daemon has none of (one it forgot, or the legacy daemon's, which keeps none) are those read.
type viewDaemon struct {
	runview.Daemon
	logged map[logKey]runlogs.Job
}

// Jobs lists the daemon's jobs, then those only their log knows.
func (d viewDaemon) Jobs(ctx context.Context) ([]run.Job, error) {
	jobs, err := d.Daemon.Jobs(ctx)
	if err != nil && !errors.As(err, new(*run.Skewed)) {
		return jobs, err
	}
	for key, job := range d.logged {
		unlisted := job.Hold == "" && job.State == runlogs.JobStopped
		if unlisted && !slices.ContainsFunc(jobs, func(j run.Job) bool { return j.WorkDir == key.dir && j.Name == key.name }) {
			jobs = append(jobs, run.Job{Name: key.name, WorkDir: key.dir, Kind: job.Kind, State: run.Stopped})
		}
	}
	return jobs, err
}

// Lines are the daemon's, or those read of the job when the daemon has none to give.
func (d viewDaemon) Lines(ctx context.Context, req run.Tail, out func(run.Line)) (run.Job, error) {
	job, logged := d.logged[logKey{dir: req.WorkDir, name: req.Name}]
	listed, err := d.Daemon.Lines(ctx, req, out)
	if err == nil || !logged || len(job.Lines) == 0 {
		return listed, err
	}
	for _, line := range job.Lines {
		out(run.Line{At: line.At, Text: line.Text})
	}
	return run.Job{Name: req.Name, WorkDir: req.WorkDir, Kind: job.Kind, State: run.State(job.State)}, nil
}
