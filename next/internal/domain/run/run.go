// Package run is what the run daemon says of the jobs it runs.
package run

import (
	"errors"
	"slices"
)

// A Job is one job the daemon knows: its name, the worktree it runs in, and whether it is up (running, detached or joined).
type Job struct {
	Name    string
	WorkDir string
	Up      bool
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

// UnknownMessage says why the jobs up are unknown, err being why the daemon could not tell.
func UnknownMessage(err error) string {
	if errors.Is(err, ErrUnknown) {
		return ErrUnknown.Error()
	}

	return "couldn't ask the run daemon which jobs are up: " + err.Error()
}
