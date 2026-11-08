package daemonstop_test

import (
	"context"
	"errors"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// A daemon is wtm's daemon a test scripts: how it stands, whether its shutdown fails, and how many it was asked.
type daemon struct {
	status    run.Daemon
	fail      error
	shutdowns int
}

func (d *daemon) Status(context.Context) (run.Daemon, error) {
	return d.status, nil
}

func (d *daemon) Shutdown(context.Context) error {
	d.shutdowns++
	if d.fail == nil {
		d.status = run.Daemon{}
	}
	return d.fail
}

var errStuck = errors.New("wtm's daemon did not stop within 30s")

// The jobs of acme: main runs web and a shared db feat/a holds, and a stack; feat/a holds a claim on the shared stack too.
var (
	web       = run.Job{Name: "web", Repo: "/acme", Branch: "main", WorkDir: "/acme", Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running}
	db        = run.Job{Name: "db", Repo: "/acme", Branch: "main", WorkDir: "/acme", Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running, SharedDir: "/acme"}
	dbHeld    = run.Job{Name: "db", Repo: "/acme", Branch: "feat/a", WorkDir: "/acme/feat-a", Up: true, Kind: project.Service, Hold: run.HoldClaim, SharedDir: "/acme"}
	cache     = run.Job{Name: "cache", Repo: "/acme", Branch: "main", WorkDir: "/acme", Up: true, Kind: project.Service, Hold: run.HoldDetached, State: run.Running, SharedDir: "/acme"}
	cacheHeld = run.Job{Name: "cache", Repo: "/acme", Branch: "feat/a", WorkDir: "/acme/feat-a", Up: true, Kind: project.Service, Hold: run.HoldClaim, SharedDir: "/acme"}
)

// running is a daemon of this build holding jobs.
func running(jobs ...run.Job) *daemon {
	return &daemon{status: run.Daemon{Answering: true, Stamp: run.Stamp{Protocol: run.ProtocolVersion, Version: "0.31.0", PID: 42}, Jobs: jobs}}
}
