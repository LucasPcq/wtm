// Package daemon is the run daemon: the jobs of every worktree of the machine, started, watched and stopped as its clients ask, recorded for the next daemon and taken over from the one before. It is blind to git: a worktree is the directory and the environment its client sends.
package daemon

import (
	"cmp"
	"context"
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
	// lastLines is how much of its output a job.crashed carries.
	lastLines = 10
	// historyBytes is how much of a job's raw output a client attaching to its terminal is replayed: enough for a terminal to draw the screen again, escapes included.
	historyBytes = 1 << 20
	followQueue  = 1024
	maxWindow    = 1<<16 - 1
)

// Params is what a [Daemon] reaches the world through: how it starts a
// process, finds orphans, keeps its index, keeps each job's log and reads it
// back, and tells whether a worktree is still there; where it publishes its
// job events (nil publishes none); its clock (time.Now when nil); and how long
// a stopped job has to exit before it is killed (5 seconds when zero).
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
}

// A Daemon holds the jobs of the machine. Its methods are safe for concurrent
// use, one per client request.
type Daemon struct {
	params Params

	mu       sync.Mutex
	jobs     map[key]*job
	stopping bool

	// saving serializes the writes of the index, each from a snapshot taken under it: a slow write never lands after a newer one.
	saving sync.Mutex
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
	last     *run.Line
	previous *job
	stopped  bool
	stopID   string
	done     chan struct{}
}

// New returns a daemon over params, holding no job until [Daemon.Adopt].
func New(params Params) *Daemon {
	if params.Now == nil {
		params.Now = time.Now
	}
	params.Grace = cmp.Or(params.Grace, stopGrace)

	return &Daemon{params: params, jobs: map[key]*job{}}
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

	var reap []int
	d.mu.Lock()
	for _, record := range records {
		decision := run.Reconcile(run.ReconcileParams{Record: record, Group: groups[record.PGID], WorkDirExists: d.params.Exists(record.Spec.WorkDir)})
		k := keyOf(record.Spec)
		if !decision.Keep || d.jobs[k] != nil {
			continue
		}
		if decision.Reaped {
			reap = append(reap, record.PGID)
		}
		j := &job{record: record, state: decision.State, reaped: decision.Reaped, adopted: true, done: closed()}
		if last := d.readLog(run.LogFile{Dir: record.Spec.LogDir, Job: record.Spec.Job.Name}, 1); len(last) > 0 {
			j.last = &last[0]
		}
		d.jobs[k] = j
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
func (d *Daemon) Start(spec run.Spec, out func(run.Line)) (run.Job, error) {
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
	j := &job{record: run.Record{Spec: spec, StartedAt: d.params.Now(), PID: process.PID(), PGID: process.PID()}, state: run.Running, process: process, output: output, done: make(chan struct{})}
	if previous != nil && previous.up() && spec.Detached() {
		j.previous = previous
	}
	d.jobs[k] = j
	d.mu.Unlock()

	switch {
	case spec.Job.Kind == project.Task:
		d.publish(j, publishParams{Event: domain.JobStarted{JobChange: j.change(nil)}})
		return d.runTask(j)
	case spec.Detached():
		return d.launch(j)
	}
	d.publish(j, publishParams{Event: domain.JobStarted{JobChange: j.change(nil)}})
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
	case stopped && j.previous != nil:
		d.jobs[k] = j.previous
	case stopped:
	case code != 0:
		j.exitCode = &code
		delete(d.jobs, k)
		if j.previous != nil {
			d.jobs[k] = j.previous
		}
	}
	j.previous = nil
	d.mu.Unlock()
	defer close(j.done)
	if !stopped && code != 0 {
		return d.view(j), &run.Error{Code: run.CodeLaunchFailed, Message: fmt.Sprintf("the launcher of %s exited with code %d", j.record.Spec.Job.Name, code), ExitCode: &code}
	}

	d.save()
	if !stopped {
		d.publish(j, publishParams{Event: domain.JobStarted{JobChange: j.change(nil)}})
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
	d.mu.Unlock()

	// Closed once announced: a stop returns with the job.stopped on the bus and the index written.
	defer close(j.done)
	d.save()
	switch state {
	case run.Stopped:
		d.publish(j, publishParams{Event: domain.JobStopped{JobChange: j.change(nil)}, CorrelationID: stopID})
	case run.Exited:
		d.publish(j, publishParams{Event: domain.JobExited{JobChange: j.change(&code)}})
	default:
		change := j.change(&code)
		change.LastLines = last
		d.publish(j, publishParams{Event: domain.JobCrashed{JobChange: change}})
	}
}

// ----------------------------------------------------------------------------
// Stop

// Stop stops a job: a process of the daemon's has its group stopped, a
// detached stack runs its stop command (failing with [run.CodeStopFailed] when
// it exits non-zero), a claim is let go. A job not up is [run.OutcomeNotRunning],
// and one that ended leaves the listing.
func (d *Daemon) Stop(req run.Stop) (run.Outcome, error) {
	k := key{workDir: req.WorkDir, name: req.Name}
	d.mu.Lock()
	j := d.jobs[k]
	if j == nil || !j.up() {
		delete(d.jobs, k)
		d.mu.Unlock()
		return run.OutcomeNotRunning, nil
	}
	j.stopped, j.stopID = true, req.CorrelationID
	process, done, previous := j.process, j.done, j.previous
	if j.record.Claim {
		delete(d.jobs, k)
	}
	d.mu.Unlock()

	switch {
	case j.record.Claim:
		d.save()
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
		stack.stopped, stack.stopID = true, req.CorrelationID
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
// stop command that fails leaves it up and indexed.
func (d *Daemon) stopStack(j *job) error {
	spec := j.record.Spec
	process, err := d.params.Spawn(spec.StopCommand(nil))
	code := -1
	if err == nil {
		code = process.Wait()
	}
	if code != 0 {
		d.mu.Lock()
		j.stopped = false
		d.mu.Unlock()
		return &run.Error{Code: run.CodeStopFailed, Message: fmt.Sprintf("the stop command of %s failed (exit %d)", spec.Job.Name, code), ExitCode: &code}
	}

	d.mu.Lock()
	j.state = run.Stopped
	d.mu.Unlock()
	d.save()
	d.publish(j, publishParams{Event: domain.JobStopped{JobChange: j.change(nil)}, CorrelationID: j.stopID})

	return nil
}

// StopAll stops every job up in req's worktree, every job of the machine when
// it names none, together, and returns each as it stood with what the stop
// did; it fails with the first stop that failed, once every stop ended.
func (d *Daemon) StopAll(req run.StopAll) ([]run.StoppedJob, error) {
	var reached []run.StoppedJob
	for _, view := range d.List(req.WorkDir) {
		if view.Up {
			reached = append(reached, run.StoppedJob{Job: view})
		}
	}

	errs := make([]error, len(reached))
	var stopping sync.WaitGroup
	for i := range reached {
		stopping.Go(func() {
			reached[i].Outcome, errs[i] = d.Stop(run.Stop{Name: reached[i].Job.Name, WorkDir: reached[i].Job.WorkDir, CorrelationID: req.CorrelationID})
		})
	}
	stopping.Wait()

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
	d.stopping = true
	var running []run.Stop
	for k, j := range d.jobs {
		if j.process != nil {
			running = append(running, run.Stop{Name: k.name, WorkDir: k.workDir})
		}
	}
	d.mu.Unlock()

	var stopping sync.WaitGroup
	for _, req := range running {
		stopping.Go(func() { _, _ = d.Stop(req) })
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

	past, lines, stop, ended := j.output.follow(n, req.Follow)
	defer stop()
	for _, line := range past {
		out(line)
	}
	for lines != nil {
		select {
		case line, open := <-lines:
			if !open {
				ended, lines = true, nil
				continue
			}
			out(line)
		case <-ctx.Done():
			return d.view(j), ctx.Err()
		}
	}
	if ended {
		// The process ended: what the listing says of it is settled once done is closed.
		<-j.done
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
	if j.record.Spec.Command(nil).Terminal {
		session.Input = process
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

// find returns the job of k, failing with [run.CodeJobNotFound] when the
// daemon does not know it.
func (d *Daemon) find(k key) (*job, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	j := d.jobs[k]
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
	j := d.jobs[k]
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
	d.mu.Lock()
	defer d.mu.Unlock()
	jobs := []run.Job{}
	for k, j := range d.jobs {
		if workDir == "" || k.workDir == workDir {
			jobs = append(jobs, j.viewLocked())
		}
	}
	slices.SortFunc(jobs, func(a, b run.Job) int { return cmp.Or(cmp.Compare(a.WorkDir, b.WorkDir), cmp.Compare(a.Name, b.Name)) })

	return jobs
}

// Busy reports whether a process of the daemon's runs: while one does, the
// daemon must not stop on its own.
func (d *Daemon) Busy() bool {
	d.mu.Lock()
	defer d.mu.Unlock()

	for _, j := range d.jobs {
		if j.process != nil {
			return true
		}
	}

	return false
}

func (d *Daemon) view(j *job) run.Job {
	d.mu.Lock()
	defer d.mu.Unlock()

	return j.viewLocked()
}

// viewLocked is the job as a client reads it; the daemon's lock is held.
func (j *job) viewLocked() run.Job {
	spec := j.record.Spec
	view := run.Job{
		Name:      spec.Job.Name,
		WorkDir:   spec.WorkDir,
		Up:        j.up(),
		Kind:      spec.Job.Kind,
		State:     j.state,
		Hold:      j.record.Hold(),
		Reaped:    j.reaped,
		URL:       spec.URL(),
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

// change is what a job event says of j, ended with code (nil for none).
func (j *job) change(code *int) domain.JobChange {
	spec := j.record.Spec
	branch := ""
	if spec.Origin != nil {
		branch = spec.Origin.Branch
	}

	return domain.JobChange{
		Worktree: domain.JobWorktree{Branch: branch, Path: spec.WorkDir},
		Job:      domain.EventJob{Name: spec.Job.Name, Kind: spec.Job.Kind, URL: spec.URL(), Hold: j.record.Hold()},
		ExitCode: code,
	}
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

// An output is what a job prints, as it comes: its last lines, cut and
// timestamped, handed to the start's client while it waits (out), written to
// its log and to each follower; its last raw bytes, handed to each client
// attached to its terminal. A follower or a client attached that falls behind
// is let go. Safe for its writer, its readers and its end at once.
type output struct {
	mu        sync.Mutex
	now       func() time.Time
	splitter  run.Splitter
	lines     []run.Line
	raw       []byte
	log       Log
	out       func(run.Line)
	followers map[chan run.Line]struct{}
	attached  map[chan []byte]struct{}
	ended     bool
}

func newOutput(now func() time.Time) *output {
	return &output{now: now, followers: map[chan run.Line]struct{}{}, attached: map[chan []byte]struct{}{}}
}

func (o *output) write(chunk []byte) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.raw = append(o.raw, chunk...)
	if extra := len(o.raw) - historyBytes; extra > 0 {
		o.raw = append([]byte(nil), o.raw[extra:]...)
	}
	for ch := range o.attached {
		select {
		case ch <- chunk:
		default:
			delete(o.attached, ch)
			close(ch)
		}
	}
	o.emit(o.splitter.Split(chunk, o.now()))
}

// end gives out the line held with no newline, ends every follower and client
// attached, closes the log, and returns the texts of the last lines a
// job.crashed carries.
func (o *output) end() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.emit(o.splitter.Flush(o.now()))
	o.ended = true
	// No client attaches to a job that ended: its raw history goes, its lines stay for a tail.
	o.raw = nil
	for ch := range o.followers {
		close(ch)
	}
	for ch := range o.attached {
		close(ch)
	}
	clear(o.followers)
	clear(o.attached)
	if o.log != nil {
		o.log.Close()
	}

	last := []string{}
	for _, line := range o.lines[max(0, len(o.lines)-lastLines):] {
		last = append(last, line.Text)
	}
	return last
}

func (o *output) emit(lines []run.Line) {
	if len(lines) == 0 {
		return
	}
	o.lines = append(o.lines, lines...)
	if extra := len(o.lines) - run.MaxTail; extra > 0 {
		o.lines = append([]run.Line(nil), o.lines[extra:]...)
	}
	if o.log != nil {
		o.log.Write(lines)
	}
	for _, line := range lines {
		if o.out != nil {
			o.out(line)
		}
		for ch := range o.followers {
			select {
			case ch <- line:
			default:
				delete(o.followers, ch)
				close(ch)
			}
		}
	}
}

func (o *output) lastLine() *run.Line {
	o.mu.Lock()
	defer o.mu.Unlock()
	if len(o.lines) == 0 {
		return nil
	}
	last := o.lines[len(o.lines)-1]
	return &last
}

// follow returns the last n lines and, when live, the channel each new one
// arrives on, closed when the job ends or the follower falls behind, with the
// function that lets it go; whether the job has ended.
func (o *output) follow(n int, live bool) ([]run.Line, chan run.Line, func(), bool) {
	o.mu.Lock()
	defer o.mu.Unlock()
	past := slices.Clone(o.lines[max(0, len(o.lines)-n):])
	if !live || o.ended {
		return past, nil, func() {}, o.ended
	}
	ch := make(chan run.Line, followQueue)
	o.followers[ch] = struct{}{}
	return past, ch, func() { o.drop(ch) }, false
}

// attach returns what the job printed last, raw, and the channel what it
// prints next arrives on, with the function that lets it go.
func (o *output) attach() ([]byte, chan []byte, func()) {
	o.mu.Lock()
	defer o.mu.Unlock()
	ch := make(chan []byte, followQueue)
	if o.ended {
		close(ch)
	} else {
		o.attached[ch] = struct{}{}
	}
	return slices.Clone(o.raw), ch, func() {
		o.mu.Lock()
		defer o.mu.Unlock()
		if _, open := o.attached[ch]; open {
			delete(o.attached, ch)
			close(ch)
		}
	}
}

func (o *output) drop(ch chan run.Line) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if _, open := o.followers[ch]; open {
		delete(o.followers, ch)
		close(ch)
	}
}
