// Package processtest stands a fake run daemon on the socket a flow dials, so
// a test can decide what is running without starting a single process.
package processtest

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

type Daemon struct {
	mu   sync.Mutex
	jobs []domain.JobInfo
	// StopError is what every stop is refused with, empty to accept them.
	StopError string
	// Survive leaves the jobs up after a stop the daemon said it made.
	Survive  bool
	requests []process.Request
}

// Serve moves HOME to a directory short enough for the socket — macOS puts it
// under Library/Application Support — and answers there with jobs.
func Serve(t *testing.T, jobs []domain.JobInfo) *Daemon {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "wtm")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	t.Setenv("HOME", home)
	socket := process.SocketPath()
	if err := os.MkdirAll(filepath.Dir(socket), 0o755); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { listener.Close() })

	daemon := &Daemon{jobs: jobs}
	go daemon.serve(listener)
	return daemon
}

func (d *Daemon) serve(listener net.Listener) {
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		var req process.Request
		if json.NewDecoder(conn).Decode(&req) == nil {
			_ = json.NewEncoder(conn).Encode(d.answer(req))
		}
		conn.Close()
	}
}

func (d *Daemon) answer(req process.Request) process.Response {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.requests = append(d.requests, req)
	switch req.Action {
	case process.ActionList:
		return process.Response{Status: process.StatusOK, Version: domain.Version, Jobs: append([]domain.JobInfo{}, d.jobs...)}
	case process.ActionStop, process.ActionStopAll:
		if d.StopError != "" {
			return process.Response{Status: process.StatusError, Version: domain.Version, Message: d.StopError}
		}
		if !d.Survive {
			d.stop(req)
		}
	}
	return process.Response{Status: process.StatusOK, Version: domain.Version}
}

func (d *Daemon) stop(req process.Request) {
	kept := d.jobs[:0]
	for _, job := range d.jobs {
		matches := job.WorkDir == req.WorkDir && (req.Action == process.ActionStopAll || job.Name == req.Name)
		if !matches {
			kept = append(kept, job)
		}
	}
	d.jobs = kept
}

// Actions are the requests that changed something, as action:name@workdir.
func (d *Daemon) Actions() []string {
	d.mu.Lock()
	defer d.mu.Unlock()
	var actions []string
	for _, req := range d.requests {
		if req.Action == process.ActionList {
			continue
		}
		actions = append(actions, string(req.Action)+":"+req.Name+"@"+req.WorkDir)
	}
	return actions
}
