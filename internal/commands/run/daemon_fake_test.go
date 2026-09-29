package run

import (
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/service/process"
)

// attachSettle is how long the fake daemon holds a job's output back after
// accepting an attach; see serve.
// fakeDaemon answers on the socket process.SocketPath resolves to, with the
// config dir redirected under a short temp home: a command under test dials it
// exactly as it dials the real daemon, and never forks one.
type fakeDaemon struct {
	// Answers maps a job name to the responses ActionStart replies with, in
	// order. A job with no script is started without a word.
	Answers map[string][]process.Response
	// Jobs is what ActionList reports.
	Jobs []domain.JobInfo
	// StopErrors maps a job name to the message ActionStop refuses it with.
	StopErrors map[string]string
	// Streams maps a job name to the raw bytes ActionAttach writes on the
	// accepted connection before hanging up.
	Streams map[string][]byte
	// Version is what the daemon stamps its answers with. Empty means this
	// build's, so only a test about the version handshake sets it; the literal
	// "none" answers unstamped, like a daemon predating the handshake.
	Version string

	mu       sync.Mutex
	requests []process.Request
}

// shortHome points the config directory — and so the daemon socket under it —
// at /tmp rather than t.TempDir(), whose name overruns sun_path on macOS (104
// bytes) and fails the bind with EINVAL.
func shortHome(t *testing.T) {
	t.Helper()

	// Idempotent: a test that both starts a daemon and sets up a project calls
	// this twice, and a second home would move the socket out from under the
	// daemon already listening on the first.
	if strings.HasPrefix(os.Getenv("HOME"), "/tmp/wtmhome") {
		return
	}

	home, err := os.MkdirTemp("/tmp", "wtmhome")
	if err != nil {
		t.Fatalf("temp home: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(home) })
	t.Setenv("HOME", home)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
}

// startFakeDaemon binds the daemon socket for the length of the test.
func startFakeDaemon(t *testing.T, daemon *fakeDaemon) *fakeDaemon {
	t.Helper()

	shortHome(t)

	socket := process.SocketPath()
	if err := os.MkdirAll(filepath.Dir(socket), 0o755); err != nil {
		t.Fatalf("socket dir: %v", err)
	}

	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatalf("listen on %s: %v", socket, err)
	}
	t.Cleanup(func() { listener.Close() })

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go daemon.serve(conn)
		}
	}()

	return daemon
}

// stampedEncoder mirrors the real daemon's replyEncoder: every answer carries a
// version, because a client refuses to talk to a daemon whose build it cannot
// match. A test wanting that refusal sets fakeDaemon.Version.
type stampedEncoder struct {
	enc     *json.Encoder
	version string
}

func (e stampedEncoder) Encode(resp process.Response) error {
	resp.Version = e.version
	return e.enc.Encode(resp)
}

// version defaults to this build's, which is what every test that is not about
// the handshake wants.
func (d *fakeDaemon) version() string {
	if d.Version == "none" {
		return ""
	}
	if d.Version != "" {
		return d.Version
	}
	return domain.Version
}

func (d *fakeDaemon) serve(conn net.Conn) {
	defer conn.Close()

	var req process.Request
	if err := json.NewDecoder(conn).Decode(&req); err != nil {
		return
	}

	d.mu.Lock()
	d.requests = append(d.requests, req)
	d.mu.Unlock()

	encoder := stampedEncoder{enc: json.NewEncoder(conn), version: d.version()}
	if req.Action == process.ActionList {
		d.mu.Lock()
		jobs := d.Jobs
		d.mu.Unlock()
		_ = encoder.Encode(process.Response{Status: process.StatusOK, Jobs: jobs})
		return
	}
	if req.Action == process.ActionStopAll {
		d.mu.Lock()
		var stopped []domain.JobInfo
		for _, job := range d.Jobs {
			if req.WorkDir == "" || job.WorkDir == req.WorkDir {
				stopped = append(stopped, job)
			}
		}
		d.mu.Unlock()
		_ = encoder.Encode(process.Response{Status: process.StatusOK, Jobs: stopped})
		return
	}
	if req.Action == process.ActionStop {
		if message, refused := d.StopErrors[req.Name]; refused {
			_ = encoder.Encode(process.Response{Status: process.StatusError, Message: message})
			return
		}
		_ = encoder.Encode(process.Response{Status: process.StatusOK})
		return
	}
	if req.Action == process.ActionAttach {
		_ = encoder.Encode(process.Response{Status: process.StatusOK})
		_, _ = conn.Write(d.Streams[req.Name])
		return
	}
	if req.Action != process.ActionStart || req.Job == nil {
		_ = encoder.Encode(process.Response{Status: process.StatusOK})
		return
	}

	scripted, ok := d.Answers[req.Job.Name]
	if !ok {
		_ = encoder.Encode(process.Response{Status: process.StatusOK})
		return
	}
	for _, resp := range scripted {
		if err := encoder.Encode(resp); err != nil {
			return
		}
	}
}

// setJobs changes what ActionList reports once the daemon is serving.
func (d *fakeDaemon) setJobs(jobs []domain.JobInfo) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.Jobs = jobs
}

// startedJobs names the jobs the commands asked the daemon to start, in order.
func (d *fakeDaemon) startedJobs() []string {
	d.mu.Lock()
	defer d.mu.Unlock()

	names := make([]string, 0, len(d.requests))
	for _, req := range d.requests {
		if req.Action == process.ActionStart && req.Job != nil {
			names = append(names, req.Job.Name)
		}
	}
	return names
}
