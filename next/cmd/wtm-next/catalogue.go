package main

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/api"
	"github.com/LucasPcq/wtm/next/internal/command/checkout"
	"github.com/LucasPcq/wtm/next/internal/command/clean"
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/command/daemonrestart"
	"github.com/LucasPcq/wtm/next/internal/command/daemonstatus"
	"github.com/LucasPcq/wtm/next/internal/command/daemonstop"
	"github.com/LucasPcq/wtm/next/internal/command/daemonstopjobs"
	envcommand "github.com/LucasPcq/wtm/next/internal/command/env"
	"github.com/LucasPcq/wtm/next/internal/command/envshow"
	"github.com/LucasPcq/wtm/next/internal/command/events"
	"github.com/LucasPcq/wtm/next/internal/command/exec"
	"github.com/LucasPcq/wtm/next/internal/command/extract"
	"github.com/LucasPcq/wtm/next/internal/command/fastforward"
	"github.com/LucasPcq/wtm/next/internal/command/initialize"
	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/command/prune"
	"github.com/LucasPcq/wtm/next/internal/command/reparent"
	"github.com/LucasPcq/wtm/next/internal/command/resolve"
	"github.com/LucasPcq/wtm/next/internal/command/rundown"
	"github.com/LucasPcq/wtm/next/internal/command/runlist"
	"github.com/LucasPcq/wtm/next/internal/command/runlogs"
	"github.com/LucasPcq/wtm/next/internal/command/runps"
	"github.com/LucasPcq/wtm/next/internal/command/runstart"
	"github.com/LucasPcq/wtm/next/internal/command/runstop"
	"github.com/LucasPcq/wtm/next/internal/command/runup"
	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/command/sync"
	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/web"
)

// catalogue lists every command for a surface other than the command line, one line per command.
func catalogue() api.Catalogue {
	return api.Catalogue{
		api.Bind(initialize.Name, openInit, initialize.UseCase.Questions, initialize.UseCase.Run),
		api.Bind(create.Name, openCreate, create.UseCase.Questions, create.UseCase.Run),
		api.Bind(extract.Name, openExtract, extract.UseCase.Questions, extract.UseCase.Run),
		api.Bind(checkout.Name, openCheckout, checkout.UseCase.Questions, checkout.UseCase.Run),
		api.Bind(clean.Name, openClean, clean.UseCase.Questions, clean.UseCase.Run),
		api.Bind(prune.Name, openPrune, prune.UseCase.Questions, prune.UseCase.Run),
		api.Bind(fastforward.Name, openFastForward, fastforward.UseCase.Questions, fastforward.UseCase.Run),
		api.Bind(envcommand.Name, openEnv, envcommand.UseCase.Questions, envcommand.UseCase.Run),
		api.Bind(sync.Name, openSync, sync.UseCase.Questions, sync.UseCase.Run),
		api.Bind(exec.Name, openExec, exec.UseCase.Questions, exec.UseCase.Run),
		api.Bind(reparent.Name, openReparent, reparent.UseCase.Questions, reparent.UseCase.Run),
		api.Bind(list.Name, openList, list.UseCase.Questions, list.UseCase.Run),
		api.Bind(resolve.Name, openResolve, resolve.UseCase.Questions, resolve.UseCase.Run),
		api.Bind(status.Name, openStatus, status.UseCase.Questions, status.UseCase.Run),
		api.Bind(tree.Name, openTree, tree.UseCase.Questions, tree.UseCase.Run),
		api.Bind(runup.Name, openRunUp, runup.UseCase.Questions, runup.UseCase.Run),
		api.Bind(runlogs.Name, openRunLogs, runlogs.UseCase.Questions, runlogs.UseCase.Run),
		api.Bind(rundown.Name, openRunDown, rundown.UseCase.Questions, rundown.UseCase.Run),
		api.Bind(runstart.Name, openRunStart, runstart.UseCase.Questions, runstart.UseCase.Run),
		api.Bind(runstop.Name, openRunStop, runstop.UseCase.Questions, runstop.UseCase.Run),
		api.Bind(runps.Name, openRunPs, runps.UseCase.Questions, runps.UseCase.Run),
		api.Bind(runlist.Name, openRunList, runlist.UseCase.Questions, runlist.UseCase.Run),
		api.Bind(envshow.Name, openEnvShow, envshow.UseCase.Questions, envshow.UseCase.Run),
		api.Bind(daemonstatus.Name, openDaemonStatus, daemonstatus.UseCase.Questions, daemonstatus.UseCase.Run),
		api.Bind(daemonstop.Name, openDaemonStop, daemonstop.UseCase.Questions, daemonstop.UseCase.Run),
		api.Bind(daemonrestart.Name, openDaemonRestart, daemonrestart.UseCase.Questions, daemonrestart.UseCase.Run),
		api.Bind(daemonstopjobs.Name, openDaemonStopJobs, daemonstopjobs.UseCase.Questions, daemonstopjobs.UseCase.Run),
	}
}

// streams lists every command that streams rather than returns, for a surface other than the command line, one line per command.
func streams() []api.Stream {
	return []api.Stream{
		api.BindStream[events.Request, events.Questions](events.Name, watchEvents(systemStream())),
	}
}

// watchEvents is the watch of the event stream over stream for a surface: each line goes out as its JSON, each warning as it is.
func watchEvents(stream events.UseCase) func(ctx context.Context, req events.Request, sink api.Sink) error {
	return func(ctx context.Context, req events.Request, sink api.Sink) error {
		return stream.Watch(ctx, req, events.Out{Emit: func(received events.Received) error { return sink.Send(received.JSON) }, Warn: sink.Warn})
	}
}

// jobLines is run logs' stream for a surface: the lines of a job, each one line of JSON its type names (runlogs.JSONOf), its warnings among them; the use case is opened on the call's directory.
func jobLines() api.Stream {
	return api.BindStream[runlogs.Request, runlogs.Questions](runlogs.Name, func(ctx context.Context, req runlogs.Request, sink api.Sink) error {
		logs, err := openRunLogs(ctx, req.Caller.Dir)
		if err != nil {
			return err
		}
		send := func(v any) error {
			line, err := runlogs.JSONOf(v)
			if err != nil {
				return err
			}
			return sink.Send(line)
		}
		return logs.Watch(ctx, req, runlogs.Out{
			Settled: sink.Open,
			Emit:    func(entry runlogs.Entry) error { return send(entry) },
			Absent:  func(absent runlogs.Absent) error { return send(absent) },
			Ended:   func(ended runlogs.Ended) error { return send(ended) },
			Warn:    func(warning domain.Warning) { _ = send(warning) },
		})
	})
}

// jobLogFile is the log file of the job of a repository's worktree job names, as run logs reads it: "" when it keeps none.
func jobLogFile(ctx context.Context, job web.JobLog) (string, error) {
	logs, err := openRunLogs(ctx, job.Dir)
	if err != nil {
		return "", err
	}
	read, err := logs.Run(ctx, runlogs.Request{Worktrees: []string{job.Worktree}, Job: job.Job, Caller: domain.Caller{Dir: job.Dir, Named: true, Cwd: job.Dir}}, domain.RunOptions{Surface: domain.SurfaceAPI})
	if err != nil {
		return "", err
	}
	for _, item := range read.Items {
		if item.Detail != nil && len(item.Detail.Jobs) > 0 {
			return item.Detail.Jobs[0].Log, nil
		}
	}
	return "", nil
}
