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
	Survive bool
	// StartError is what every start is refused with, empty to accept them.
	StartError string
	// Release answers every stop as a shared job let go of, still up elsewhere.
	Release bool
	// ProxyPublicPort is where the daemon says its proxy answers, zero for off.
	ProxyPublicPort int
	// Version is the build the daemon stamps its answers with, empty for this
	// one's: a test about a version mismatch sets it.
	Version  string
	requests []process.Request
}

// Serve moves HOME to a directory short enough for the socket — macOS puts it
// under Library/Application Support — and answers there with jobs. Linux reads
// XDG_CONFIG_HOME first, which CI runners set, so it moves too.
func Serve(t *testing.T, jobs []domain.JobInfo) *Daemon {
	t.Helper()
	home, err := os.MkdirTemp("/tmp", "wtm")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
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
		// A shutdown is the one request that ends the daemon: the socket stops
		// answering, which is what a client waits for.
		if req.Action == process.ActionShutdown {
			listener.Close()
			return
		}
	}
}

func (d *Daemon) answer(req process.Request) process.Response {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.requests = append(d.requests, req)
	resp := d.respond(req)
	resp.Version = domain.Version
	if d.Version != "" {
		resp.Version = d.Version
	}
	return resp
}

func (d *Daemon) respond(req process.Request) process.Response {
	switch req.Action {
	case process.ActionList:
		return process.Response{Status: process.StatusOK, Jobs: append([]domain.JobInfo{}, d.jobs...), ProxyPublicPort: d.ProxyPublicPort}
	case process.ActionStart:
		if d.StartError != "" {
			return process.Response{Status: process.StatusError, Message: d.StartError}
		}
	case process.ActionStop, process.ActionStopAll:
		if d.StopError != "" {
			return process.Response{Status: process.StatusError, Message: d.StopError}
		}
		resp := process.Response{Status: process.StatusOK, Released: d.Release}
		if req.Action == process.ActionStopAll {
			resp.Jobs = d.in(req.WorkDir)
		}
		if !d.Survive {
			d.stop(req)
		}
		return resp
	}
	return process.Response{Status: process.StatusOK}
}

// in is what a stop_all answers with, as the real daemon does: the jobs it
// found up there.
func (d *Daemon) in(workDir string) []domain.JobInfo {
	var jobs []domain.JobInfo
	for _, job := range d.jobs {
		if job.WorkDir == workDir {
			jobs = append(jobs, job)
		}
	}
	return jobs
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
