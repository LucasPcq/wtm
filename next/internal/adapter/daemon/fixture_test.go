package daemon_test

import (
	"context"
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
	proxy    run.Proxy
	lines    []string
	err      error
	outcome  run.Outcome
	requests []any
	returned chan struct{}
	// lateAfter is how long after the client typed the job prints once more, none when zero.
	lateAfter time.Duration
	typed     typed
}

// typed keeps what a client typed into a job's terminal, safe to read while it is written, and closes done at the first key.
type typed struct {
	mu   sync.Mutex
	got  []byte
	done chan struct{}
}

func (t *typed) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.got) == 0 && t.done != nil {
		close(t.done)
	}
	t.got = append(t.got, p...)
	return len(p), nil
}

func (t *typed) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.got)
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

func (j *jobs) Proxy() run.Proxy {
	return j.proxy
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

func (j *jobs) Lines(ctx context.Context, req run.Tail, out func(run.Line)) (run.Job, error) {
	j.record(req)
	for _, line := range j.lines {
		out(run.Line{At: started, Text: line})
	}
	if req.Follow {
		<-ctx.Done()
		return run.Job{}, ctx.Err()
	}
	return web, j.err
}

func (j *jobs) Attach(req run.Window) (run.Session, error) {
	j.record(req)
	if j.err != nil {
		return run.Session{}, j.err
	}
	stream := make(chan []byte, 2)
	stream <- []byte("live")
	// The job ends once the client typed into it: a session that ended first would hang up on its keys.
	j.typed.done = make(chan struct{})
	go func() {
		<-j.typed.done
		if j.lateAfter > 0 {
			time.Sleep(j.lateAfter)
			stream <- []byte("|late")
		}
		close(stream)
	}()
	return run.Session{History: []byte("history|"), Stream: stream, Input: &j.typed, Release: func() {}}, nil
}

func (j *jobs) Resize(req run.Window) error {
	j.record(req)
	return j.err
}

func (j *jobs) seen() []any {
	j.mu.Lock()
	defer j.mu.Unlock()
	return append([]any(nil), j.requests...)
}
