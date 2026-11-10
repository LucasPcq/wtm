package daemonstatus_test

import (
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

const build = "0.31.0"

// held are the jobs of two repositories: acme's main runs web and a shared db feat/a holds, feat/a runs a stack; wtm's main a crashed docs job, not up.
func held() []run.Job {
	return []run.Job{
		{Name: "web", Repo: "/acme", Branch: "main", WorkDir: "/acme", Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running},
		{Name: "db", Repo: "/acme", Branch: "main", WorkDir: "/acme", Up: true, Kind: project.Service, Hold: run.HoldOwn, State: run.Running, SharedDir: "/acme"},
		{Name: "db", Repo: "/acme", Branch: "feat/a", WorkDir: "/acme/.trees/feat-a", Up: true, Kind: project.Service, Hold: run.HoldClaim, SharedDir: "/acme"},
		{Name: "cache", Repo: "/acme", Branch: "feat/a", WorkDir: "/acme/.trees/feat-a", Up: true, Kind: project.Service, Hold: run.HoldDetached, State: run.Running},
		{Name: "docs", Repo: "/wtm", Branch: "main", WorkDir: "/wtm", Kind: project.Service, Hold: run.HoldOwn, State: run.Crashed},
	}
}

// answering is a daemon of stamp holding held's jobs, its proxy on 11080.
func answering(stamp run.Stamp, skew run.Skew) run.Daemon {
	return run.Daemon{Answering: true, Stamp: stamp, Skew: skew, Jobs: held(), Proxy: run.Proxy{Port: 11080, PublicPort: 11080}, Socket: "/cfg/wtm/wtm.sock", Index: "/cfg/wtm/jobs.json"}
}
