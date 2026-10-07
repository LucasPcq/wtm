// Package runjobs reads the run daemon's index of jobs. It exists so a surface
// that is not a cobra command — the dashboard — can ask the same question the
// CLI asks, without reaching into internal/commands.
package runjobs

import (
	"github.com/LucasPcq/wtm/internal/config"
	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// Listing is what the daemon holds, and which build answered: an older daemon
// is still listed, and a surface says it runs the jobs its own way.
type Listing struct {
	Jobs          []domain.JobInfo
	DaemonVersion string
	Reached       bool
}

// Diverged says a daemon of another build answered.
func (l Listing) Diverged() bool {
	return l.Reached && l.DaemonVersion != domain.Version
}

// List fetches the daemon's jobs. A daemon exits once no foreground job is left,
// so nobody listening says nothing about whether detached stacks are up: when
// the index still holds some, one is started to read them back. When it holds
// nothing there is nothing to report, and no daemon is forked for it.
func List() Listing {
	socketPath := process.SocketPath()
	if !process.IsDaemonRunning(socketPath) {
		if !process.HasAnyIndexedJob() {
			return Listing{}
		}
		global, err := config.LoadGlobal()
		if err != nil {
			return Listing{}
		}
		if err := process.EnsureDaemon(process.DaemonParams{
			SocketPath: socketPath,
			ProxyPort:  rules.ProxyPort(global),
		}); err != nil {
			return Listing{}
		}
	}
	resp, err := process.NewClient(socketPath).Send(process.Request{Action: process.ActionList})
	if err != nil {
		return Listing{}
	}
	return Listing{Jobs: liveJobs(resp.Jobs), DaemonVersion: resp.Version, Reached: true}
}

// liveJobs asks the disk once per worktree: the daemon keeps a stopped job's
// entry after its worktree was cleaned, and never runs git to find out.
func liveJobs(jobs []domain.JobInfo) []domain.JobInfo {
	exists := map[string]bool{}
	for _, job := range jobs {
		if _, seen := exists[job.WorkDir]; !seen {
			exists[job.WorkDir] = infra.FileExists(job.WorkDir)
		}
	}
	return rules.JobsOfLiveWorktrees(rules.JobsOfLiveWorktreesParams{Jobs: jobs, Exists: exists})
}

// Load is List for a caller whose question is only what is running.
func Load() []domain.JobInfo {
	return List().Jobs
}

// Peek is Load for a reader whose question does not justify waking anything —
// the dashboard's poll, since forking a daemon every three seconds is not what
// a background refresh is for. The second result says whether it could tell at
// all: a daemon exits once no foreground job is left, so nobody listening while
// the index still holds jobs means "cannot say", never "nothing is running". A
// caller that took that silence for an answer would blink a detached stack out
// of its panel on every poll.
func Peek() (jobs []domain.JobInfo, known bool) {
	socketPath := process.SocketPath()
	if !process.IsDaemonRunning(socketPath) {
		return nil, !process.HasAnyIndexedJob()
	}
	resp, err := process.NewClient(socketPath).Send(process.Request{Action: process.ActionList})
	if err != nil {
		return nil, false
	}
	return liveJobs(resp.Jobs), true
}

// Current is what is up without waking anything: the daemon when one listens,
// else the index read back the way a daemon starting now would read it.
func Current() []domain.JobInfo {
	socketPath := process.SocketPath()
	if !process.IsDaemonRunning(socketPath) {
		return liveJobs(process.IndexedJobs())
	}
	resp, err := process.NewClient(socketPath).Send(process.Request{Action: process.ActionList})
	if err != nil {
		return liveJobs(process.IndexedJobs())
	}
	return liveJobs(resp.Jobs)
}
