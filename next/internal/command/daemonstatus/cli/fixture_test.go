package cli

import (
	"context"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstatus"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	uicli "github.com/LucasPcq/wtm/next/internal/ui/cli"
)

// A world is the daemon a test reads.
type world struct {
	Daemon run.Daemon
}

func (w world) env() uicli.Env {
	return uicli.Env{Cwd: "/acme", ProjectDir: "/acme"}
}

func (w world) open(context.Context, string) (daemonstatus.UseCase, error) {
	return daemonstatus.UseCase{Daemon: func(context.Context) (run.Daemon, error) { return w.Daemon, nil }, Version: "0.31.0", Restart: &domain.FollowUp{Command: "daemon-restart"}}, nil
}

// running is a daemon of stamp holding the jobs of two repositories.
func running(stamp run.Stamp, skew run.Skew) world {
	return world{Daemon: run.Daemon{Answering: true, Stamp: stamp, Skew: skew, Proxy: run.Proxy{Port: 11080, PublicPort: 80}, Socket: "/cfg/wtm/wtm.sock", Index: "/cfg/wtm/jobs.json", Jobs: []run.Job{
		{Name: "web", Repo: "/src/acme", Branch: "main", WorkDir: "/src/acme", Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running},
		{Name: "db", Repo: "/src/acme", Branch: "main", WorkDir: "/src/acme", Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running, SharedDir: "/src/acme"},
		{Name: "db", Repo: "/src/acme", Branch: "feat/a", WorkDir: "/src/acme.trees/feat-a", Up: true, Kind: project.Service, Hold: run.HoldClaim, SharedDir: "/src/acme"},
		{Name: "mail", Repo: "/src/acme", Branch: "feat/a", WorkDir: "/src/acme.trees/feat-a", Up: true, Kind: project.Service, Hold: run.HoldDetached, State: run.Running},
		{Name: "docs", Repo: "/src/wtm", Branch: "main", WorkDir: "/src/wtm", Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running},
	}}}
}
