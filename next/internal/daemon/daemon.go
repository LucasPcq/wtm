// Package daemon is the run daemon: the jobs of every worktree of the machine, started, watched and stopped as its clients ask, recorded for the next daemon and taken over from the one before. It is blind to git: a worktree is the directory and the environment its client sends.
package daemon

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

const (
	stopGrace = 5 * time.Second
	// stopLimit bounds a stack's stop command: compose takes its time to take a stack down, never minutes.
	stopLimit = 5 * time.Minute
	// lastLines is how much of its output a job.crashed carries.
	lastLines = 10
	maxWindow = 1<<16 - 1
)

// Params is what a [Daemon] reaches the world through: how it starts a
// process, finds orphans, keeps its index, keeps each job's log and reads it
// back, and tells whether a worktree is still there; where it publishes its
// job events (nil publishes none); its clock (time.Now when nil); how long
// a stopped job has to exit before it is killed (5 seconds when zero); how
// long a shared job's namespace create is retried, and how often (30 seconds,
// every second, when zero); and the proxy (nil for none: each job answers on
// its port) with how it finds the proxy behind port 80, asked on every answer
// (nil finds none).
type Params struct {
	Spawn   Spawn
	Orphans Orphans
	Index   Index
	OpenLog OpenLog
	ReadLog ReadLog
	Exists  Exists
	Publish Publish
	Now     func() time.Time
	Grace   time.Duration

	NamespaceBudget time.Duration
	NamespaceRetry  time.Duration
	// After is the timer a namespace's create attempt or a stop command hangs against, time.After when nil.
	After func(time.Duration) <-chan time.Time

	Routes Routes
	Probe  Probe
}

// A Daemon holds the jobs of the machine. Its methods are safe for concurrent
// use, one per client request.
type Daemon struct {
	params Params

	mu       sync.Mutex
	jobs     map[key]*job
	stopping bool
	// quit is closed once the daemon stops: what waits outside the job table (a namespace's create) ends with it.
	quit chan struct{}

	// saving serializes the writes of the index, each from a snapshot taken under it: a slow write never lands after a newer one.
	saving sync.Mutex

	// waited, set by a test, runs each time a shared start waits on a job being stopped or launched.
	waited func()
}

type key struct {
	workDir string
	name    string
}

// A job is what the daemon knows of one: its record; its state and how it
// ended; whether it was taken over from the daemon before (adopted); its
// process while one of the daemon's runs it, and what it printed (nil for one
// taken over, whose last line its log gave, last); the job up it replaces
// while its launcher runs again (previous, a detached stack); whether a stop
// reached it and for which command; and done, closed once the end of its
// process is settled.
type job struct {
	record   run.Record
	state    run.State
	reaped   bool
	adopted  bool
	exitCode *int
	process  Process
	output   *output
	terminal *terminal
	last     *run.Line
	previous *job
	stopped  bool
	stopID   string
	done     chan struct{}
	// served marks a job whose names its proxy serves: only those are withdrawn, never the same names a newer job of its worktree serves.
	served bool
	// mainHold numbers the main checkout's holds of a shared job: a start lets go only of the one it took, never a newer one.
	mainHold int
	// unannounced marks a claim whose start has not announced it yet, nor will its release.
	unannounced bool
}

// New returns a daemon over params, holding no job until [Daemon.Adopt].
func New(params Params) *Daemon {
	if params.Now == nil {
		params.Now = time.Now
	}
	params.Grace = cmp.Or(params.Grace, stopGrace)
	if params.After == nil {
		params.After = time.After
	}

	return &Daemon{params: params, jobs: map[key]*job{}, quit: make(chan struct{})}
}

// ----------------------------------------------------------------------------
// Recovery

// Adopt takes over the jobs the index names, as [run.Reconcile] decides each:
// an orphan left by a daemon that died is reaped, a foreground service whose
// process died with it is listed crashed, a detached stack and a claim come
// back as they were. It then rewrites the index.
func (d *Daemon) Adopt() {
	records := d.params.Index.Load()
	started := map[int]time.Time{}
	for _, record := range records {
		if record.Foreground() && record.PGID > 1 {
			started[record.PGID] = record.StartedAt
		}
	}
	groups := map[int]run.Group{}
	if len(started) > 0 {
		groups = d.params.Orphans.Probe(started)
	}

	// Read before the lock: a log is a file, its last line is not worth holding every request for.
	last := make([]*run.Line, len(records))
	for i, record := range records {
		if lines := d.readLog(run.LogFile{Dir: record.Spec.LogDir, Job: record.Spec.Job.Name}, 1); len(lines) > 0 {
			last[i] = &lines[0]
		}
	}

	var reap []int
	d.mu.Lock()
	for i, record := range records {
		decision := run.Reconcile(run.ReconcileParams{Record: record, Group: groups[record.PGID], WorkDirExists: d.params.Exists(record.Spec.WorkDir)})
		k := keyOf(record.Spec)
		if !decision.Keep || d.jobs[k] != nil {
			continue
		}
		if decision.Reaped {
			reap = append(reap, record.PGID)
		}
		d.jobs[k] = &job{record: record, state: decision.State, reaped: decision.Reaped, adopted: true, last: last[i], done: closed()}
	}
	d.dropDanglingClaimsLocked()
	for _, j := range d.jobs {
		d.serveLocked(j)
	}
	d.mu.Unlock()

	if len(reap) > 0 {
		d.params.Orphans.Reap(reap)
	}
	d.save()
}

// ----------------------------------------------------------------------------
// Start

// Start starts the job spec asks for, and hands each line it prints to out
// (nil drops them) while Start runs: a task until it exits, failing with
// [run.CodeTaskFailed] when it exits non-zero; a detached launcher until it
// exits, its stack then listed running with no process of the daemon's, or
// failing with [run.CodeLaunchFailed]; a foreground service only until it is
// up, the daemon then watching it. It refuses a job already running in that
// worktree, [run.CodeJobRunning], and any start once the daemon is stopping.
// A job of shared scope runs once, in its main checkout, every other worktree
// claiming it (see startShared).
func (d *Daemon) Start(spec run.Spec, out func(run.Line)) (run.Job, error) {
	if spec.Job.Scope == project.Shared {
		return d.startShared(spec, out)
	}
	return d.start(spec, out)
}

// start starts the process of the job spec asks for, in the worktree it names.
func (d *Daemon) start(spec run.Spec, out func(run.Line)) (run.Job, error) {
	if strings.TrimSpace(spec.Job.Cmd) == "" {
		return run.Job{}, &run.Error{Code: run.CodeEmptyCommand, Message: fmt.Sprintf("job %s has no cmd", spec.Job.Name)}
	}

	k := keyOf(spec)
	output := newOutput(d.params.Now)
	if spec.Job.Kind == project.Task || spec.Detached() {
		output.out = out
	}
	d.mu.Lock()
	if d.stopping {
		d.mu.Unlock()
		return run.Job{}, &run.Error{Code: run.CodeStopping, Message: "the run daemon is stopping"}
	}
	previous := d.jobs[k]
	if previous != nil && previous.process != nil {
		d.mu.Unlock()
		return run.Job{}, &run.Error{Code: run.CodeJobRunning, Message: fmt.Sprintf("job %s is already running in %s", spec.Job.Name, spec.WorkDir)}
	}
	// Opened only past the refusal: opening a log empties it, the log of the job a refused start would name included.
	output.log = d.openLog(run.LogFile{Dir: spec.LogDir, Job: spec.Job.Name, Append: previous != nil && previous.up()})
	process, err := d.params.Spawn(spec.Command(output.write))
	if err != nil {
		d.mu.Unlock()
		output.end()
		return run.Job{}, &run.Error{Code: run.CodeLaunchFailed, Message: fmt.Sprintf("start job %s: %v", spec.Job.Name, err)}
	}
	j := &job{record: run.Record{Spec: spec, StartedAt: d.params.Now(), PID: process.PID(), PGID: process.PID(), SharedDir: sharedDir(spec)}, state: run.Running, process: process, output: output, done: make(chan struct{})}
	if previous != nil && previous.up() && spec.Detached() {
		j.previous = previous
	}
	if spec.Command(nil).Terminal {
		j.terminal = newTerminal(process, j.done)
	}
	d.jobs[k] = j
	if spec.Job.Kind == project.Service && !spec.Detached() {
		d.serveLocked(j)
	}
	d.mu.Unlock()

	switch {
	case spec.Job.Kind == project.Task:
		d.publish(j, publishParams{Event: domain.JobStarted{JobChange: d.change(j, nil)}})
		return d.runTask(j)
	case spec.Detached():
		return d.launch(j)
	}
	d.publish(j, publishParams{Event: domain.JobStarted{JobChange: d.change(j, nil)}})
	d.save()
	go func() { d.settle(j, process.Wait()) }()

	return d.view(j), nil
}

// runTask waits for a task to end, settles it and returns it, with the
// failure of a non-zero exit nobody asked for.
func (d *Daemon) runTask(j *job) (run.Job, error) {
	d.settle(j, j.process.Wait())
	view := d.view(j)
	if view.State != run.Crashed {
		return view, nil
	}

	return view, &run.Error{Code: run.CodeTaskFailed, Message: fmt.Sprintf("task %s exited with code %d", view.Name, *view.ExitCode), ExitCode: view.ExitCode}
}

// launch waits for a detached launcher j to exit: its stack is then up, with
// no process of the daemon's; a launcher that failed gives the worktree back
// the stack it had, if any. A launcher stopped (by a shutdown, or by a
// client's stop, which runs the stack's stop command next) leaves the stack up
// and indexed: the one it had, else the one it may have started in part, for
// the stop command or the next daemon to take down.
func (d *Daemon) launch(j *job) (run.Job, error) {
	code := j.process.Wait()
	j.output.end()

	d.mu.Lock()
	j.process = nil
	stopped := j.stopped
	k := keyOf(j.record.Spec)
	switch {
	// The stack given back is served: one a stopped launcher left up never was.
	case stopped && j.previous != nil:
		d.jobs[k] = j.previous
		d.serveLocked(j.previous)
	case stopped:
	case code != 0:
		j.exitCode = &code
		delete(d.jobs, k)
		if j.previous != nil {
			d.jobs[k] = j.previous
			d.serveLocked(j.previous)
		}
		d.dropDanglingClaimsLocked()
	default:
		// The stack it replaces may have served names this one no longer has.
		if j.previous != nil {
			d.withdrawLocked(j.previous)
		}
		d.serveLocked(j)
	}
	j.previous = nil
	d.mu.Unlock()
	defer close(j.done)
	if !stopped && code != 0 {
		return d.view(j), &run.Error{Code: run.CodeLaunchFailed, Message: fmt.Sprintf("the launcher of %s exited with code %d", j.record.Spec.Job.Name, code), ExitCode: &code}
	}

	d.save()
	if !stopped {
		d.publish(j, publishParams{Event: domain.JobStarted{JobChange: d.change(j, nil)}})
	}

	return d.view(j), nil
}

// settle records the end of a job's process, exited with code, writes the
// index and publishes it: stopped when a stop reached it, exited for a task
// that succeeded, crashed otherwise (a service never ends on its own).
func (d *Daemon) settle(j *job, code int) {
	last := j.output.end()

	d.mu.Lock()
	j.process = nil
	j.exitCode = &code
	switch {
	case j.stopped:
		j.state = run.Stopped
	case code == 0 && j.record.Spec.Job.Kind == project.Task:
		j.state = run.Exited
	default:
		j.state = run.Crashed
	}
	state, stopID := j.state, j.stopID
	d.withdrawLocked(j)
	// A shared job that ended takes its claims with it, which its events name as they held it: a claim on nothing would refuse its worktree's next start.
	held := d.heldLocked(j)
	d.dropDanglingClaimsLocked()
	d.mu.Unlock()

	// Closed once announced: a stop returns with the job.stopped on the bus and the index written.
	defer close(j.done)
	d.save()
	change := func(code *int) domain.JobChange {
		change := d.change(j, code)
		change.HeldBy = held
		return change
	}
	switch state {
	case run.Stopped:
		d.publish(j, publishParams{Event: domain.JobStopped{JobChange: change(nil)}, CorrelationID: stopID})
	case run.Exited:
		d.publish(j, publishParams{Event: domain.JobExited{JobChange: change(&code)}})
	default:
		crashed := change(&code)
		crashed.LastLines = last
		d.publish(j, publishParams{Event: domain.JobCrashed{JobChange: crashed}})
	}
}

// ----------------------------------------------------------------------------
// Stop

// Stop stops a job: a process of the daemon's has its group stopped, a
// detached stack runs its stop command (failing with [run.CodeStopFailed] when
// it exits non-zero), a shared job is let go (see release). A job not up is
// [run.OutcomeNotRunning], and one that ended leaves the listing.
func (d *Daemon) Stop(req run.Stop) (run.Outcome, error) {
	k := key{workDir: req.WorkDir, name: req.Name}
	d.mu.Lock()
	j := d.jobs[k]
	if j == nil || !j.up() {
		delete(d.jobs, k)
		d.mu.Unlock()
		return run.OutcomeNotRunning, nil
	}
	shared := j.record.Spec.Job.Scope == project.Shared
	d.mu.Unlock()

	if shared {
		return d.release(j, req.CorrelationID)
	}
	return d.halt(j, req.CorrelationID)
}

// halt stops j itself, for the command of correlation: a process of the
// daemon's has its group stopped, a detached stack runs its stop command.
func (d *Daemon) halt(j *job, correlation string) (run.Outcome, error) {
	k := keyOf(j.record.Spec)
	d.mu.Lock()
	if !j.up() {
		d.mu.Unlock()
		return run.OutcomeNotRunning, nil
	}
	j.stopped, j.stopID = true, correlation
	process, done, previous := j.process, j.done, j.previous
	d.mu.Unlock()

	switch {
	case process != nil:
		process.Stop(d.params.Grace)
		<-done
		// A launcher stopped leaves its stack up, the one it had or the one it may have started: its stop command takes it down, unless a shutdown leaves it to the next daemon.
		if j.record.Hold() != run.HoldDetached || d.isStopping() {
			break
		}
		d.mu.Lock()
		stack := d.jobs[k]
		// Another stop may have taken the stack down meanwhile, or a new start put its own launcher there: neither is this stop's to run.
		if stack == nil || (stack != j && stack != previous) || !stack.up() {
			d.mu.Unlock()
			return run.OutcomeStopped, nil
		}
		stack.stopped, stack.stopID = true, correlation
		d.mu.Unlock()
		if err := d.stopStack(stack); err != nil {
			return "", err
		}
	default:
		if err := d.stopStack(j); err != nil {
			return "", err
		}
	}

	return run.OutcomeStopped, nil
}

// stopStack runs a detached stack's stop command, then lists it stopped; a
// stop command that fails, or still runs after 5 minutes or once the daemon
// stops, leaves it up and indexed: a start waiting on it never waits forever.
func (d *Daemon) stopStack(j *job) error {
	spec := j.record.Spec
	code, err := d.runUntil(spec.StopCommand(nil), stopLimit)
	if err != nil || code != 0 {
		d.mu.Lock()
		j.stopped = false
		// Left up, it is served, a launcher stopped before it served it included.
		if d.jobs[keyOf(spec)] == j {
			d.serveLocked(j)
		}
		d.mu.Unlock()
		why := fmt.Sprintf("failed (exit %d)", code)
		switch {
		case errors.Is(err, errDeadline) && d.isStopping():
			why = "was stopped as the run daemon stopped"
		case errors.Is(err, errDeadline):
			why = fmt.Sprintf("was still running after %s, and was stopped", stopLimit)
		}
		return &run.Error{Code: run.CodeStopFailed, Message: fmt.Sprintf("the stop command of %s %s", spec.Job.Name, why), ExitCode: &code}
	}

	d.mu.Lock()
	j.state = run.Stopped
	d.withdrawLocked(j)
	d.dropDanglingClaimsLocked()
	d.mu.Unlock()
	d.save()
	d.publish(j, publishParams{Event: domain.JobStopped{JobChange: d.change(j, nil)}, CorrelationID: j.stopID})

	return nil
}

// StopAll stops every job up in req's worktree, every job of the machine when
// it names none, together, its shared jobs released after, and returns each as
// it stood with what the stop did; it fails with the first stop that failed,
// once every stop ended.
func (d *Daemon) StopAll(req run.StopAll) ([]run.StoppedJob, error) {
	var reached []run.StoppedJob
	for _, view := range d.List(req.WorkDir) {
		if view.Up {
			reached = append(reached, run.StoppedJob{Job: view})
		}
	}

	errs := make([]error, len(reached))
	stop := func(i int) {
		reached[i].Outcome, errs[i] = d.Stop(run.Stop{Name: reached[i].Job.Name, WorkDir: reached[i].Job.WorkDir, CorrelationID: req.CorrelationID})
	}
	var stopping sync.WaitGroup
	var shared []int
	for i := range reached {
		if d.isShared(keyOf(run.Spec{Job: project.Job{Name: reached[i].Job.Name}, WorkDir: reached[i].Job.WorkDir})) {
			shared = append(shared, i)
			continue
		}
		stopping.Go(func() { stop(i) })
	}
	stopping.Wait()
	// One after the other: whether a shared job stops depends on the claims its releases leave.
	for _, i := range shared {
		stop(i)
	}

	return reached, cmp.Or(slices.DeleteFunc(errs, func(err error) bool { return err == nil })...)
}

func (d *Daemon) isStopping() bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	return d.stopping
}

// Shutdown refuses every later start and stops every process of the daemon's
// (foreground services, tasks, launchers), leaving detached stacks and claims
// to the index, for the next daemon.
func (d *Daemon) Shutdown() {
	d.mu.Lock()
	if !d.stopping {
		close(d.quit)
	}
	d.stopping = true
	var running []*job
	for _, j := range d.jobs {
		if j.process != nil {
			running = append(running, j)
		}
	}
	d.mu.Unlock()

	// Halted, never released: a shared job's claims stay in the index for the next daemon, which drops those its job is gone from.
	var stopping sync.WaitGroup
	for _, j := range running {
		stopping.Go(func() { _, _ = d.halt(j, "") })
	}
	stopping.Wait()
}

// ----------------------------------------------------------------------------
// Lines and terminals

// Lines hands out the last lines of the job req names (all it keeps when
// req.Lines is 0) and, when req.Follow, each new one until the job ends or ctx
// is done, then returns the job as it then stands. A job the daemon runs no
// process of anymore (taken over from the daemon before) gives its log. It
// fails with [run.CodeJobNotFound] for a job it does not know.
func (d *Daemon) Lines(ctx context.Context, req run.Tail, out func(run.Line)) (run.Job, error) {
	j, err := d.find(key{workDir: req.WorkDir, name: req.Name})
	if err != nil {
		return run.Job{}, err
	}
	n := req.Lines
	if n <= 0 || n > run.MaxTail {
		n = run.MaxTail
	}
	if j.output == nil {
		for _, line := range d.readLog(run.LogFile{Dir: j.record.Spec.LogDir, Job: req.Name}, n) {
			out(line)
		}
		return d.view(j), nil
	}

	past, f, stop, ended := j.output.follow(n, req.Follow)
	defer stop()
	for _, line := range past {
		out(line)
	}
	for f != nil {
		select {
		case line, open := <-f.lines:
			if open {
				out(line)
				continue
			}
			if f.behind.Load() {
				return d.view(j), &run.Error{Code: run.CodeFollowerBehind, Message: fmt.Sprintf("the follower of %s fell %d lines behind", req.Name, followQueue)}
			}
			ended, f = true, nil
		case <-ctx.Done():
			return d.view(j), ctx.Err()
		}
	}
	if ended {
		// The process ended: what the listing says of it is settled once done is closed.
		select {
		case <-j.done:
		case <-ctx.Done():
			return d.view(j), ctx.Err()
		}
	}

	return d.view(j), nil
}

// Attach attaches a client to the terminal of the job req names, sized
// req.Cols by req.Rows when both are set: what the job printed last, then what
// it prints, and its terminal to type into (none for a job under no
// terminal). It fails with [run.CodeJobNotFound] for a job no process of the
// daemon's runs.
func (d *Daemon) Attach(req run.Window) (run.Session, error) {
	j, process, err := d.running(key{workDir: req.WorkDir, name: req.Name})
	if err != nil {
		return run.Session{}, err
	}
	session := run.Session{}
	session.History, session.Stream, session.Release = j.output.attach()
	if j.terminal != nil {
		session.Input = j.terminal
		if fits(req) {
			_ = process.Resize(req.Cols, req.Rows)
		}
	}

	return session, nil
}

// Resize sizes the terminal of the job req names, failing with
// [run.CodeNoTerminal] for a job under none and [run.CodeInvalidRequest] for a
// size no terminal has.
func (d *Daemon) Resize(req run.Window) error {
	if !fits(req) {
		return &run.Error{Code: run.CodeInvalidRequest, Message: fmt.Sprintf("no terminal is %d by %d", req.Cols, req.Rows)}
	}
	j, process, err := d.running(key{workDir: req.WorkDir, name: req.Name})
	if err != nil {
		return err
	}
	if !j.record.Spec.Command(nil).Terminal {
		return &run.Error{Code: run.CodeNoTerminal, Message: fmt.Sprintf("job %s runs under no terminal", req.Name)}
	}

	return process.Resize(req.Cols, req.Rows)
}

// fits reports whether a terminal can be sized as w says.
func fits(w run.Window) bool {
	return w.Cols >= 1 && w.Rows >= 1 && w.Cols <= maxWindow && w.Rows <= maxWindow
}

// servingLocked returns the job of k, the shared job it claims for a claim;
// the lock is held.
func (d *Daemon) servingLocked(k key) *job {
	j := d.jobs[k]
	if j != nil && j.record.Claim {
		if claimed := d.jobs[key{workDir: j.record.SharedDir, name: k.name}]; claimed != nil {
			return claimed
		}
	}
	return j
}

// find returns the job of k (for a claim, the shared job it claims), failing with [run.CodeJobNotFound] when the
// daemon does not know it.
func (d *Daemon) find(k key) (*job, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	j := d.servingLocked(k)
	if j == nil {
		return nil, &run.Error{Code: run.CodeJobNotFound, Message: fmt.Sprintf("no job %s in %s", k.name, k.workDir)}
	}

	return j, nil
}

// running returns the job of k and its process, failing with
// [run.CodeJobNotFound] when no process of the daemon's runs it.
func (d *Daemon) running(k key) (*job, Process, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	j := d.servingLocked(k)
	if j == nil || j.process == nil {
		return nil, nil, &run.Error{Code: run.CodeJobNotFound, Message: fmt.Sprintf("no job %s running in %s", k.name, k.workDir)}
	}

	return j, j.process, nil
}

// readLog returns the last n lines of the log file names, none when its job
// keeps none.
func (d *Daemon) readLog(file run.LogFile, n int) []run.Line {
	if file.Dir == "" || d.params.ReadLog == nil {
		return nil
	}

	return d.params.ReadLog(file, n)
}

// openLog opens the log file names, none when its job keeps none or it cannot
// be opened: a job runs all the same.
func (d *Daemon) openLog(file run.LogFile) Log {
	if file.Dir == "" || d.params.OpenLog == nil {
		return nil
	}
	log, err := d.params.OpenLog(file)
	if err != nil {
		return nil
	}

	return log
}

// ----------------------------------------------------------------------------
// The listing

// List returns the jobs the daemon knows in the worktree at workDir, every
// worktree's when empty, by worktree then name.
func (d *Daemon) List(workDir string) []run.Job {
	public := d.Proxy().PublicPort
	d.mu.Lock()
	defer d.mu.Unlock()
	jobs := []run.Job{}
	for k, j := range d.jobs {
		if workDir == "" || k.workDir == workDir {
			jobs = append(jobs, j.viewLocked(public))
		}
	}
	slices.SortFunc(jobs, func(a, b run.Job) int { return cmp.Or(cmp.Compare(a.WorkDir, b.WorkDir), cmp.Compare(a.Name, b.Name)) })

	return jobs
}

// Busy reports whether a process of the daemon's runs, or its proxy serves a
// job's name: while one does, the daemon must not stop on its own, which
// would take the name down while the stack behind it runs.
func (d *Daemon) Busy() bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, j := range d.jobs {
		if j.process != nil || j.served && d.params.Routes.Port() > 0 {
			return true
		}
	}

	return false
}

func (d *Daemon) view(j *job) run.Job {
	public := d.Proxy().PublicPort
	d.mu.Lock()
	defer d.mu.Unlock()

	return j.viewLocked(public)
}

// viewLocked is the job as a client reads it, its URL on the proxy's
// publicPort; the daemon's lock is held.
func (j *job) viewLocked(public int) run.Job {
	spec := j.record.Spec
	view := run.Job{
		Name:      spec.Job.Name,
		WorkDir:   spec.WorkDir,
		Profile:   spec.Profile,
		Up:        j.up(),
		Kind:      spec.Job.Kind,
		State:     j.state,
		Hold:      j.record.Hold(),
		Reaped:    j.reaped,
		URL:       spec.URL(public),
		Ports:     spec.Ports(),
		ExitCode:  j.exitCode,
		SharedDir: j.record.SharedDir,
	}
	if view.Up {
		view.StartedAt = j.record.StartedAt
	}
	view.LastLine = j.last
	if j.output != nil {
		view.LastLine = j.output.lastLine()
	}
	// A detached stack's launcher is gone, a claim runs nothing, and a process taken over from the daemon before is dead or reaped: a pid would point at nothing, or in time at a stranger.
	if view.Hold == run.HoldOwn && !j.adopted {
		view.PID = j.record.PID
	}

	return view
}

// up reports whether the job is up: running, or a claim on a shared job.
func (j *job) up() bool {
	return j.record.Claim || j.state == run.Running
}

// ----------------------------------------------------------------------------
// The index and the events

// save rewrites the index with the jobs up a next daemon can take over: the
// foreground services, the detached stacks and the claims, never a task nor a
// launcher still running, but the stack a launcher running again replaces.
func (d *Daemon) save() {
	d.saving.Lock()
	defer d.saving.Unlock()

	d.mu.Lock()
	var records []run.Record
	for _, j := range d.jobs {
		hold := j.record.Hold()
		switch {
		// A stack launched again stays up while its launcher runs: a daemon killed then must hand it over.
		case j.process != nil && j.previous != nil:
			records = append(records, j.previous.record)
		case hold == run.HoldClaim, j.state == run.Running && (j.record.Foreground() || hold == run.HoldDetached && j.process == nil):
			records = append(records, j.record)
		}
	}
	d.mu.Unlock()
	slices.SortFunc(records, func(a, b run.Record) int {
		return cmp.Or(cmp.Compare(a.Spec.WorkDir, b.Spec.WorkDir), cmp.Compare(a.Spec.Job.Name, b.Spec.Job.Name))
	})

	// A daemon that cannot write its index runs its jobs all the same: the next daemon only loses them.
	_ = d.params.Index.Save(records)
}

// publishParams is a job event and the correlation id it carries, the start's
// when empty.
type publishParams struct {
	Event         domain.Event
	CorrelationID string
}

// publish relays a job's event on the bus, under the repository its client
// named; a job started with no origin publishes none.
func (d *Daemon) publish(j *job, params publishParams) {
	origin := j.record.Spec.Origin
	if d.params.Publish == nil || origin == nil {
		return
	}
	d.params.Publish(domain.Line{
		V:             domain.StreamVersion,
		TS:            d.params.Now().UTC().Format(time.RFC3339Nano),
		Repo:          &domain.Repo{Root: origin.Root, CommonDir: origin.CommonDir},
		CorrelationID: cmp.Or(params.CorrelationID, origin.CorrelationID),
		Event:         params.Event,
	})
}

// change is what a job event says of j, ended with code (nil for none), and
// for a shared job, the worktrees holding it.
func (d *Daemon) change(j *job, code *int) domain.JobChange {
	spec := j.record.Spec
	change := domain.JobChange{
		Worktree: worktreeOf(j),
		Job:      domain.EventJob{Name: spec.Job.Name, Kind: spec.Job.Kind, URL: spec.URL(d.Proxy().PublicPort), Hold: j.record.Hold()},
		ExitCode: code,
	}
	if spec.Job.Scope == project.Shared && !j.record.Claim {
		d.mu.Lock()
		change.HeldBy = d.holdersLocked(j)
		d.mu.Unlock()
	}

	return change
}

// worktreeOf is the worktree a job event names for j.
func worktreeOf(j *job) domain.JobWorktree {
	spec := j.record.Spec
	branch := ""
	if spec.Origin != nil {
		branch = spec.Origin.Branch
	}
	return domain.JobWorktree{Branch: branch, Path: spec.WorkDir}
}

// ----------------------------------------------------------------------------
// Helpers

func keyOf(spec run.Spec) key {
	return key{workDir: spec.WorkDir, name: spec.Job.Name}
}

func closed() chan struct{} {
	done := make(chan struct{})
	close(done)
	return done
}
