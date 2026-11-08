package cli

import (
	"context"
	"errors"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/daemonrestart"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
)

// A world is wtm's daemon a test restarts.
type world struct {
	daemon *daemon
}

type daemon struct {
	status  run.Daemon
	records []run.Record
	refuse  map[string]error
}

func (d *daemon) Status(context.Context) (run.Daemon, error) { return d.status, nil }
func (d *daemon) Records() []run.Record                      { return d.records }
func (d *daemon) Shutdown(context.Context) error             { return nil }
func (d *daemon) Ensure(context.Context) error               { return nil }

func (d *daemon) Start(_ context.Context, spec run.Spec, _ func(run.Line)) (run.Job, error) {
	return run.Job{Name: spec.Job.Name, WorkDir: spec.WorkDir}, d.refuse[spec.Job.Name]
}

var errPort = errors.New("listen tcp :3000: address already in use")

// newWorld is a 0.30.9 daemon holding acme's web, a shared db feat/a holds, a stack, and a task still running.
func newWorld() world {
	jobs := []run.Job{
		{Name: "web", Repo: "/src/acme", Branch: "main", WorkDir: "/src/acme", Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running},
		{Name: "db", Repo: "/src/acme", Branch: "main", WorkDir: "/src/acme", Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running, SharedDir: "/src/acme"},
		{Name: "db", Repo: "/src/acme", Branch: "feat/a", WorkDir: "/src/acme.trees/feat-a", Up: true, Kind: project.Service, Hold: run.HoldClaim, SharedDir: "/src/acme"},
		{Name: "mail", Repo: "/src/acme", Branch: "feat/a", WorkDir: "/src/acme.trees/feat-a", Up: true, Kind: project.Service, Hold: run.HoldDetached, State: run.Running},
		{Name: "seed", Repo: "/src/acme", Branch: "main", WorkDir: "/src/acme", Up: true, Kind: project.Task, Hold: run.HoldOwn, State: run.Running},
	}
	d := &daemon{status: run.Daemon{Answering: true, Stamp: run.Stamp{Protocol: run.ProtocolVersion, Version: "0.30.9", PID: 42}, Skew: run.SkewBuild, Jobs: jobs}}
	for _, job := range jobs {
		spec := run.Spec{Job: project.Job{Name: job.Name, Kind: job.Kind}, WorkDir: job.WorkDir}
		if job.Hold == run.HoldDetached {
			spec.Job.Stop = "docker compose down"
		}
		d.records = append(d.records, run.Record{Spec: spec, Claim: job.Hold == run.HoldClaim, SharedDir: job.SharedDir})
	}
	return world{daemon: d}
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: "/src/acme", ProjectDir: "/src/acme"}
}

func (w world) open(context.Context, string) (daemonrestart.UseCase, error) {
	locks := &operationtest.Locks{}
	return daemonrestart.UseCase{Operations: operation.Service{Locker: locks, Publisher: locks, NewOpID: func() string { return "op-1" }, Now: time.Now}, Daemon: w.daemon, Version: "0.31.0"}, nil
}
