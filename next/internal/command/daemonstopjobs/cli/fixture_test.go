package cli

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstopjobs"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
)

// A world is wtm's daemon a test stops the jobs of.
type world struct {
	daemon *daemon
}

type daemon struct {
	status run.Daemon
	refuse map[string]error
	mu     sync.Mutex
}

func (d *daemon) Status(context.Context) (run.Daemon, error) { return d.status, nil }

func (d *daemon) Stop(_ context.Context, req run.Stop, _ func(run.Line)) (run.Outcome, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.refuse[req.Name]; err != nil {
		return "", err
	}
	return run.OutcomeStopped, nil
}

var errStack = errors.New("docker compose down exited 1")

// newWorld is a daemon holding acme's web, a shared db feat/a holds, a stack, a crashed api, and wtm's docs.
func newWorld() world {
	return world{daemon: &daemon{status: run.Daemon{Answering: true, Stamp: run.Stamp{Protocol: run.ProtocolVersion, Version: "0.31.0"}, Jobs: []run.Job{
		{Name: "web", Repo: "/src/acme", Branch: "main", WorkDir: "/src/acme", Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running},
		{Name: "db", Repo: "/src/acme", Branch: "main", WorkDir: "/src/acme", Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running, SharedDir: "/src/acme"},
		{Name: "db", Repo: "/src/acme", Branch: "feat/a", WorkDir: "/src/acme.trees/feat-a", Up: true, Kind: project.Service, Hold: run.HoldClaim, SharedDir: "/src/acme"},
		{Name: "mail", Repo: "/src/acme", Branch: "feat/a", WorkDir: "/src/acme.trees/feat-a", Up: true, Kind: project.Service, Hold: run.HoldDetached, State: run.Running, StopCommand: "docker compose down"},
		{Name: "api", Repo: "/src/acme", Branch: "main", WorkDir: "/src/acme", Kind: project.Service, Hold: run.HoldOwn, State: run.Crashed},
		{Name: "docs", Repo: "/src/wtm", Branch: "main", WorkDir: "/src/wtm", Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running},
	}}}}
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: "/src/acme", ProjectDir: "/src/acme"}
}

func (w world) open(context.Context, string) (daemonstopjobs.UseCase, error) {
	locks := &operationtest.Locks{}
	return daemonstopjobs.UseCase{
		Operations: operation.Service{Locker: locks, Publisher: locks, NewOpID: func() string { return "op-1" }, Now: time.Now},
		Daemon:     w.daemon,
		RepoRoot:   func(context.Context, string) (string, error) { return "/src/acme", nil },
	}, nil
}
