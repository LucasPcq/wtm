package daemon

import (
	"bufio"
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/next/internal/adapter/statefile"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

const (
	listTimeout  = 2 * time.Second
	stopTimeout  = time.Minute
	shutdownWait = 30 * time.Second
	pollEvery    = 50 * time.Millisecond

	indexName = "jobs.json"
	lockName  = "wtm.lock"
	logName   = "daemon.log"
)

// startWait is how long a daemon started has to answer, and a process on the socket that is no run daemon to go.
var startWait = 5 * time.Second

// errNoDaemon is the error of a request no daemon answered.
var errNoDaemon = errors.New("no run daemon answers")

// Daemon is the run daemon listening on SocketPath, as a client asks it:
// Version is this build's, which a daemon of another is told apart by, and
// StartDaemon starts a daemon of this build when one is needed (nil starts
// none). Its index of jobs, its lock and its log lie beside the socket.
type Daemon struct {
	SocketPath  string
	Version     string
	StartDaemon func() error
}

// LogPath returns the file a daemon started beside socket writes why it
// stopped, its standard error.
func LogPath(socket string) string {
	return filepath.Join(filepath.Dir(socket), logName)
}

// IndexPath returns the index of jobs of the daemon listening on socket.
func IndexPath(socket string) string {
	return filepath.Join(filepath.Dir(socket), indexName)
}

// ----------------------------------------------------------------------------
// Reads

// Jobs returns the jobs the daemon knows, never starting it; a daemon of the
// legacy wtm is read as it spells them. When none answers it reads its index:
// detached stacks and claims run without a daemon, while a foreground service
// left in it means a daemon died, and whether it is up cannot be told: it then
// fails with run.ErrUnknown.
func (d Daemon) Jobs(ctx context.Context) ([]run.Job, error) {
	listing, err := d.exchange(ctx, exchangeParams{Request: request{Action: actionList}, Timeout: listTimeout})
	if errors.Is(err, errNoDaemon) {
		return d.indexed()
	}
	if err != nil {
		return nil, err
	}
	if err := listing.failure(); err != nil {
		return nil, err
	}
	skew := run.SkewOf(listing.stamp(), d.Version)
	if skew == run.SkewProtocol {
		return nil, &run.Error{Code: run.CodeSkew, Message: fmt.Sprintf("the run daemon speaks protocol %d, this wtm %d: restart it (wtm run daemon restart)", listing.Protocol, run.ProtocolVersion)}
	}

	jobs := make([]run.Job, 0, len(listing.Jobs))
	for _, j := range listing.Jobs {
		jobs = append(jobs, j.runJob(skew == run.SkewLegacy))
	}

	return jobs, nil
}

// indexed returns the jobs the index says are up with no daemon to ask.
func (d Daemon) indexed() ([]run.Job, error) {
	records, _ := Index{Path: IndexPath(d.SocketPath)}.Load()
	var jobs []run.Job
	for _, record := range records {
		spec := record.Spec
		if !record.Claim && !spec.Detached() {
			return nil, run.ErrUnknown
		}
		job := run.Job{Name: spec.Job.Name, WorkDir: spec.WorkDir, Up: true, Kind: spec.Job.Kind, Claim: record.Claim, Detached: !record.Claim, SharedDir: record.SharedDir}
		if !record.Claim {
			job.State, job.StartedAt = run.Running, record.StartedAt
		}
		jobs = append(jobs, job)
	}

	return jobs, nil
}

// ----------------------------------------------------------------------------
// Start and stop

// Start starts the job spec asks for, handing out each line it prints while
// the start runs (a task until it ends, a launcher until it exits), and
// returns it as the daemon then lists it; a daemon of this build is made sure
// of first ([Daemon.Ensure]). It fails with the daemon's [run.Error].
func (d Daemon) Start(ctx context.Context, spec run.Spec, out func(run.Line)) (run.Job, error) {
	if err := d.Ensure(ctx); err != nil {
		return run.Job{}, err
	}

	started, err := d.exchange(ctx, exchangeParams{Request: request{Action: actionStart, Spec: specOf(spec)}, Out: out})
	if err != nil {
		return run.Job{}, err
	}
	job := run.Job{}
	if started.Job != nil {
		job = started.Job.runJob(false)
	}

	return job, started.failure()
}

// Stop stops the job req names. A daemon of another build or of the legacy
// wtm is asked all the same: stopping what it runs is what a restart needs.
// With no daemon answering, one is started only when the index holds a job of
// that worktree, a detached stack outliving its daemon; otherwise nothing was
// running.
func (d Daemon) Stop(ctx context.Context, req run.Stop) (run.Outcome, error) {
	stopped, err := d.reach(ctx, reachParams{WorkDir: req.WorkDir, Request: request{Action: actionStop, Name: req.Name, WorkDir: req.WorkDir, CorrelationID: req.CorrelationID}})
	if errors.Is(err, errNoDaemon) {
		return run.OutcomeNotRunning, nil
	}
	if err != nil {
		return "", err
	}
	if err := stopped.failure(); err != nil {
		return "", err
	}

	// The legacy daemon answers no outcome: it says stopped whatever it did.
	return cmp.Or(run.Outcome(stopped.Outcome), run.OutcomeStopped), nil
}

// StopAll stops every job up in req's worktree, every job of the machine when
// it names none, and returns each as it stood with what the stop did; it
// reaches a daemon as [Daemon.Stop] does.
func (d Daemon) StopAll(ctx context.Context, req run.StopAll) ([]run.StoppedJob, error) {
	stopped, err := d.reach(ctx, reachParams{WorkDir: req.WorkDir, Request: request{Action: actionStopAll, WorkDir: req.WorkDir, CorrelationID: req.CorrelationID}})
	if errors.Is(err, errNoDaemon) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	legacy := stopped.Protocol == 0
	jobs := make([]run.StoppedJob, 0, len(stopped.Jobs))
	for _, j := range stopped.Jobs {
		jobs = append(jobs, run.StoppedJob{Job: j.runJob(legacy), Outcome: cmp.Or(run.Outcome(j.Outcome), run.OutcomeStopped)})
	}

	return jobs, stopped.failure()
}

// reachParams is a stop request, and the worktree whose jobs in the index
// make it worth starting a daemon for, every worktree's when empty.
type reachParams struct {
	WorkDir string
	Request request
}

// reach sends a stop request to the daemon. With none answering it starts one
// when the index holds a job of the worktree, and asks it; else it fails with
// errNoDaemon: nothing is up.
func (d Daemon) reach(ctx context.Context, params reachParams) (answer, error) {
	exchange := exchangeParams{Request: params.Request, Timeout: stopTimeout}
	answered, err := d.exchange(ctx, exchange)
	if !errors.Is(err, errNoDaemon) {
		return answered, err
	}
	records, _ := Index{Path: IndexPath(d.SocketPath)}.Load()
	if !slices.ContainsFunc(records, func(record run.Record) bool { return params.WorkDir == "" || record.Spec.WorkDir == params.WorkDir }) {
		return answer{}, err
	}
	if err := d.launch(ctx); err != nil {
		return answer{}, err
	}

	return d.exchange(ctx, exchange)
}

// ----------------------------------------------------------------------------
// The daemon itself

// Ensure makes sure a daemon of this build answers, as a command about to start
// a job needs: none answering, it starts one; one of another build or of the
// legacy wtm holding no job up is stopped and replaced; one holding jobs is
// left alone, refused with [run.CodeSkew] or [run.CodeLegacy], since stopping
// it would stop the services someone works in. A daemon is never signalled:
// it is asked to stop, or named. A process on the socket that is no run daemon
// (an older wtm-next's event bus, which stops once idle) is waited for a
// moment, then refused with [run.CodeBusy].
func (d Daemon) Ensure(ctx context.Context) error {
	listing, err := d.exchange(ctx, exchangeParams{Request: request{Action: actionList}, Timeout: listTimeout})
	if errors.Is(err, errNoDaemon) {
		return d.launch(ctx)
	}
	if err != nil {
		return err
	}
	if listing.Status == statusError && listing.Protocol == 0 {
		return d.afterBus(ctx)
	}

	skew := run.SkewOf(listing.stamp(), d.Version)
	up := 0
	for _, j := range listing.Jobs {
		if j.runJob(skew == run.SkewLegacy).Up {
			up++
		}
	}
	if skew == run.SkewNone {
		return nil
	}
	if !run.Replaceable(skew, up) {
		return skewed(skewedParams{Skew: skew, Daemon: listing.Version, Client: d.Version, Up: up})
	}
	if err := d.Shutdown(ctx); err != nil {
		return err
	}

	return d.launch(ctx)
}

// afterBus waits for a process on the socket that is no run daemon to stop,
// then starts one.
func (d Daemon) afterBus(ctx context.Context) error {
	if err := d.poll(ctx, pollParams{Within: startWait, Done: d.unlocked}); err != nil {
		return &run.Error{Code: run.CodeBusy, Message: "a process that is no run daemon holds its socket " + d.SocketPath + " (an older wtm-next's event bus): it stops once idle, try again in a minute"}
	}

	return d.launch(ctx)
}

// skewedParams is a daemon refused for its skew: the builds of the daemon and
// of the client, and the jobs it holds up.
type skewedParams struct {
	Skew   run.Skew
	Daemon string
	Client string
	Up     int
}

func skewed(params skewedParams) error {
	code, daemon := run.CodeSkew, "wtm "+cmp.Or(params.Daemon, "an older build")
	if params.Skew == run.SkewLegacy {
		code, daemon = run.CodeLegacy, "the legacy wtm "+cmp.Or(params.Daemon, "of an older build")
	}

	return &run.Error{Code: code, Message: fmt.Sprintf("the run daemon is %s, this is wtm %s, and it runs %d job(s): restart it to hand them over to this wtm (wtm run daemon restart)", daemon, params.Client, params.Up)}
}

// Shutdown asks the daemon to stop and returns once it is gone: its foreground
// services stopped, its detached stacks and claims left to the next. None
// answering is no error.
func (d Daemon) Shutdown(ctx context.Context) error {
	stopping, err := d.exchange(ctx, exchangeParams{Request: request{Action: actionShutdown}, Timeout: listTimeout})
	if errors.Is(err, errNoDaemon) {
		return nil
	}
	if err != nil {
		return err
	}
	if err := stopping.failure(); err != nil {
		return err
	}

	if err := d.poll(ctx, pollParams{Within: shutdownWait, Done: d.unlocked}); err != nil {
		return &run.Error{Code: run.CodeStartFailed, Message: fmt.Sprintf("the run daemon did not stop within %v", shutdownWait)}
	}

	return nil
}

// launch starts a daemon and waits for it to answer. A socket path past what
// the system allows is refused before anything starts; a daemon that does not
// answer in time is reported with the last line it wrote to its log.
func (d Daemon) launch(ctx context.Context) error {
	if limit := maxSocketPath(); len(d.SocketPath) > limit {
		return &run.Error{Code: run.CodeSocketTooLong, Message: fmt.Sprintf("the run daemon's socket %s is %d bytes long, past the %d this system allows: move the user config directory to a shorter path", d.SocketPath, len(d.SocketPath), limit)}
	}
	if d.StartDaemon == nil {
		return errNoDaemon
	}
	if err := d.StartDaemon(); err != nil {
		return &run.Error{Code: run.CodeStartFailed, Message: "start the run daemon: " + err.Error()}
	}

	if err := d.poll(ctx, pollParams{Within: startWait, Done: func() bool { return d.answers(ctx) }}); err != nil {
		message := fmt.Sprintf("the run daemon did not start within %v", startWait)
		if why := lastLine(LogPath(d.SocketPath)); why != "" {
			message += ": " + why
		}
		return &run.Error{Code: run.CodeStartFailed, Message: message}
	}

	return nil
}

// maxSocketPath is the longest path a unix socket may have here: sun_path
// holds 104 bytes on macOS and the BSDs, 108 on Linux, its last a NUL.
func maxSocketPath() int {
	if runtime.GOOS == "linux" {
		return 107
	}

	return 103
}

// lastLine returns the last line of the file at path, "" when it has none.
func lastLine(path string) string {
	file, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer file.Close()
	last := ""
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		if line := strings.TrimSpace(scanner.Text()); line != "" {
			last = line
		}
	}

	return last
}

// answers reports whether a daemon answers a listing.
func (d Daemon) answers(ctx context.Context) bool {
	_, err := d.exchange(ctx, exchangeParams{Request: request{Action: actionList}, Timeout: listTimeout})
	return err == nil
}

// unlocked reports whether no daemon holds the lock beside the socket: the
// kernel releases it when its process exits, whatever the exit.
func (d Daemon) unlocked() bool {
	lock, err := statefile.TryLock(filepath.Join(filepath.Dir(d.SocketPath), lockName), domain.LockExclusive)
	if err != nil {
		return false
	}
	_ = lock.Close()

	return true
}

// pollParams is how long to wait, and what ends the wait.
type pollParams struct {
	Within time.Duration
	Done   func() bool
}

// poll waits until params.Done, checking every 50 ms; it fails once
// params.Within passed, or with ctx's error.
func (d Daemon) poll(ctx context.Context, params pollParams) error {
	deadline := time.Now().Add(params.Within)
	for !params.Done() {
		if time.Now().After(deadline) {
			return context.DeadlineExceeded
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(pollEvery):
		}
	}

	return nil
}

// ----------------------------------------------------------------------------
// The exchange

// exchangeParams is a request, how long its whole exchange may take (no limit
// when zero), and where the lines its answer sends first go (nil drops them).
type exchangeParams struct {
	Request request
	Timeout time.Duration
	Out     func(run.Line)
}

// exchange sends a request and returns the answer that ends it; it fails
// with errNoDaemon when nothing listens on the socket, and stops when ctx is
// done.
func (d Daemon) exchange(ctx context.Context, params exchangeParams) (answer, error) {
	if params.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, params.Timeout)
		defer cancel()
	}
	conn, err := (&net.Dialer{}).DialContext(ctx, "unix", d.SocketPath)
	if err != nil {
		return answer{}, fmt.Errorf("%w: %w", errNoDaemon, err)
	}
	defer conn.Close()
	defer context.AfterFunc(ctx, func() { _ = conn.Close() })()

	if err := json.NewEncoder(conn).Encode(params.Request); err != nil {
		return answer{}, fmt.Errorf("ask the run daemon: %w", err)
	}
	decoder := json.NewDecoder(conn)
	for {
		var a answer
		if err := decoder.Decode(&a); err != nil {
			return answer{}, fmt.Errorf("read the run daemon's answer: %w", cmp.Or(ctx.Err(), err))
		}
		if a.Status != statusLine {
			return a, nil
		}
		if params.Out != nil {
			params.Out(run.Line{At: a.At, Text: a.Text})
		}
	}
}
