package daemonrestart_test

import (
	"context"
	"errors"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// A daemon is wtm's daemon a test scripts: how it stands, its index, what fails, and each call it got.
type daemon struct {
	status   run.Daemon
	records  []run.Record
	shutdown error
	ensure   error
	refuse   map[string]error // the starts refused, by job name and work dir
	calls    []string
	started  []run.Spec
}

func (d *daemon) Status(context.Context) (run.Daemon, error) { return d.status, nil }

func (d *daemon) Records() []run.Record { return d.records }

func (d *daemon) Shutdown(context.Context) error {
	d.calls = append(d.calls, "shutdown")
	return d.shutdown
}

func (d *daemon) Ensure(context.Context) error {
	d.calls = append(d.calls, "ensure")
	return d.ensure
}

func (d *daemon) Start(_ context.Context, spec run.Spec, _ func(run.Line)) (run.Job, error) {
	d.calls = append(d.calls, "start "+spec.Job.Name+" "+spec.WorkDir)
	d.started = append(d.started, spec)
	return run.Job{Name: spec.Job.Name, WorkDir: spec.WorkDir}, d.refuse[spec.Job.Name+" "+spec.WorkDir]
}

var errPort = errors.New("listen tcp :3000: address already in use")

const (
	main  = "/acme"
	featA = "/acme.trees/feat-a"
)

// The jobs of acme as the daemon lists them: main runs web and a shared db feat/a holds, a migrate task still running, and a stack; feat/a runs api.
var (
	web     = run.Job{Name: "web", Repo: main, Branch: "main", WorkDir: main, Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running}
	db      = run.Job{Name: "db", Repo: main, Branch: "main", WorkDir: main, Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running, SharedDir: main}
	dbHeld  = run.Job{Name: "db", Repo: main, Branch: "feat/a", WorkDir: featA, Up: true, Kind: project.Service, Hold: run.HoldClaim, SharedDir: main}
	migrate = run.Job{Name: "migrate", Repo: main, Branch: "main", WorkDir: main, Up: true, Kind: project.Task, Hold: run.HoldOwn, State: run.Running}
	stack   = run.Job{Name: "mail", Repo: main, Branch: "main", WorkDir: main, Up: true, Kind: project.Service, Hold: run.HoldDetached, State: run.Running}
	api     = run.Job{Name: "api", Repo: main, Branch: "feat/a", WorkDir: featA, Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running}
)

// recordOf is the record of job as its start left it in the index.
func recordOf(job run.Job) run.Record {
	spec := run.Spec{Job: project.Job{Name: job.Name, Kind: job.Kind, Cmd: "run " + job.Name}, Profile: "dev", WorkDir: job.WorkDir, Env: map[string]string{"PORT": "3000"}, Origin: &run.Origin{Root: job.Repo, Branch: job.Branch, CorrelationID: "up-1"}}
	if job.Hold == run.HoldDetached {
		spec.Job.Stop = "stop " + job.Name
	}
	if job.SharedDir != "" {
		spec.Job.Scope = project.Shared
		spec.Shared = &run.Shared{WorkDir: job.SharedDir}
	}
	return run.Record{Spec: spec, Claim: job.Hold == run.HoldClaim, SharedDir: job.SharedDir}
}

// running is a daemon of build holding jobs, each recorded in its index.
func running(build string, jobs ...run.Job) *daemon {
	d := &daemon{status: run.Daemon{Answering: true, Stamp: run.Stamp{Protocol: run.ProtocolVersion, Version: build, PID: 42}, Skew: run.SkewBuild, Jobs: jobs, Index: "/cfg/wtm/jobs.json"}}
	for _, job := range jobs {
		d.records = append(d.records, recordOf(job))
	}
	return d
}
