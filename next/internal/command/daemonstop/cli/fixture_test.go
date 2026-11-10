package cli

import (
	"context"
	"errors"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstop"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/operation"
	"github.com/LucasPcq/wtm/next/internal/service/operationtest"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
)

// A world is wtm's daemon a test stops: how it stands and whether its shutdown fails.
type world struct {
	daemon *daemon
}

type daemon struct {
	status run.Daemon
	fail   error
}

func (d *daemon) Status(context.Context) (run.Daemon, error) { return d.status, nil }

func (d *daemon) Shutdown(context.Context) error {
	if d.fail == nil {
		d.status = run.Daemon{}
	}
	return d.fail
}

// newWorld is a daemon holding acme's web, a shared db feat/a holds, and a stack.
func newWorld() world {
	return world{daemon: &daemon{status: run.Daemon{Answering: true, Stamp: run.Stamp{Protocol: run.ProtocolVersion, Version: "0.31.0", PID: 42}, Jobs: []run.Job{
		{Name: "web", Repo: "/src/acme", Branch: "main", WorkDir: "/src/acme", Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running},
		{Name: "db", Repo: "/src/acme", Branch: "main", WorkDir: "/src/acme", Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running, SharedDir: "/src/acme"},
		{Name: "db", Repo: "/src/acme", Branch: "feat/a", WorkDir: "/src/acme.trees/feat-a", Up: true, Kind: project.Service, Hold: run.HoldClaim, SharedDir: "/src/acme"},
		{Name: "mail", Repo: "/src/acme", Branch: "feat/a", WorkDir: "/src/acme.trees/feat-a", Up: true, Kind: project.Service, Hold: run.HoldDetached, State: run.Running},
	}}}}
}

var errStuck = errors.New("wtm's daemon did not stop within 30s")

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: "/src/acme", ProjectDir: "/src/acme"}
}

func (w world) open(context.Context, string) (daemonstop.UseCase, error) {
	locks := &operationtest.Locks{}
	return daemonstop.UseCase{Operations: operation.Service{Locker: locks, Publisher: locks, NewOpID: func() string { return "op-1" }, Now: time.Now}, Daemon: w.daemon}, nil
}
