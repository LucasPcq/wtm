// Package run is what the run daemon says of the jobs it runs.
package run

import "errors"

// A Job is one job the daemon knows: its name, the worktree it runs in, and whether it is up (running, detached or joined).
type Job struct {
	Name    string
	WorkDir string
	Up      bool
}

// ErrUnknown says whether a job is up cannot be told: no daemon answers, yet its index holds jobs it would run.
var ErrUnknown = errors.New("the run daemon is not running: which jobs are up is unknown")
