// Package daemon asks the run daemon which jobs it runs, over its socket, without ever starting it.
package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

const (
	askTimeout  = 2 * time.Second
	indexName   = "jobs.json"
	actionList  = "list"
	statusError = "error"
)

// Daemon is the run daemon listening on SocketPath, whose index of the jobs it runs lies beside it.
type Daemon struct {
	SocketPath string
}

// Jobs returns the jobs the daemon runs. When no daemon answers it returns none,
// unless the index still holds jobs, which a daemon would run again: then whether
// one is up cannot be told, and it fails with run.ErrUnknown.
func (d Daemon) Jobs(ctx context.Context) ([]run.Job, error) {
	ctx, cancel := context.WithTimeout(ctx, askTimeout)
	defer cancel()

	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", d.SocketPath)
	if err != nil {
		return nil, d.unanswered()
	}
	defer conn.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	if err := json.NewEncoder(conn).Encode(request{Action: actionList}); err != nil {
		return nil, err
	}
	var answer response
	if err := json.NewDecoder(conn).Decode(&answer); err != nil {
		return nil, err
	}
	if answer.Status == statusError {
		return nil, errors.New("run daemon: " + answer.Message)
	}

	jobs := make([]run.Job, 0, len(answer.Jobs))
	for _, job := range answer.Jobs {
		jobs = append(jobs, run.Job{Name: job.Name, WorkDir: job.WorkDir, Up: up(job.Status)})
	}

	return jobs, nil
}

// unanswered is why no daemon answering tells nothing: nil when its index holds no job, run.ErrUnknown when it holds some.
func (d Daemon) unanswered() error {
	data, err := os.ReadFile(filepath.Join(filepath.Dir(d.SocketPath), indexName))
	if err != nil {
		return nil
	}

	// An index that cannot be read is an empty one, as the daemon itself reads it.
	var index struct {
		Jobs []json.RawMessage `json:"jobs"`
	}
	if json.Unmarshal(data, &index) != nil || len(index.Jobs) == 0 {
		return nil
	}

	return run.ErrUnknown
}

// up reports whether a job of status is up: running in the foreground, detached, or joined by this worktree (attached, as older daemons say).
func up(status string) bool {
	switch status {
	case "running", "detached", "joined", "attached":
		return true
	}

	return false
}

type request struct {
	Action string `json:"action"`
}

type response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
	Jobs    []struct {
		Name    string `json:"name"`
		Status  string `json:"status"`
		WorkDir string `json:"work_dir"`
	} `json:"jobs"`
}
