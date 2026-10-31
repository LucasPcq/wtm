// Package daemon is the run daemon: the jobs of every worktree of the machine, started, watched and stopped as its clients ask, recorded for the next daemon and taken over from the one before. It is blind to git: a worktree is the directory and the environment its client sends.
package daemon

import (
	"cmp"
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
)

// Params is what a [Daemon] reaches the world through: how it starts a
// process, finds orphans, keeps its index and tells whether a worktree is
// still there; where it publishes its job events (nil publishes none); its
// clock (time.Now when nil); and how long a stopped job has to exit before it
// is killed (5 seconds when zero).
type Params struct {
	Spawn   Spawn
	Orphans Orphans
	Index   Index
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
// ended; its process while one of the daemon's runs it; whether a stop reached
// it and for which command; and done, closed once the end of its process is
// settled.
type job struct {
	record   run.Record
	state    run.State
	reaped   bool
	exitCode *int
	process  Process
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
	records, _ := d.params.Index.Load()
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
		d.jobs[k] = &job{record: record, state: decision.State, reaped: decision.Reaped, done: closed()}
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
	output := &tail{now: d.params.Now}
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
	process, err := d.params.Spawn(spec.Command(output.write))
	if err != nil {
		d.mu.Unlock()
		return run.Job{}, &run.Error{Code: run.CodeLaunchFailed, Message: fmt.Sprintf("start job %s: %v", spec.Job.Name, err)}
	}
	j := &job{record: run.Record{Spec: spec, StartedAt: d.params.Now(), PID: process.PID(), PGID: process.PID()}, state: run.Running, process: process, done: make(chan struct{})}
	d.jobs[k] = j
	d.mu.Unlock()

	switch {
	case spec.Job.Kind == project.Task:
		d.publish(j, publishParams{Event: domain.JobStarted{JobChange: j.change(nil)}})
		return d.runTask(j, output)
	case spec.Detached():
		return d.launch(launchParams{Job: j, Previous: previous, Output: output})
	}
	d.publish(j, publishParams{Event: domain.JobStarted{JobChange: j.change(nil)}})
	d.save()
	go d.watch(j, output)

	return d.view(j), nil
}

// runTask waits for a task to end, settles it and returns it, with the
// failure of a non-zero exit nobody asked for.
func (d *Daemon) runTask(j *job, output *tail) (run.Job, error) {
	d.settle(j, settleParams{Code: j.process.Wait(), Output: output})
	view := d.view(j)
	if view.State != run.Crashed {
		return view, nil
	}

	return view, &run.Error{Code: run.CodeTaskFailed, Message: fmt.Sprintf("task %s exited with code %d", view.Name, *view.ExitCode), ExitCode: view.ExitCode}
}

// launchParams is a detached launcher started, the job it replaces in its
// worktree (a stack up whose launcher runs again, nil for none), and its
// output.
type launchParams struct {
	Job      *job
	Previous *job
	Output   *tail
}

// launch waits for a detached launcher to exit: its stack is then up, with no
// process of the daemon's; a launcher that failed gives the worktree back the
// stack it had, if any. A stop that reached the launcher has the last word.
func (d *Daemon) launch(params launchParams) (run.Job, error) {
	j := params.Job
	code := j.process.Wait()
	params.Output.flush()

	d.mu.Lock()
	j.process = nil
	stopped := j.stopped
	switch {
	case stopped:
		j.state = run.Stopped
	case code != 0:
		j.exitCode = &code
		k := keyOf(j.record.Spec)
		delete(d.jobs, k)
		if params.Previous != nil {
			d.jobs[k] = params.Previous
		}
	}
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

// watch settles a foreground service once its process ends.
func (d *Daemon) watch(j *job, output *tail) {
	d.settle(j, settleParams{Code: j.process.Wait(), Output: output})
}

// settleParams is how a job's process exited, and the output it printed.
type settleParams struct {
	Code   int
	Output *tail
}

// settle records the end of a job's process, writes the index and publishes
// it: stopped when a stop reached it, exited for a task that succeeded,
// crashed otherwise (a service never ends on its own).
func (d *Daemon) settle(j *job, params settleParams) {
	last := params.Output.flush()

	d.mu.Lock()
	j.process = nil
	j.exitCode = &params.Code
	switch {
	case j.stopped:
		j.state = run.Stopped
	case params.Code == 0 && j.record.Spec.Job.Kind == project.Task:
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
		d.publish(j, publishParams{Event: domain.JobExited{JobChange: j.change(&params.Code)}})
	default:
		change := j.change(&params.Code)
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
	process, done := j.process, j.done
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
	default:
		if err := d.stopStack(j); err != nil {
			return "", err
		}
	}

	return run.OutcomeStopped, nil
}

// stopStack runs a detached stack's stop command, then lists it stopped.
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
		Claim:     j.record.Claim,
		Detached:  spec.Detached(),
		Reaped:    j.reaped,
		URL:       spec.URL(),
		Ports:     spec.Ports(),
		ExitCode:  j.exitCode,
		SharedDir: j.record.SharedDir,
	}
	if view.Up {
		view.StartedAt = j.record.StartedAt
	}
	// A detached stack's launcher is gone and a claim runs nothing: a pid would point at nothing, or in time at a stranger.
	if !view.Claim && !view.Detached {
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
// launcher still running.
func (d *Daemon) save() {
	d.saving.Lock()
	defer d.saving.Unlock()

	d.mu.Lock()
	var records []run.Record
	for _, j := range d.jobs {
		spec := j.record.Spec
		kept := j.record.Claim || (j.state == run.Running && (j.record.Foreground() || spec.Detached() && j.process == nil))
		if kept {
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
		Job:      domain.EventJob{Name: spec.Job.Name, Kind: spec.Job.Kind, URL: spec.URL(), Detached: spec.Detached()},
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

// A tail cuts a job's output into lines as it comes, hands each to out and
// keeps the last ones; safe for its writer and a flush at once.
type tail struct {
	mu       sync.Mutex
	splitter run.Splitter
	last     []string
	out      func(run.Line)
	now      func() time.Time
}

func (t *tail) write(chunk []byte) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.emit(t.splitter.Split(chunk, t.now()))
}

// flush gives out the line held with no newline and returns the last lines.
func (t *tail) flush() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.emit(t.splitter.Flush(t.now()))

	return slices.Clone(t.last)
}

func (t *tail) emit(lines []run.Line) {
	for _, line := range lines {
		t.last = append(t.last, line.Text)
		if len(t.last) > lastLines {
			t.last = t.last[1:]
		}
		if t.out != nil {
			t.out(line)
		}
	}
}
