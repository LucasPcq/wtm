package runview

import (
	"context"
	"errors"
	"net"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

const (
	mainDir  = "/repo/main"
	featADir = "/repo/wt-feat-a"
	featBDir = "/repo/wt-feat-b"
)

var (
	now     = time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
	started = now.Add(-95 * time.Second)
	exit3   = 3
)

// board is feat/a and feat/b after `run up --profile full`: feat/b holds the
// shared db main runs, its task done, api up, web never started; feat/a runs
// its seed task and api.
func board() ([]Worktree, []run.Job) {
	worktrees := []Worktree{
		{Branch: "feat/b", Path: featBDir, Jobs: []string{"migrate", "db", "api", "web"}},
		{Branch: "feat/a", Path: featADir, Jobs: []string{"seed", "api"}},
	}
	jobs := []run.Job{
		{Name: "db", WorkDir: mainDir, Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn, SharedDir: mainDir, StartedAt: started, Ports: map[string]int{"PGPORT": 26653}},
		{Name: "db", WorkDir: featBDir, Up: true, Kind: project.Service, Hold: run.HoldClaim, SharedDir: mainDir, StartedAt: started, Ports: map[string]int{"PGPORT": 26653}},
		{Name: "migrate", WorkDir: featBDir, Kind: project.Task, State: run.Exited, Hold: run.HoldOwn, ExitCode: new(int)},
		{Name: "api", WorkDir: featBDir, Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn, StartedAt: started, URL: "http://api.feat-b.acme.localhost:11080"},
		{Name: "seed", WorkDir: featADir, Up: true, Kind: project.Task, State: run.Running, Hold: run.HoldOwn, StartedAt: started},
		{Name: "api", WorkDir: featADir, Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn, StartedAt: started, URL: "http://api.feat-a.acme.localhost:11080"},
		{Name: "api", WorkDir: "/elsewhere/repo", Up: true, Kind: project.Service, State: run.Running, Hold: run.HoldOwn},
	}
	return worktrees, jobs
}

// A daemon lists its jobs, attaches through attach (refused when nil),
// reads each job's log from logs, and records what it was asked.
type daemon struct {
	mu       sync.Mutex
	jobs     []run.Job
	attach   func(run.Window) (net.Conn, error)
	logs     map[string][]string
	attached []run.Window
	resized  []run.Window
}

var errNoTerminal = errors.New("attach refused")

func (d *daemon) Jobs(context.Context) ([]run.Job, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return slices.Clone(d.jobs), nil
}

func (d *daemon) Attach(_ context.Context, req run.Window) (net.Conn, error) {
	d.mu.Lock()
	d.attached = append(d.attached, req)
	attach := d.attach
	d.mu.Unlock()
	if attach == nil {
		return nil, errNoTerminal
	}
	return attach(req)
}

func (d *daemon) Resize(_ context.Context, req run.Window) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.resized = append(d.resized, req)
	return nil
}

func (d *daemon) Lines(_ context.Context, req run.Tail, out func(run.Line)) (run.Job, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	lines, found := d.logs[req.Name]
	if !found {
		return run.Job{}, &run.Error{Code: run.CodeJobNotFound, Message: "no such job"}
	}
	for _, text := range lines {
		out(run.Line{At: now, Text: text})
	}
	return run.Job{}, nil
}

func (d *daemon) set(jobs []run.Job) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.jobs = jobs
}

func (d *daemon) attaches() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.attached)
}

// A terminal is the job's side of an attach: what the view typed, and what
// the job prints.
type terminal struct {
	job   net.Conn
	typed chan []byte
}

// read is what the view typed next, failing after a second.
func (t *terminal) read(tb testing.TB) string {
	tb.Helper()
	select {
	case typed := <-t.typed:
		return string(typed)
	case <-time.After(time.Second):
		tb.Fatal("nothing was typed into the job")
		return ""
	}
}

// piped attaches every job to a terminal of its own, the last one kept.
func piped(d *daemon) *terminal {
	t := &terminal{typed: make(chan []byte, 16)}
	d.attach = func(run.Window) (net.Conn, error) {
		view, job := net.Pipe()
		t.job = job
		go func() {
			buf := make([]byte, 64)
			for {
				n, err := job.Read(buf)
				if err != nil {
					return
				}
				t.typed <- slices.Clone(buf[:n])
			}
		}()
		return view, nil
	}
	return t
}
