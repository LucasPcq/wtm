// Package run is what the run daemon says of the jobs it runs.
package run

import (
	"errors"
	"slices"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
)

// A Job is one job the daemon knows: its name, the worktree it runs in, whether it is up (running, detached or joined), its kind, its State (empty for a Claim, which runs no process), whether it is a Claim a worktree holds on a shared job another worktree runs, its url, the code its process exited with (-1 when a signal killed it), when the daemon started it, and the main checkout a shared job runs in.
type Job struct {
	Name      string
	WorkDir   string
	Up        bool
	Kind      project.JobKind
	State     State
	Claim     bool
	URL       string
	ExitCode  *int
	StartedAt time.Time
	SharedDir string
}

// State is a job's state, as wtm's events report it.
type State string

// The states of a job.
const (
	Starting State = "starting"
	Running  State = "running"
	Crashed  State = "crashed"
	Exited   State = "exited" // a task that ran to its end
	Stopped  State = "stopped"
)

// States returns every State.
func States() []State {
	return []State{Starting, Running, Crashed, Exited, Stopped}
}

// ErrUnknown says whether a job is up cannot be told: no daemon answers, yet its index holds jobs it would run.
var ErrUnknown = errors.New("the run daemon is not running: which jobs are up is unknown")

// UpIn returns the names of the jobs up in the worktree at dir, sorted, empty and not nil when none is.
func UpIn(jobs []Job, dir string) []string {
	names := []string{}
	for _, job := range jobs {
		if job.Up && job.WorkDir == dir {
			names = append(names, job.Name)
		}
	}
	slices.Sort(names)

	return names
}

// URLsIn returns the URLs of the jobs up in the worktree at dir that have one, sorted, empty and not nil when none has.
func URLsIn(jobs []Job, dir string) []string {
	urls := []string{}
	for _, job := range jobs {
		if job.Up && job.WorkDir == dir && job.URL != "" {
			urls = append(urls, job.URL)
		}
	}
	slices.Sort(urls)

	return urls
}

// UnknownMessage says why the jobs up are unknown, err being why the daemon could not tell.
func UnknownMessage(err error) string {
	if errors.Is(err, ErrUnknown) {
		return ErrUnknown.Error()
	}

	return "couldn't ask the run daemon which jobs are up: " + err.Error()
}
