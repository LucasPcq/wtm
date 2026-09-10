// Package process manages long-running jobs with pseudo-terminals.
package process

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"

	"github.com/LucasPcq/wtm/internal/domain"
	"github.com/LucasPcq/wtm/internal/rules"
)

// detachedOutputBufferSize bounds how much PTY output we keep around while
// waiting for a detached-service launcher (e.g. `docker compose up -d`) to
// exit. Enough for the typical Docker/compose error line.
const detachedOutputBufferSize = 8 * 1024

// outputHistoryBytes is the size of the per-job rolling output buffer that the
// daemon drains from each long-running PTY. Replayed to clients on attach, so it
// has to hold enough raw bytes for a terminal emulator to rebuild the screen —
// escape sequences included — not just the last few visible lines.
const outputHistoryBytes = 1024 * 1024

// outputSubscriberQueue is the per-subscriber chunk queue. Deep enough to
// absorb a burst from a chatty job while a subscriber is busy rendering.
const outputSubscriberQueue = 256

// defaultPTYRows and defaultPTYCols are the fallback PTY dimensions used when
// a job is spawned before any client has attached. TUI apps read the PTY size
// at startup — a 0x0 window makes them bail to plain log mode.
const (
	defaultPTYRows = 40
	defaultPTYCols = 120
)

// stopGracePeriod is how long Stop waits for a process group to exit on
// SIGTERM before escalating to SIGKILL. Long enough for well-behaved dev
// servers (next, vite, turbo) to flush and shut down their children, short
// enough that the user doesn't notice a hang.
const stopGracePeriod = 5 * time.Second

// drainGracePeriod bounds the wait for a stopped job's last bytes to reach its
// log. It is short because the process is already reaped: what is left is a
// read of what the PTY still holds.
const drainGracePeriod = time.Second

// detachedDrainGracePeriod bounds how long the PTY drain goroutine is awaited
// to reach natural EOF after the process exits, before force-closing the master
// to unblock it. It backstops both waitDetached (detached launchers) and runTask
// (foreground tasks). The happy path hits EOF within milliseconds; this only
// bites if a descendant keeps the slave open, so the daemon can never hang on a
// misbehaving launcher.
const detachedDrainGracePeriod = 2 * time.Second

type ManagedJob struct {
	Name   string
	Config domain.JobConfig
	Cmd    *exec.Cmd
	// PTY, unlike the two fields below, lives outside the manager lock: it is
	// closed by whichever goroutine reaps or stops the job, holding nothing. A
	// user of it must therefore hold a reference on the descriptor itself
	// (setWinsize) rather than read its number out.
	PTY    *os.File
	Status domain.JobStatus
	PID    int
	// PGID is the group every signal actually targets. Recorded rather than
	// derived from PID: Setsid and Setpgid make the two equal today, and an
	// assumption like that breaks in silence.
	PGID      int
	WorkDir   string
	StartedAt time.Time
	// Env is kept so the stop command runs in the environment the start did: a
	// `docker compose down` that lost COMPOSE_PROJECT_NAME tears down the wrong
	// project, or nothing at all.
	Env map[string]string
	// Routes are the names the proxy serves this job under — its own, and one
	// per published job it runs. Kept so they are withdrawn by the same names
	// they were published under, whatever the config has become since.
	Routes []domain.JobRoute
	// LogDir is where this job's output is persisted, kept for the index: a
	// daemon adopting the job must be able to hand it back to `run logs`.
	LogDir string
	// SharedDir is the main checkout a shared job runs in — set on the real job
	// and on every claim, so a claim finds its service by key rather than by
	// name. The daemon is machine-wide: matching on the name alone would let two
	// repositories that both declare "db" release each other's.
	SharedDir string
	// ExitCode, like Status, is written by the goroutine that reaps the process
	// and read by List: both are only ever touched under the manager lock.
	ExitCode *int
	output   *outputHub    // nil for detached launcher-style services
	logs     *LogSink      // nil when the client asked for no persisted log
	exited   chan struct{} // closed when the underlying process has been reaped
	// drained is closed once the PTY has been copied to its natural EOF and the
	// log sink closed. Reaping is not the end of the output: the tail of what a
	// job printed on its way out is still in flight when exited closes, and the
	// sink batches its writes. Nil for a job with no drain of its own — one
	// adopted from the index, a task, a detached launcher.
	drained chan struct{}
}

// RouteSink is where a started job's route is published. The proxy implements
// it; nil means no proxy, which changes nothing about the job.
type RouteSink interface {
	Add(route domain.ProxyRoute)
	Remove(host string)
}

type Manager struct {
	jobs   map[string]*ManagedJob
	routes RouteSink
	index  JobIndex
	// namespaceBudget bounds the retries of a namespace's attach. Zero takes
	// domain.NamespaceCreateTimeout; a test sets it so a command that is simply
	// wrong does not hold the suite for the whole budget.
	namespaceBudget time.Duration
	orphans         Orphans
	stacks          Stacks
	mu              sync.Mutex
}

func NewManager() *Manager {
	return NewManagerWithRoutes(nil)
}

func NewManagerWithRoutes(routes RouteSink) *Manager {
	return &Manager{
		jobs:    make(map[string]*ManagedJob),
		routes:  routes,
		orphans: systemOrphans{},
		stacks:  systemStacks{},
	}
}

type ManagerParams struct {
	Routes RouteSink
	// Index is the durable record of what is up. Nil keeps the manager entirely
	// in memory, which is what a job whose process dies with us would want
	// anyway.
	Index JobIndex
	// NamespaceBudget bounds the retries of a shared job's namespace attach. Zero
	// takes domain.NamespaceCreateTimeout.
	NamespaceBudget time.Duration
	// Orphans finds and takes down the process groups a killed daemon left
	// behind. Nil takes the real one, which signals; a test supplies its own so
	// Adopt neither forks a ps nor kills anything.
	Orphans Orphans
	// Stacks verifies what a detached launcher started. Nil takes the real one,
	// which shells out to docker.
	Stacks Stacks
}

func NewManagerWith(params ManagerParams) *Manager {
	orphans := params.Orphans
	if orphans == nil {
		orphans = systemOrphans{}
	}
	stacks := params.Stacks
	if stacks == nil {
		stacks = systemStacks{}
	}
	return &Manager{
		jobs:            make(map[string]*ManagedJob),
		routes:          params.Routes,
		index:           params.Index,
		namespaceBudget: params.NamespaceBudget,
		orphans:         orphans,
		stacks:          stacks,
	}
}

// Adopt takes over the jobs a previous daemon left behind. It registers what
// rules.ReconcileJob makes of each entry, so `run ps` can report it and
// `run down` can tear it down; the adopted jobs carry no process, and every path
// that would reach for one gates on a status they do not have.
//
// It has exactly one effect on the machine, and it is the reason this pass
// exists: a foreground service whose group is still alive and still identifiably
// ours is killed here. A daemon dies without running a handler often enough —
// SIGKILL, a crash, an OOM — and nothing downstream of that death can clean up
// after it. The next start-up is the only place left.
func (m *Manager) Adopt(records []domain.JobRecord) {
	states := m.orphans.Probe(orphanQueries(records))
	stacks := m.stacks.Probe(stackQueriesOf(records))

	var reap []int
	m.mu.Lock()
	for _, record := range records {
		state := states[record.PGID]
		stack := stacks[jobKey(record.Name, record.WorkDir)]
		decision := rules.ReconcileJob(rules.ReconcileJobParams{
			Record:            record,
			WorkDirExists:     dirExists(record.WorkDir),
			GroupAlive:        state.Alive,
			IdentityConfirmed: state.IdentityConfirmed,
			StackKnownDown:    stack.Known && !stack.Up,
		})
		if !decision.Adopt {
			continue
		}
		key := jobKey(record.Name, record.WorkDir)
		if _, taken := m.jobs[key]; taken {
			continue
		}
		if decision.Reap {
			reap = append(reap, record.PGID)
		}
		exited := make(chan struct{})
		close(exited)
		m.jobs[key] = &ManagedJob{
			Name:      record.Name,
			Config:    record.Config,
			Status:    decision.Status,
			WorkDir:   record.WorkDir,
			StartedAt: record.StartedAt,
			PID:       record.PID,
			PGID:      record.PGID,
			Env:       record.Env,
			Routes:    record.Routes,
			LogDir:    record.LogDir,
			SharedDir: record.SharedDir,
			exited:    exited,
		}
	}
	m.dropDanglingClaimsLocked()
	adopted := m.upRecordsLocked()
	m.mu.Unlock()

	m.orphans.Reap(reap)

	for _, job := range m.List() {
		if job.Status == domain.JobStatusDetached {
			m.publishRoute(&job)
		}
	}
	m.saveIndex(adopted)
}

// orphanQueries asks about the only entries wtm owns a process for. A claim owns
// nothing and a detached stack belongs to Docker, so probing either would spend a
// syscall to learn something no decision reads.
func orphanQueries(records []domain.JobRecord) []GroupQuery {
	queries := make([]GroupQuery, 0, len(records))
	for _, record := range records {
		if !rules.IsForegroundService(record) || record.PGID <= 1 {
			continue
		}
		queries = append(queries, GroupQuery{PGID: record.PGID, StartedAt: record.StartedAt})
	}
	return queries
}

// dropDanglingClaimsLocked removes the claims left standing on a shared service
// that is no longer up — the one this pass has just reaped, typically. A claim is
// what makes the job table a reference count, so one pointing at nothing would
// have the next worktree told its service is already running.
func (m *Manager) dropDanglingClaimsLocked() {
	for key, job := range m.jobs {
		if job.Status != domain.JobStatusAttached {
			continue
		}
		service, found := m.realSharedLocked(sharedRef{Name: job.Name, Dir: job.SharedDir})
		if found && rules.IsJobUp(service.Status) {
			continue
		}
		delete(m.jobs, key)
	}
}

func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// upRecordsLocked snapshots what belongs in the index. The caller holds the
// lock; the write itself happens outside it, since a slow state dir must not
// hold up every List, Attach and Stop the daemon serves.
func (m *Manager) upRecordsLocked() []domain.JobRecord {
	records := make([]domain.JobRecord, 0, len(m.jobs))
	for _, job := range m.jobs {
		if !rules.IsJobUp(job.Status) {
			continue
		}
		records = append(records, domain.JobRecord{
			Name:      job.Name,
			WorkDir:   job.WorkDir,
			Config:    job.Config,
			Env:       job.Env,
			Routes:    job.Routes,
			LogDir:    job.LogDir,
			StartedAt: job.StartedAt,
			Attached:  job.Status == domain.JobStatusAttached,
			SharedDir: job.SharedDir,
			PID:       job.PID,
			PGID:      job.PGID,
		})
	}
	return records
}

func (m *Manager) saveIndex(records []domain.JobRecord) {
	if m.index == nil {
		return
	}
	_ = m.index.Save(records)
}

// persist rewrites the index from the current state. Never called with the lock
// held: it takes it to snapshot, then writes outside it.
func (m *Manager) persist() {
	if m.index == nil {
		return
	}
	m.mu.Lock()
	records := m.upRecordsLocked()
	m.mu.Unlock()
	m.saveIndex(records)
}

func jobKey(name string, workDir string) string {
	return workDir + ":" + name
}

type StartParams struct {
	Job     domain.JobConfig
	WorkDir string
	// LogDir is the worktree's log directory, resolved by the client. Empty
	// persists nothing.
	LogDir string
	Env    map[string]string
	// Routes are the names the proxy serves this job under, resolved by the
	// client for the same reason LogDir and Env are.
	Routes   []domain.JobRoute
	Streamer io.Writer
	// Shared is where a shared job actually runs and with what. Nil for a
	// per-worktree job, and cleared by startShared before it starts the real one
	// so the ordinary path takes over.
	Shared *domain.SharedJobContext
	// real marks the second pass startShared makes to spawn the service itself.
	// Without it a nil Shared would mean two different things — "this is the
	// real start" and "the client resolved nothing" — and the second would
	// quietly run one instance per worktree, which is the whole thing this
	// feature exists to stop.
	real bool
}

// Start blocks for two of the three kinds it serves, which its signature does
// not say: a task until the command exits (and errors on a non-zero one), a
// detached service until its launcher exits, while a foreground service returns
// as soon as its PTY is up and is drained in the background. All three persist
// their output when a log dir is given.
func (m *Manager) Start(params StartParams) error {
	job := params.Job
	key := jobKey(job.Name, params.WorkDir)

	if rules.IsBlankCommand(job.Cmd) {
		return fmt.Errorf("job %s has empty cmd", job.Name)
	}

	if rules.IsShared(job) && !params.real {
		return m.startShared(params)
	}

	hub := newJobHub(job)

	m.mu.Lock()
	if existing, ok := m.jobs[key]; ok && existing.Status == domain.JobStatusRunning {
		m.mu.Unlock()
		return fmt.Errorf("job %s %s", job.Name, domain.JobAlreadyRunningSuffix)
	}

	// Opened only past the refusal, and under the lock that decides it: opening
	// a log empties it, so doing it any earlier would let a start the manager is
	// about to refuse wipe the log of the job already running under that name.
	logs := openJobLog(params)

	spec := rules.ShellCommand(job.Cmd)
	cmd := exec.Command(spec.Name, spec.Args...)
	cmd.Dir = rules.JobDir(rules.JobDirParams{WorkDir: params.WorkDir, Cwd: job.Cwd})
	// Resolved once and kept on the job: the stop command must run with the same
	// ports its start did, or it tears down a stack it never brought up.
	env := withJobPorts(job, params.Env)
	cmd.Env = jobEnv(jobEnvParams{Kind: job.Kind, Overrides: env})
	// Tasks run through a plain pipe; without Setpgid they would inherit the
	// daemon's process group, leaving us no safe way to signal the whole
	// subtree on Stop. Services run through pty.Start, which forces Setsid
	// (a stronger guarantee than Setpgid), so they already get their own
	// process group automatically.
	if job.Kind == domain.JobKindTask {
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	}

	outputFile, err := spawnJob(cmd, job.Kind)
	if err != nil {
		m.mu.Unlock()
		closeSink(logs)
		return fmt.Errorf("start job %s: %w", job.Name, err)
	}

	managed := &ManagedJob{
		Name:      job.Name,
		Config:    job,
		Cmd:       cmd,
		PTY:       outputFile,
		Status:    domain.JobStatusRunning,
		PID:       cmd.Process.Pid,
		PGID:      processGroupOf(cmd.Process.Pid),
		WorkDir:   params.WorkDir,
		StartedAt: time.Now(),
		Env:       env,
		Routes:    params.Routes,
		LogDir:    params.LogDir,
		SharedDir: sharedDirOf(params),
		output:    hub,
		logs:      logs,
		exited:    make(chan struct{}),
	}
	if job.Kind != domain.JobKindTask && !rules.IsDetached(job) {
		managed.drained = make(chan struct{})
	}
	m.jobs[key] = managed
	m.mu.Unlock()

	m.publishRoute(managed)

	switch {
	case job.Kind == domain.JobKindTask:
		return m.runTask(managed, params.Streamer)
	case rules.IsDetached(job):
		if err := m.waitDetached(managed, params.Streamer); err != nil {
			m.mu.Lock()
			delete(m.jobs, key)
			m.mu.Unlock()
			return err
		}
		// The launcher is gone and the work it started is not ours. Indexed only
		// now: a launcher that failed left nothing behind.
		m.mu.Lock()
		managed.Status = domain.JobStatusDetached
		m.mu.Unlock()
		m.persist()
		return nil
	default:
		go m.drainToHub(managed)
		go m.waitForExit(managed)
		m.persist()
		return nil
	}
}

// newJobHub allocates the fan-out before the job is published in the map, so
// that a client listing or attaching the instant it appears cannot read the
// field while the starting goroutine still writes it. A detached launcher gets
// none: its output ends with the launcher, and nothing can attach afterwards.
func newJobHub(job domain.JobConfig) *outputHub {
	if rules.IsDetached(job) {
		return nil
	}
	return newOutputHub(outputHistoryBytes)
}

// openJobLog: a job still starts when its log cannot be opened.
func openJobLog(params StartParams) *LogSink {
	if params.LogDir == "" {
		return nil
	}
	sink, err := OpenLogSink(LogSinkParams{LogDir: params.LogDir, Job: params.Job.Name})
	if err != nil {
		return nil
	}
	return sink
}

// spawnJob starts the command and returns the *os.File from which the child's
// merged stdout/stderr can be drained. Services run through a PTY (so TUI
// apps render properly); tasks run through a plain pipe (so isatty(stdout)
// returns false in the child and TUI tools auto-fall back to sequential log
// output instead of corrupting the user's terminal with cursor-control codes
// targeted at a fictional PTY size).
func spawnJob(cmd *exec.Cmd, kind domain.JobKind) (*os.File, error) {
	if kind == domain.JobKindTask {
		pr, pw, err := os.Pipe()
		if err != nil {
			return nil, fmt.Errorf("create pipe: %w", err)
		}
		cmd.Stdout = pw
		cmd.Stderr = pw
		if err := cmd.Start(); err != nil {
			pr.Close()
			pw.Close()
			return nil, err
		}
		// Close the parent's copy of the write end so the reader sees EOF
		// when the child closes its own stdout/stderr (i.e. when it exits).
		pw.Close()
		return pr, nil
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}
	// Initialize the PTY to a reasonable size before the child reads it.
	// TUI frameworks like Ink (turbo, pnpm dev) decide at startup whether to
	// render based on window size — a 0x0 PTY makes them fall back to plain
	// log mode permanently. The real size is re-synced when a client attaches.
	_ = pty.Setsize(ptmx, &pty.Winsize{Rows: defaultPTYRows, Cols: defaultPTYCols})
	return ptmx, nil
}

type jobEnvParams struct {
	Kind domain.JobKind
	// Overrides is what the client resolved about the worktree. It is applied
	// before the kind's defaults below, so a worktree can never take the
	// terminal contract away from a task.
	Overrides map[string]string
}

// jobEnv returns the environment to use for spawned jobs. The daemon runs
// with Setsid so its own TTY-related env may be missing or degraded.
//
// Services run inside a PTY: we force a TUI-friendly env so frameworks like
// Ink (turbo dev, pnpm dev) render properly when a client attaches.
//
// Tasks run through a pipe and stream their output directly to the user's
// terminal. We tell them TERM=dumb so they don't emit cursor or alt-screen
// sequences (which would erase the output once the task exits) but keep
// FORCE_COLOR / COLORTERM so colored log output still works. CI=true is the
// universal "I'm in a non-interactive env, output plain logs" hint that
// turbo, jest, npm, etc. respect.
func jobEnv(params jobEnvParams) []string {
	// The inherited worktree variables are dropped, not merely overridden: a
	// request that resolved nothing must leave the job with no worktree identity
	// rather than with the one the daemon was forked with.
	env := rules.MergeEnv(rules.MergeEnvParams{
		Env:       os.Environ(),
		Clear:     domain.WorktreeScopedEnv,
		Overrides: params.Overrides,
	})

	if params.Kind == domain.JobKindTask {
		env = rules.SetEnv(env, "TERM", "dumb")
		if _, ok := rules.LookupEnv(env, "COLORTERM"); !ok {
			env = append(env, "COLORTERM=truecolor")
		}
		if _, ok := rules.LookupEnv(env, "FORCE_COLOR"); !ok {
			env = append(env, "FORCE_COLOR=1")
		}
		if _, ok := rules.LookupEnv(env, "CI"); !ok {
			env = append(env, "CI=true")
		}
		return env
	}

	if _, ok := rules.LookupEnv(env, "TERM"); !ok {
		env = append(env, "TERM=xterm-256color")
	}
	if _, ok := rules.LookupEnv(env, "COLORTERM"); !ok {
		env = append(env, "COLORTERM=truecolor")
	}
	if _, ok := rules.LookupEnv(env, "FORCE_COLOR"); !ok {
		env = append(env, "FORCE_COLOR=1")
	}
	return env
}

func (m *Manager) runTask(job *ManagedJob, streamer io.Writer) error {
	defer close(job.exited)

	key := jobKey(job.Name, job.WorkDir)

	// streamDone is closed once the streaming goroutine has drained every
	// buffered chunk. We wait on it before returning so the caller (the daemon)
	// never emits its terminal response while StatusOutput chunks are still in
	// flight on the same connection.
	//
	// Subscribe BEFORE starting drainToHub: a fast task (echo + exit) can
	// otherwise have its output drained and the hub closed before we attach,
	// making Subscribe fail with "job output closed" so the streaming goroutine
	// is never spawned and the streamed output is silently dropped.
	var streamDone chan struct{}
	if streamer != nil {
		history, ch, _, subErr := job.output.Subscribe()
		if subErr == nil {
			streamDone = make(chan struct{})
			go func() {
				defer close(streamDone)
				// Replay any history snapshotted at Subscribe time before ranging
				// the live channel — defensive, since we now subscribe before any
				// write, so history is normally empty on this path.
				if len(history) > 0 {
					_, _ = streamer.Write(history)
				}
				for chunk := range ch {
					_, _ = streamer.Write(chunk)
				}
			}()
		}
	}

	// drained closes once drainToHub has copied the PTY to its natural EOF and
	// closed the hub. We wait on it before tearing anything down so a fast
	// task's final output isn't truncated — the same race waitDetached guards
	// against (LUC-84): on a fast machine io.Copy has already read everything,
	// on a slow CI runner closing the master PTY interrupts the read mid-buffer
	// and the streamed output is silently dropped.
	drained := make(chan struct{})
	go func() {
		m.drainToHub(job)
		close(drained)
	}()

	waitErr := job.Cmd.Wait()

	// Once the task releases the slave, the master read returns EOF (Darwin) or
	// EIO (Linux) and io.Copy returns on its own; the force-close after the
	// grace period is only a liveness backstop in case a descendant keeps the
	// slave open.
	select {
	case <-drained:
	case <-time.After(detachedDrainGracePeriod):
	}
	_ = job.PTY.Close()
	<-drained

	// drainToHub has now closed the hub, but the history ring buffer still
	// holds the output — snapshot it for the error path so the CLI can surface
	// "why it failed" without the user having to run `run logs`.
	var captured string
	if waitErr != nil {
		job.output.mu.Lock()
		captured = strings.TrimSpace(string(job.output.history.Snapshot()))
		job.output.mu.Unlock()
	}

	// Closing the hub closed the subscriber channel, which lets the streaming
	// goroutine drain and exit; wait for it so all chunks reach the client.
	if streamDone != nil {
		<-streamDone
	}

	exit := exitCodeOf(waitErr)

	m.mu.Lock()
	delete(m.jobs, key)
	m.mu.Unlock()

	if waitErr != nil {
		// When a streamer was attached the client already saw the output live,
		// so we return a concise error instead of re-embedding the full capture.
		if streamer != nil {
			return failure{message: fmt.Sprintf("task %s failed (exit %d)", job.Name, exit), code: exit}
		}
		if captured != "" {
			return failure{message: fmt.Sprintf("task %s failed (exit %d):\n%s", job.Name, exit, captured), code: exit}
		}
		return fmt.Errorf("task %s failed: %w", job.Name, waitErr)
	}
	return nil
}

// failure is a job that ended badly, carrying the code beside the sentence
// rather than only inside it. The messages below are read by people and could
// not take a %w without growing a second copy of the error; without the code
// travelling separately, everything downstream — the JSON document's exit_code
// among it — read the 1 exitCodeOf invents for an unattributed failure.
type failure struct {
	message string
	code    int
}

func (f failure) Error() string { return f.message }

// ExitCode is what exitCodeOf reads back, the same shape *exec.ExitError offers.
func (f failure) ExitCode() int { return f.code }

// exitCodeOf invents the 1 it answers for a failure Wait did not attribute to
// the process itself; the -1 comes from the stdlib and means a signal killed it.
func exitCodeOf(err error) int {
	if err == nil {
		return 0
	}
	var carried failure
	if errors.As(err, &carried) {
		return carried.ExitCode()
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode()
	}
	return 1
}

// waitDetached blocks until the launcher exits, mirroring its output live like
// a task's and keeping a bounded copy to embed in the error on failure. On
// success the job stays registered as Running: the real work is detached.
func (m *Manager) waitDetached(job *ManagedJob, streamer io.Writer) error {
	defer close(job.exited)

	buf := newRingBuffer(detachedOutputBufferSize)
	writers := []io.Writer{buf}
	if streamer != nil {
		writers = append(writers, streamer)
	}
	if job.logs != nil {
		writers = append(writers, job.logs)
	}
	sink := io.MultiWriter(writers...)
	drained := make(chan struct{})
	go func() {
		_, _ = io.Copy(sink, job.PTY)
		close(drained)
	}()

	err := job.Cmd.Wait()

	// Same LUC-84 race runTask guards against: closing the master before the
	// drain goroutine reaches EOF truncated the output on slow CI runners.
	select {
	case <-drained:
	case <-time.After(detachedDrainGracePeriod):
	}
	_ = job.PTY.Close()
	<-drained
	job.closeLogs()

	if err != nil {
		// When a streamer was attached the client already saw the output live,
		// so return a concise error instead of re-embedding the capture (mirrors
		// runTask) — otherwise `run up` would print the launcher output twice.
		if streamer != nil {
			return failure{message: fmt.Sprintf("job %s failed (exit %d)", job.Name, exitCodeOf(err)), code: exitCodeOf(err)}
		}
		out := cleanPTYOutput(buf.String())
		if out == "" {
			return fmt.Errorf("job %s failed: %w", job.Name, err)
		}
		return fmt.Errorf("job %s failed:\n%s", job.Name, out)
	}
	return nil
}

// cleanPTYOutput expands a carriage-return redraw into distinct lines rather
// than collapsing it: docker compose writes its whole progress block that way
// (`[+] Running 1/2\r[+] Running 2/2`), and an error embedded in it must survive.
func cleanPTYOutput(raw string) string {
	raw = rules.StripTerminalEscapes(raw)
	raw = strings.ReplaceAll(raw, "\r", "\n")

	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}
	return strings.Join(lines, "\n")
}

// ringBuffer is a fixed array written round. Sliding a slice forward over an
// appended one instead spends its remaining capacity, so append reallocates and
// recopies the whole window every capacity bytes of output — per job, for the
// life of the daemon.
type ringBuffer struct {
	buf  []byte
	next int
	// full tells a partial window from a whole one once next has come back round
	// to zero.
	full bool
}

func newRingBuffer(capacity int) *ringBuffer {
	return &ringBuffer{buf: make([]byte, capacity)}
}

func (r *ringBuffer) Write(p []byte) (int, error) {
	if len(p) >= len(r.buf) {
		copy(r.buf, p[len(p)-len(r.buf):])
		r.next, r.full = 0, true
		return len(p), nil
	}

	head := copy(r.buf[r.next:], p)
	if head < len(p) {
		copy(r.buf, p[head:])
	}
	r.next += len(p)
	if r.next >= len(r.buf) {
		r.next -= len(r.buf)
		r.full = true
	}
	return len(p), nil
}

func (r *ringBuffer) String() string { return string(r.Snapshot()) }

func (r *ringBuffer) Snapshot() []byte {
	if !r.full {
		out := make([]byte, r.next)
		copy(out, r.buf[:r.next])
		return out
	}
	out := make([]byte, len(r.buf))
	tail := copy(out, r.buf[r.next:])
	copy(out[tail:], r.buf[:r.next])
	return out
}

// outputHub fans PTY output out to a rolling history buffer and every attached
// subscriber, so one job can feed a CLI streamer, a log file and any number of
// panes at once.
type outputHub struct {
	mu      sync.Mutex
	history *ringBuffer
	subs    map[int]chan []byte
	nextID  int
	closed  bool
}

func newOutputHub(capacity int) *outputHub {
	return &outputHub{history: newRingBuffer(capacity), subs: make(map[int]chan []byte)}
}

func (h *outputHub) Write(p []byte) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.history.Write(p)
	if len(h.subs) > 0 {
		// One copy of the caller's buffer for all of them: the chunk a
		// subscriber receives is shared with every other subscriber, so it is
		// read-only. A pane that wants to keep or rewrite it copies it first.
		data := make([]byte, len(p))
		copy(data, p)
		for _, sub := range h.subs {
			// A full queue means that one subscriber stopped reading; dropping
			// its chunk keeps the PTY reader and the other subscribers moving.
			select {
			case sub <- data:
			default:
			}
		}
	}
	return len(p), nil
}

// Subscribe returns the current history snapshot and a channel streaming
// subsequent writes. The returned unsubscribe func must be called to release
// the subscription; it is safe to call after the hub has been closed.
//
// Chunks arriving on the channel are shared with every other subscriber and
// must be treated as read-only — mutating one is visible to all of them.
// A slow subscriber loses chunks rather than stalling the job, so the stream
// is a live view, not a record: TailJobLog is what reads back a complete one.
func (h *outputHub) Subscribe() (history []byte, ch <-chan []byte, unsub func(), err error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, nil, nil, fmt.Errorf("job output closed")
	}
	history = h.history.Snapshot()
	id := h.nextID
	h.nextID++
	subCh := make(chan []byte, outputSubscriberQueue)
	h.subs[id] = subCh
	unsub = func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if _, ok := h.subs[id]; !ok {
			return
		}
		delete(h.subs, id)
		close(subCh)
	}
	return history, subCh, unsub, nil
}

func (h *outputHub) close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.closed = true
	for id, sub := range h.subs {
		close(sub)
		delete(h.subs, id)
	}
}

// drainToHub runs for the job's whole life: without a continuous reader the OS
// PTY buffer fills up and blocks the job's own writes before any client attaches.
func (m *Manager) drainToHub(job *ManagedJob) {
	if job.drained != nil {
		defer close(job.drained)
	}
	var sink io.Writer = job.output
	if job.logs != nil {
		sink = io.MultiWriter(job.output, job.logs)
	}
	_, _ = io.Copy(sink, job.PTY)
	job.output.close()
	job.closeLogs()
}

func (j *ManagedJob) closeLogs() {
	closeSink(j.logs)
}

func closeSink(sink *LogSink) {
	if sink != nil {
		_ = sink.Close()
	}
}

func (m *Manager) Stop(name string, workDir string) error {
	return m.stopByKey(jobKey(name, workDir))
}

// StopAll spans every worktree and every kind, detached stacks included: it
// serves `run down --all`, where stopping everything is the whole request. A
// caller after one worktree's jobs wants StopAllInWorkDir instead, and one
// shutting the daemon down wants StopForeground.
func (m *Manager) StopAll() error {
	return m.stopAllMatching(func(*ManagedJob) bool { return true })
}

// StopForeground is the shutdown path. It leaves detached jobs alone: their work
// runs outside this process and outlives it by design, and the index is what
// hands them to the next daemon. Foreground services would die with us anyway —
// stopping them first is what turns that into a clean exit rather than a SIGHUP.
func (m *Manager) StopForeground() error {
	return m.stopAllMatching(func(job *ManagedJob) bool {
		return job.Status == domain.JobStatusRunning
	})
}

func (m *Manager) StopAllInWorkDir(workDir string) error {
	return m.stopAllMatching(func(job *ManagedJob) bool {
		return job.WorkDir == workDir
	})
}

func (m *Manager) stopAllMatching(keep func(*ManagedJob) bool) error {
	m.mu.Lock()
	keys := make([]string, 0, len(m.jobs))
	for key, job := range m.jobs {
		if !rules.IsJobUp(job.Status) {
			continue
		}
		if !keep(job) {
			continue
		}
		keys = append(keys, key)
	}
	m.mu.Unlock()

	var firstErr error
	for _, key := range keys {
		if err := m.stopByKey(key); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (m *Manager) stopByKey(key string) error {
	// Snapshot the running flag under the lock so we don't race with
	// waitForExit, which mutates Status to Crashed in its own goroutine.
	m.mu.Lock()
	job, ok := m.jobs[key]
	isRunning := ok && job.Status == domain.JobStatusRunning
	m.mu.Unlock()

	if !ok {
		// Idempotent: a job that isn't tracked is already stopped, so stopping
		// it again is a no-op success. Whether the job name is actually declared
		// is validated at the command layer (which has the run.toml config).
		return nil
	}

	// A shared job is released, not stopped: the claim goes, and the service
	// only follows it once no worktree holds it any more.
	if rules.IsShared(job.Config) {
		return m.stopShared(job)
	}

	return m.stopProcess(stopProcessParams{Job: job, Running: isRunning})
}

type stopProcessParams struct {
	Job     *ManagedJob
	Running bool
}

// stopProcess is the tear-down itself, reached either directly or, for a shared
// job, once stopShared has established that no worktree holds it any more.
func (m *Manager) stopProcess(params stopProcessParams) error {
	m.withdrawRoute(params.Job)

	// Always run the stop command if configured — handles detached processes
	// like "docker compose up -d" where the launcher exits but services keep
	// running.
	if params.Job.Config.Stop != "" {
		return m.stopWithCommand(params.Job)
	}

	if !params.Running {
		return nil
	}

	return m.stopWithSignal(params.Job)
}

// AttachSession hands out the job's live PTY, for stdin forwarding and
// window-size ioctls. Release must be called when done.
//
// A job accepts any number of concurrent attachments — the run view needs
// several, and refusing the second one used to be what kept stdin
// single-writer. Nothing arbitrates that PTY now: every attachment writes
// into it directly, so two of them typing at once interleave their bytes
// (Vite's r/q/u/o shortcuts land in whichever order they arrive). Output is
// unaffected, each subscriber gets the whole stream.
type AttachSession struct {
	PTY     *os.File
	History []byte
	Stream  <-chan []byte
	Release func()
	// Writable says the subscriber may feed the job's stdin. A task's PTY field
	// holds the read end of a pipe: writing to it fails at once, which ended the
	// attach on the spot instead of streaming the task (LUC-208).
	Writable bool
}

type jobRef struct {
	Name    string
	WorkDir string
}

// attachableJob is the gate both Attach and Resize pass: a job a pane can bind
// to is registered, still running, and streaming through a hub — which a
// detached launcher never does, its output having ended with its launcher.
func (m *Manager) attachableJob(ref jobRef) (*ManagedJob, error) {
	m.mu.Lock()
	job, ok := m.jobs[jobKey(ref.Name, ref.WorkDir)]
	// A claim owns no stream; the service it holds does.
	if ok && job.Status == domain.JobStatusAttached {
		job, ok = m.realSharedLocked(sharedRef{Name: ref.Name, Dir: job.SharedDir})
	}
	// Snapshotted, never re-read: Status is written by whichever goroutine reaps
	// or stops the job, so a second read outside the lock is a race.
	var status domain.JobStatus
	if ok {
		status = job.Status
	}
	m.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("job %s not found", ref.Name)
	}
	if status == domain.JobStatusDetached {
		return nil, fmt.Errorf("job %s is detached: its launcher exited and left no stream (see wtm run logs)", ref.Name)
	}
	if status != domain.JobStatusRunning {
		return nil, fmt.Errorf("job %s is not running", ref.Name)
	}
	if job.output == nil {
		return nil, fmt.Errorf("job %s has no attachable output (detached launcher)", ref.Name)
	}
	return job, nil
}

func (m *Manager) Attach(name string, workDir string) (*AttachSession, error) {
	job, err := m.attachableJob(jobRef{Name: name, WorkDir: workDir})
	if err != nil {
		return nil, err
	}

	history, stream, unsub, err := job.output.Subscribe()
	if err != nil {
		return nil, err
	}
	return &AttachSession{
		PTY:      job.PTY,
		History:  history,
		Stream:   stream,
		Release:  unsub,
		Writable: !rules.RunsOnPipe(job.Config.Kind),
	}, nil
}

type ResizeParams struct {
	Name    string
	WorkDir string
	Cols    int
	Rows    int
}

// Resize sizes a job's PTY to the pane rendering it, which is the only way the
// child ever reflows: the emulator drawing that pane does not re-wrap what it
// has already been sent. Each attached pane resizes for itself, so the last one
// to speak wins — a job shown twice at two sizes is drawn for the latest.
func (m *Manager) Resize(params ResizeParams) error {
	job, err := m.attachableJob(jobRef{Name: params.Name, WorkDir: params.WorkDir})
	if err != nil {
		return err
	}
	if job.Config.Kind == domain.JobKindTask {
		return fmt.Errorf("job %s runs on a pipe, not a PTY (task)", params.Name)
	}

	if err := setWinsize(winsizeParams{File: job.PTY, Cols: params.Cols, Rows: params.Rows}); err != nil {
		return fmt.Errorf("job %s: %w", params.Name, err)
	}
	return nil
}

func (m *Manager) List() []ManagedJob {
	m.mu.Lock()
	defer m.mu.Unlock()

	result := make([]ManagedJob, 0, len(m.jobs))
	for _, job := range m.jobs {
		result = append(result, *job)
	}
	return result
}

func (m *Manager) IsRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, job := range m.jobs {
		if job.Status == domain.JobStatusRunning {
			return true
		}
	}
	return false
}

func (m *Manager) markStopped(job *ManagedJob) {
	// An adopted job has no PTY: the daemon that owned it is gone, and this one
	// only ever knew how to stop it.
	if job.PTY != nil {
		job.PTY.Close()
	}

	m.mu.Lock()
	job.Status = domain.JobStatusStopped
	m.mu.Unlock()

	m.persist()
}

func (m *Manager) stopWithCommand(job *ManagedJob) error {
	if rules.IsBlankCommand(job.Config.Stop) {
		return m.stopWithSignal(job)
	}

	spec := rules.ShellCommand(job.Config.Stop)
	cmd := exec.Command(spec.Name, spec.Args...)
	cmd.Dir = job.WorkDir
	cmd.Env = rules.MergeEnv(rules.MergeEnvParams{
		Env:       os.Environ(),
		Clear:     domain.WorktreeScopedEnv,
		Overrides: job.Env,
	})
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("stop %s: %s: %w", job.Name, strings.TrimSpace(string(out)), err)
	}

	m.markStopped(job)
	return nil
}

func (m *Manager) stopWithSignal(job *ManagedJob) error {
	if job.Cmd == nil || job.Cmd.Process == nil {
		return nil
	}

	pid := job.Cmd.Process.Pid

	// A negative PID targets the whole process group, so npm AND every
	// node child it spawned receive SIGTERM. ESRCH means the group is
	// already gone, which is fine. Any other failure (e.g. job spawned
	// without its own group) falls back to signalling just the parent.
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil && !errors.Is(err, syscall.ESRCH) {
		if sigErr := job.Cmd.Process.Signal(syscall.SIGTERM); sigErr != nil && !errors.Is(sigErr, os.ErrProcessDone) {
			return fmt.Errorf("signal %s: %w", job.Name, sigErr)
		}
	}

	// Wait for actual reaping so the caller never sees "stopped" while a
	// child is still running. Dev TUIs (next, vite, turbo) sometimes
	// swallow SIGTERM to run their own cleanup — escalate to SIGKILL on
	// the whole group if they overrun the grace period.
	select {
	case <-job.exited:
	case <-time.After(stopGracePeriod):
		_ = syscall.Kill(-pid, syscall.SIGKILL)
		<-job.exited
	}
	waitDrained(job)

	m.markStopped(job)
	return nil
}

// waitDrained holds until the job's last bytes have reached its log. The daemon
// stops its jobs on the way out and exits as soon as they report stopped, so
// without this the shutdown output — the one thing the log is opened for — is
// still in the sink's buffer when the process goes. Bounded: a PTY whose other
// end is held open by an orphan would otherwise never reach EOF.
func waitDrained(job *ManagedJob) {
	if job.drained == nil {
		return
	}
	select {
	case <-job.drained:
	case <-time.After(drainGracePeriod):
	}
}

func (m *Manager) waitForExit(job *ManagedJob) {
	defer close(job.exited)

	exit := exitCodeOf(job.Cmd.Wait())

	m.mu.Lock()
	job.ExitCode = &exit
	crashed := job.Status == domain.JobStatusRunning && job.Config.Stop == ""
	if crashed {
		job.Status = domain.JobStatusCrashed
	}
	m.mu.Unlock()

	if !crashed {
		return
	}

	m.withdrawRoute(job)
	m.persist()
}

// publishRoute makes a started job reachable by name — under its own, and under
// each of the names it holds for the jobs it runs itself. A route whose port did
// not resolve is skipped rather than published on a guess: the process is one,
// but the ports it was given are its children's too.
func (m *Manager) publishRoute(job *ManagedJob) {
	if m.routes == nil {
		return
	}
	ports := jobPorts(job.Config, job.Env)
	for _, route := range job.Routes {
		port, resolved := ports[route.Port]
		if !resolved {
			continue
		}
		m.routes.Add(domain.ProxyRoute{
			Host:     route.Host,
			Target:   fmt.Sprintf(domain.ProxyTargetFmt, port),
			Job:      route.Job,
			Worktree: job.Env[domain.EnvWorktree],
			Project:  job.Env[domain.EnvProject],
		})
	}
}

func (m *Manager) withdrawRoute(job *ManagedJob) {
	if m.routes == nil {
		return
	}
	for _, route := range job.Routes {
		m.routes.Remove(route.Host)
	}
}
