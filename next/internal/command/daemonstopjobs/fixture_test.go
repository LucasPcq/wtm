package daemonstopjobs_test

import (
	"context"
	"errors"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// A daemon is wtm's daemon a test scripts: how it stands, the stops it refuses (by name and work dir), and each stop it got.
type daemon struct {
	status run.Daemon
	refuse map[string]error

	mu      sync.Mutex
	stopped []string
}

func (d *daemon) Status(context.Context) (run.Daemon, error) { return d.status, nil }

func (d *daemon) Stop(_ context.Context, req run.Stop, _ func(run.Line)) (run.Outcome, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopped = append(d.stopped, req.Name+" "+req.WorkDir)
	if err := d.refuse[req.Name+" "+req.WorkDir]; err != nil {
		return "", err
	}
	return run.OutcomeStopped, nil
}

var errStack = errors.New("stack: docker compose down exited 1: still running mailpit")

// The jobs of two repositories: acme's main runs web, a shared db feat/a holds, a stack, and a crashed api; feat/a runs worker; wtm's main runs docs.
var (
	web    = run.Job{Name: "web", Repo: "/src/acme", Branch: "main", WorkDir: "/src/acme", Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running}
	db     = run.Job{Name: "db", Repo: "/src/acme", Branch: "main", WorkDir: "/src/acme", Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running, SharedDir: "/src/acme"}
	dbHeld = run.Job{Name: "db", Repo: "/src/acme", Branch: "feat/a", WorkDir: "/src/acme.trees/feat-a", Up: true, Kind: project.Service, Hold: run.HoldClaim, SharedDir: "/src/acme"}
	stack  = run.Job{Name: "mail", Repo: "/src/acme", Branch: "main", WorkDir: "/src/acme", Up: true, Kind: project.Service, Hold: run.HoldDetached, State: run.Running, StopCommand: "docker compose down"}
	api    = run.Job{Name: "api", Repo: "/src/acme", Branch: "main", WorkDir: "/src/acme", Kind: project.Service, Hold: run.HoldOwn, State: run.Crashed}
	worker = run.Job{Name: "worker", Repo: "/src/acme", Branch: "feat/a", WorkDir: "/src/acme.trees/feat-a", Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running}
	docs   = run.Job{Name: "docs", Repo: "/src/wtm", Branch: "main", WorkDir: "/src/wtm", Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running}
)

// running is a daemon of this build holding jobs.
func running(jobs ...run.Job) *daemon {
	return &daemon{status: run.Daemon{Answering: true, Stamp: run.Stamp{Protocol: run.ProtocolVersion, Version: "0.31.0"}, Jobs: jobs}}
}
