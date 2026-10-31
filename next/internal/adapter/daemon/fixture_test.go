package daemon_test

import (
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// started is when the jobs of a test started.
var started = time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)

// web is a foreground service up, as the run daemon lists it.
var web = run.Job{Name: "web", WorkDir: "/wt/a", Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn, PID: 4242, URL: "http://localhost:3100", Ports: map[string]int{"PORT": 3100}, StartedAt: started}

// jobs is a run daemon a test scripts: what it lists, prints, refuses, and every request it got.
type jobs struct {
	mu       sync.Mutex
	listed   []run.Job
	lines    []string
	err      error
	outcome  run.Outcome
	requests []any
	returned chan struct{}
}

func (j *jobs) record(req any) {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.requests = append(j.requests, req)
}

func (j *jobs) List(workDir string) []run.Job {
	j.record("list " + workDir)
	return j.listed
}

func (j *jobs) Start(spec run.Spec, out func(run.Line)) (run.Job, error) {
	j.record(spec)
	for _, line := range j.lines {
		out(run.Line{At: started, Text: line})
	}
	if j.returned != nil {
		close(j.returned)
	}
	return web, j.err
}

func (j *jobs) Stop(req run.Stop) (run.Outcome, error) {
	j.record(req)
	return j.outcome, j.err
}

func (j *jobs) StopAll(req run.StopAll) ([]run.StoppedJob, error) {
	j.record(req)
	return []run.StoppedJob{{Job: web, Outcome: run.OutcomeStopped}}, j.err
}

func (j *jobs) seen() []any {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]any(nil), j.requests...)
}
