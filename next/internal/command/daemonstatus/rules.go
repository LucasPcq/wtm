package daemonstatus

import (
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what daemon status reports from: the daemon as read, and this wtm's build.
type Facts struct {
	Daemon  run.Daemon
	Version string
}

// ----------------------------------------------------------------------------
// The detail

// A Detail is the daemon as the JSON envelope reports it: whether it runs, its pid, this wtm's build and the daemon's, the protocol it speaks and its skew (empty: the same build), whether its jobs could not be read (another protocol), the jobs it holds by repository (with none running, the stacks and claims its index holds, which run on without it), its proxy (none when it serves none), its socket and its index, and whether a newer wtm wrote that index.
type Detail struct {
	Running        bool           `json:"running"`
	PID            int            `json:"pid,omitempty"`
	Build          string         `json:"build"`
	Daemon         string         `json:"daemon,omitempty"`
	Protocol       int            `json:"protocol,omitempty"`
	Skew           run.Skew       `json:"skew,omitempty"`
	JobsUnreadable bool           `json:"jobs_unreadable,omitempty"`
	Repos          []run.HeldRepo `json:"repos"`
	Proxy          *Proxy         `json:"proxy,omitempty"`
	Socket         string         `json:"socket"`
	Index          string         `json:"index"`
	IndexFrozen    bool           `json:"index_frozen,omitempty"`
}

// A Proxy is the proxy the daemon serves named addresses on: the port it bound, and the one an address announces (80 once the redirection is installed).
type Proxy struct {
	Port       int `json:"port"`
	PublicPort int `json:"public_port"`
}

// ----------------------------------------------------------------------------
// Report

// The codes of what a reader of the daemon should know: a newer wtm wrote its index, it is of another build.
const (
	CodeIndexFrozen = Name + domain.IndexFrozen
	CodeDaemonSkew  = Name + domain.DaemonSkew
)

// ReportParams are what Report reports with besides the facts: the follow-up that restarts a daemon of another build.
type ReportParams struct {
	Restart *domain.FollowUp
}

// Report is the read of the daemon: its one item, and a warning for a frozen index and for a daemon of another build, followed by its restart.
func Report(f Facts, params ReportParams) Outcome {
	d := f.Daemon
	detail := &Detail{Running: d.Answering, Build: f.Version, Socket: d.Socket, Index: d.Index, IndexFrozen: d.IndexFrozen, Repos: run.Held(up(d.Jobs))}
	if d.Answering {
		detail.PID, detail.Daemon, detail.Protocol, detail.Skew = d.Stamp.PID, d.Stamp.Version, d.Stamp.Protocol, d.Skew
		detail.JobsUnreadable = d.Skew == run.SkewProtocol
	}
	if d.Proxy.Port > 0 {
		detail.Proxy = &Proxy{Port: d.Proxy.Port, PublicPort: d.Proxy.PublicPort}
	}

	var warnings []domain.Warning
	if d.IndexFrozen {
		warnings = append(warnings, domain.IndexFrozenWarning(Name, d.Index))
	}
	if d.Answering && d.Skew != run.SkewNone {
		warnings = append(warnings, domain.SkewWarning(domain.SkewWarningParams{Command: Name, Skewed: &run.Skewed{Skew: d.Skew, Daemon: d.Stamp.Version}, Restart: params.Restart}))
	}

	return Outcome{Items: []Item{{Subject: "daemon", Status: domain.StatusDone, Detail: detail}}, Warnings: warnings}
}

// up are the jobs of jobs that are up.
func up(jobs []run.Job) []run.Job {
	return slices.DeleteFunc(slices.Clone(jobs), func(job run.Job) bool { return !job.Up })
}
