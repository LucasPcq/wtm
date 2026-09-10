package process

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/infra"
	"github.com/LucasPcq/wtm/internal/rules"
	"github.com/LucasPcq/wtm/internal/service/proxy"
)

var (
	daemonIdleTimeout = time.Duration(domain.DaemonIdleTimeoutSeconds) * time.Second
	// daemonNamespaceBudget is how long a shared job's attach may retry. It is a
	// variable for the same reason daemonIdleTimeout is: the two interact, and a
	// test wants both wound down together.
	daemonNamespaceBudget = domain.NamespaceCreateTimeout
)

// DaemonParams holds inputs for starting the daemon.
type DaemonParams struct {
	SocketPath string
	// ProxyPort is where the run proxy listens, resolved by the client from the
	// global config. Zero leaves the proxy off, and jobs keep their own ports.
	ProxyPort int
}

type daemonServer struct {
	manager    *Manager
	listener   net.Listener
	socketPath string
	// proxyPort is what the proxy actually bound, zero when it is off or the
	// port was taken. Every answer carries it so no client ever announces a
	// name nothing serves.
	proxyPort int
	clients   sync.WaitGroup
	// inflight counts the connections being served. The idle watcher reads it
	// beside the job count: a shared service launches detached and leaves
	// nothing Running, so the request carving out its namespace — seconds of
	// retries against a database that has just been started — would otherwise be
	// auto-exited under, and the caller would read a closed socket instead of
	// what the command said.
	inflight atomic.Int64
	shutdown chan struct{}
}

// RunDaemon starts the daemon, listens on the Unix socket, and blocks until shutdown.
func RunDaemon(params DaemonParams) error {
	if err := os.MkdirAll(filepath.Dir(params.SocketPath), 0o755); err != nil {
		return fmt.Errorf("create socket dir: %w", err)
	}

	// Remove stale socket
	os.Remove(params.SocketPath)

	listener, err := net.Listen("unix", params.SocketPath)
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	registry := proxy.NewRegistry()
	store := NewStateStore(StatePath())
	manager := NewManagerWith(ManagerParams{Routes: registry, Index: store, NamespaceBudget: daemonNamespaceBudget})
	d := &daemonServer{
		manager:    manager,
		listener:   listener,
		socketPath: params.SocketPath,
		shutdown:   make(chan struct{}),
	}

	if params.ProxyPort > 0 {
		server := proxy.NewServer(proxy.ServerParams{Port: params.ProxyPort, Registry: registry})
		if startErr := server.Start(); startErr != nil {
			// The whole window was taken, which costs the names but never the
			// jobs. Nobody reads a forked daemon's stderr, so the refusal
			// travels in every answer instead: d.proxyPort stays zero and the
			// clients say why.
			log.Printf(domain.ProxyBindFailedFmt, params.ProxyPort, startErr)
		} else {
			d.proxyPort = server.Port()
			defer server.Close()
		}
	}

	// Adopted after the proxy is up, so a detached job's name is served again
	// from the moment it is known rather than from the next request.
	manager.Adopt(store.Load())

	// Handle signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		<-sigCh
		d.stop()
	}()

	// Idle timer for auto-exit
	go d.idleWatcher()

	// Accept loop
	for {
		conn, acceptErr := listener.Accept()
		if acceptErr != nil {
			select {
			case <-d.shutdown:
				return nil
			default:
				continue
			}
		}
		d.clients.Add(1)
		d.inflight.Add(1)
		go func() {
			defer d.clients.Done()
			defer d.inflight.Add(-1)
			d.handleConnection(conn)
		}()
	}
}

// publicPort is probed per answer, never cached: an install or an uninstall
// happens while the daemon is alive, and a value frozen at startup would keep
// announcing a name nothing serves — or hide one that works.
func (d *daemonServer) publicPort() int {
	if d.proxyPort == 0 {
		return 0
	}
	return rules.PublicPort(rules.PublicPortParams{
		BindPort: d.proxyPort,
		Probed:   proxy.Probe(proxy.ProbeParams{Port: domain.ProxyPrivilegedPort}),
		DaemonUp: true,
	})
}

func (d *daemonServer) stop() {
	close(d.shutdown)
	d.listener.Close()
	d.manager.StopForeground()
	os.Remove(d.socketPath)
	d.clients.Wait()
}

func (d *daemonServer) idleWatcher() {
	ticker := time.NewTicker(daemonIdleTimeout)
	defer ticker.Stop()

	for {
		select {
		case <-d.shutdown:
			return
		case <-ticker.C:
			if !d.manager.IsRunning() && d.inflight.Load() == 0 {
				d.stop()
				return
			}
		}
	}
}

// replyEncoder stamps every answer with this daemon's build and PID. It is the
// only way out of a handler, so a client never receives an unstamped response
// and can read a missing version as "older than the field itself".
type replyEncoder struct {
	enc *json.Encoder
}

func (e replyEncoder) Encode(resp Response) error {
	resp.Version = domain.Version
	resp.DaemonPID = os.Getpid()
	return e.enc.Encode(resp)
}

func (d *daemonServer) handleConnection(conn net.Conn) {
	defer conn.Close()

	decoder := json.NewDecoder(conn)
	encoder := replyEncoder{enc: json.NewEncoder(conn)}

	var req Request
	if err := decoder.Decode(&req); err != nil {
		encoder.Encode(Response{Status: StatusError, Message: fmt.Sprintf("invalid request: %v", err)})
		return
	}

	switch req.Action {
	case ActionStart:
		d.handleStart(encoder, req)
	case ActionStop:
		d.handleStop(encoder, req)
	case ActionStopAll:
		d.handleStopAll(encoder, req)
	case ActionList:
		d.handleList(encoder, req)
	case ActionAttach:
		d.handleAttach(conn, encoder, req)
	case ActionResize:
		d.handleResize(encoder, req)
	case ActionShutdown:
		d.handleShutdown(encoder)
	default:
		encoder.Encode(Response{Status: StatusError, Message: fmt.Sprintf("unknown action: %s", req.Action)})
	}
}

func (d *daemonServer) handleStart(encoder replyEncoder, req Request) {
	if req.Job == nil {
		encoder.Encode(Response{Status: StatusError, Message: "job config required"})
		return
	}

	ports := jobPorts(*req.Job, req.Env)

	if req.Job.Kind == domain.JobKindTask {
		// Tasks block until the command exits and stream their output back over
		// the socket as StatusOutput chunks, so the CLI can render it live
		// (`run up` / `run start`). Start blocks until every chunk has been
		// flushed, so the terminal response below never races the stream.
		err := d.manager.Start(StartParams{
			Job:      *req.Job,
			WorkDir:  req.WorkDir,
			LogDir:   req.LogDir,
			Env:      req.Env,
			Routes:   req.Routes,
			Shared:   req.Shared,
			Streamer: responseStreamWriter{encoder: encoder},
		})
		if err != nil {
			code := exitCodeOf(err)
			encoder.Encode(Response{Status: StatusError, Message: err.Error(), ExitCode: &code})
			return
		}
		zero := 0
		encoder.Encode(Response{Status: StatusDone, Message: fmt.Sprintf("task %s done", req.Job.Name), ExitCode: &zero, Ports: ports, ProxyPort: d.proxyPort, ProxyPublicPort: d.publicPort()})
		return
	}

	// Detached launchers (e.g. docker compose up -d) run like a task for their
	// lifetime — they stream their startup output and then exit — but stay
	// registered as Running afterwards. Stream that output live so `run up`
	// shows the launcher's lines as they happen, then send the terminal "started".
	if rules.IsDetached(*req.Job) {
		if err := d.manager.Start(StartParams{
			Job:      *req.Job,
			WorkDir:  req.WorkDir,
			LogDir:   req.LogDir,
			Env:      req.Env,
			Routes:   req.Routes,
			Shared:   req.Shared,
			Streamer: responseStreamWriter{encoder: encoder},
		}); err != nil {
			encoder.Encode(Response{Status: StatusError, Message: err.Error()})
			return
		}
		encoder.Encode(Response{Status: StatusOK, Message: fmt.Sprintf("job %s started", req.Job.Name), Ports: ports, ProxyPort: d.proxyPort, ProxyPublicPort: d.publicPort()})
		return
	}

	if err := d.manager.Start(StartParams{Job: *req.Job, WorkDir: req.WorkDir, LogDir: req.LogDir, Env: req.Env, Routes: req.Routes, Shared: req.Shared}); err != nil {
		encoder.Encode(Response{Status: StatusError, Message: err.Error()})
		return
	}

	encoder.Encode(Response{Status: StatusOK, Message: fmt.Sprintf("job %s started", req.Job.Name), Ports: ports, ProxyPort: d.proxyPort, ProxyPublicPort: d.publicPort()})
}

// handleShutdown answers before it stops, since stopping closes the socket the
// answer travels on. It leaves detached jobs running: they are what the index
// hands to the next daemon, and tearing them down here would make every version
// mismatch cost the user their stack.
func (d *daemonServer) handleShutdown(encoder replyEncoder) {
	encoder.Encode(Response{Status: StatusOK, Message: "daemon stopping"})
	go d.stop()
}

func (d *daemonServer) handleStop(encoder replyEncoder, req Request) {
	if err := d.manager.Stop(req.Name, req.WorkDir); err != nil {
		encoder.Encode(Response{Status: StatusError, Message: err.Error()})
		return
	}

	encoder.Encode(Response{Status: StatusOK, Message: fmt.Sprintf("job %s stopped", req.Name)})
}

func (d *daemonServer) handleStopAll(encoder replyEncoder, req Request) {
	// Snapshot the jobs that are about to be stopped so the client can report
	// which ones (or say "none running" when the list is empty).
	var stopped []domain.JobInfo
	for _, job := range d.manager.List() {
		if !rules.IsJobUp(job.Status) {
			continue
		}
		if req.WorkDir != "" && job.WorkDir != req.WorkDir {
			continue
		}
		stopped = append(stopped, d.jobInfoOf(job))
	}

	var err error
	if req.WorkDir != "" {
		err = d.manager.StopAllInWorkDir(req.WorkDir)
	} else {
		err = d.manager.StopAll()
	}
	if err != nil {
		encoder.Encode(Response{Status: StatusError, Message: err.Error()})
		return
	}

	encoder.Encode(Response{Status: StatusOK, Jobs: stopped})
}

func (d *daemonServer) handleList(encoder replyEncoder, req Request) {
	jobs := d.manager.List()
	infos := make([]domain.JobInfo, 0, len(jobs))
	for _, job := range jobs {
		infos = append(infos, d.jobInfoOf(job))
	}

	encoder.Encode(Response{Status: StatusOK, Jobs: infos, ProxyPort: d.proxyPort, ProxyPublicPort: d.publicPort()})
}

func (d *daemonServer) jobInfoOf(job ManagedJob) domain.JobInfo {
	return domain.JobInfo{
		Name:    job.Name,
		Kind:    job.Config.Kind,
		WorkDir: job.WorkDir,
		Status:  job.Status,
		// A detached job reports no PID: the one it was spawned with belongs to
		// a launcher that has exited, and printing it points at nothing — or, in
		// time, at a stranger.
		PID:       detachedAwarePID(job),
		StartedAt: job.StartedAt,
		ExitCode:  job.ExitCode,
		URL: rules.JobURL(rules.JobURLParams{
			Job:        job.Config,
			Ports:      jobPorts(job.Config, job.Env),
			Host:       rules.JobOwnRoute(job.Routes, job.Name),
			PublicPort: d.publicPort(),
		}),
	}
}

// A claim, like a detached launcher, has no process of its own to report: the
// service it holds runs under the main checkout's key, and printing its PID
// beside three worktrees would read as three processes.
func detachedAwarePID(job ManagedJob) int {
	if job.Status == domain.JobStatusDetached || job.Status == domain.JobStatusAttached {
		return 0
	}
	return job.PID
}

// handleResize answers on its own connection by design: an attach connection
// carries raw PTY bytes as soon as it is accepted, so a size sent there would
// be typed into the job instead of resizing it.
func (d *daemonServer) handleResize(encoder replyEncoder, req Request) {
	err := d.manager.Resize(ResizeParams{
		Name:    req.Name,
		WorkDir: req.WorkDir,
		Cols:    req.Cols,
		Rows:    req.Rows,
	})
	if err != nil {
		encoder.Encode(Response{Status: StatusError, Message: err.Error()})
		return
	}

	encoder.Encode(Response{Status: StatusOK, Message: fmt.Sprintf("job %s resized", req.Name)})
}

func (d *daemonServer) handleAttach(conn net.Conn, encoder replyEncoder, req Request) {
	session, err := d.manager.Attach(req.Name, req.WorkDir)
	if err != nil {
		encoder.Encode(Response{Status: StatusError, Message: err.Error()})
		return
	}
	defer session.Release()

	if session.Writable && req.Cols > 0 && req.Rows > 0 {
		_ = setWinsize(winsizeParams{File: session.PTY, Cols: req.Cols, Rows: req.Rows})
	}

	// Send OK before switching to raw mode
	encoder.Encode(Response{Status: StatusOK, Message: "attached"})

	// Replay buffered history so the client sees the current TUI state
	// (progress lines, running frame, etc.) before live output resumes.
	if len(session.History) > 0 {
		conn.Write(session.History)
	}

	done := make(chan struct{}, 2)

	// Live job output → client. The PTY itself is drained by the hub goroutine
	// elsewhere; we just relay what the hub delivers to this subscriber.
	go func() {
		for data := range session.Stream {
			if _, err := conn.Write(data); err != nil {
				done <- struct{}{}
				return
			}
		}
		done <- struct{}{}
	}()

	// Client stdin → PTY, unarbitrated: see AttachSession on what two clients
	// typing at the same time get. A job with no PTY has no stdin to feed, and
	// the copy would fail at once and take the whole attach down with it.
	if session.Writable {
		go func() {
			io.Copy(session.PTY, conn)
			done <- struct{}{}
		}()
	}

	<-done
}

// responseStreamWriter adapts a job's output stream onto the daemon
// connection: each write is encoded as a StatusOutput response so the client
// can forward task output to the user in real time. It is only ever written to
// by the task's single streaming goroutine, so the encoder sees no concurrent
// use while the task runs.
type responseStreamWriter struct {
	encoder replyEncoder
}

func (w responseStreamWriter) Write(p []byte) (int, error) {
	if err := w.encoder.Encode(Response{Status: StatusOutput, Data: p}); err != nil {
		return 0, err
	}
	return len(p), nil
}

// SocketPath returns the default daemon socket path, empty on a machine that
// has no user-config directory at all — there is nowhere to put a socket, and
// every caller reads that as "no daemon".
func SocketPath() string {
	dir, err := infra.GlobalDir()
	if err != nil {
		return ""
	}
	return filepath.Join(dir, domain.DaemonSocketName)
}
