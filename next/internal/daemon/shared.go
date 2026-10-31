package daemon

import (
	"cmp"
	"errors"
	"fmt"
	"maps"
	"slices"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

const (
	// namespaceBudget bounds the retries of a namespace's create: its service was started moments ago, and a first refusal means it does not accept connections yet far more often than a wrong command.
	namespaceBudget = 30 * time.Second
	namespaceRetry  = time.Second
	// namespaceHang bounds one attempt of a create, far past the budget: a database made from a large template takes minutes.
	namespaceHang = 5 * time.Minute
	// pendingPoll is how often a start waiting on a shared job being stopped looks again: a stack's stop command has no channel to wait on.
	pendingPoll = 10 * time.Millisecond
)

// startShared starts a job of shared scope: it runs once, in the main
// checkout spec.Shared names, under that checkout's key, and every other
// worktree asking for it posts a claim beside it: the job table is its
// reference count. The main checkout's own hold is a flag of the job itself
// (MainHolds). Losing the race to another worktree starting it is no failure.
// Once held, the job's namespace is created for the worktree asking, its
// create retried while the service comes up; a namespace that cannot be made
// takes the hold back, and the job with it when nobody else holds it.
func (d *Daemon) startShared(spec run.Spec, out func(run.Line)) (run.Job, error) {
	shared := spec.Shared
	if shared == nil || shared.WorkDir == "" {
		return run.Job{}, &run.Error{Code: run.CodeNoMainCheckout, Message: fmt.Sprintf("job %s is shared, and no main checkout was named to run it in", spec.Job.Name)}
	}
	own, served := keyOf(spec), key{workDir: shared.WorkDir, name: spec.Job.Name}
	mainAsks := own == served

	runs := spec
	runs.WorkDir, runs.Env, runs.LogDir = shared.WorkDir, shared.Env, shared.LogDir
	if spec.Origin != nil {
		origin := *spec.Origin
		origin.Branch = shared.Env[env.BranchKey]
		runs.Origin = &origin
	}
	var service, hold *job
	mine, launched := 0, false
	for service == nil {
		d.mu.Lock()
		if d.stopping {
			d.mu.Unlock()
			return run.Job{}, &run.Error{Code: run.CodeStopping, Message: "the run daemon is stopping"}
		}
		if held := d.jobs[own]; held != nil && held.up() && (!mainAsks || held.record.MainHolds) {
			d.mu.Unlock()
			return run.Job{}, &run.Error{Code: run.CodeJobRunning, Message: fmt.Sprintf("job %s is already running in %s", spec.Job.Name, spec.WorkDir)}
		}
		// A job being stopped, or a stack whose first launch runs, is waited for: a claim on it would hold a job about to be gone.
		if pending := d.jobs[served]; pending != nil && pending.up() && (pending.stopped || pending.process != nil && pending.record.Spec.Detached()) {
			// A stack's done closed once its launcher ended: only a process's tells when it ends.
			var ended <-chan struct{}
			if pending.process != nil {
				ended = pending.done
			}
			d.mu.Unlock()
			if d.waited != nil {
				d.waited()
			}
			select {
			case <-ended:
			case <-time.After(pendingPoll):
			}
			continue
		}
		// Posted under the lock that found the job up: a release deciding nobody holds it marks it stopping under the same lock.
		if service = d.realLocked(served); service != nil {
			hold = service
			if mainAsks {
				service.record.MainHolds = true
				service.mainHold++
				mine = service.mainHold
			} else {
				hold = &job{record: run.Record{Spec: spec, StartedAt: d.params.Now(), Claim: true, SharedDir: shared.WorkDir}, done: closed(), unannounced: true}
				d.jobs[own] = hold
			}
			d.mu.Unlock()
			break
		}
		// This start launched it, and it is gone already: a task that ended, a service that died at once.
		if launched {
			d.mu.Unlock()
			return run.Job{}, &run.Error{Code: run.CodeLaunchFailed, Message: fmt.Sprintf("job %s ended as soon as it started", spec.Job.Name)}
		}
		d.mu.Unlock()

		// Started, or started by another worktree meanwhile: looked at again.
		var refusal *run.Error
		_, err := d.start(runs, out)
		if err != nil && !(errors.As(err, &refusal) && refusal.Code == run.CodeJobRunning) {
			return run.Job{}, err
		}
		launched = err == nil
	}

	// The job may have ended, or this worktree let go of it, while its namespace was made: the hold this start took is gone then. Read under the lock.
	stands := func() bool {
		return d.jobs[own] == hold && hold.up() && (!mainAsks || hold.record.MainHolds && hold.mainHold == mine)
	}
	if err := d.createNamespace(spec, service); err != nil {
		_, _ = d.letGo(letGoParams{Job: hold, Stands: stands})
		return run.Job{}, err
	}
	d.mu.Lock()
	held := stands()
	hold.unannounced = false
	d.mu.Unlock()
	if !held {
		return run.Job{}, &run.Error{Code: run.CodeLaunchFailed, Message: fmt.Sprintf("job %s ended, or was let go, while its namespace was made", spec.Job.Name)}
	}
	d.save()
	if !mainAsks {
		d.publish(hold, publishParams{Event: domain.JobStarted{JobChange: d.change(hold, nil)}})
	}

	return d.view(hold), nil
}

// createNamespace creates the namespace of spec's job for its worktree, if it
// has one, retrying its create while its budget lasts, each attempt stopped
// once it hangs, and notes it in the log of the service that serves it.
func (d *Daemon) createNamespace(spec run.Spec, service *job) error {
	namespace, ok, err := env.NamespaceFor(spec.Job, spec.Env)
	if err != nil {
		return &run.Error{Code: run.CodeNamespace, Message: err.Error()}
	}
	if !ok {
		return nil
	}

	command := spec.Command(nil)
	command.Line, command.Dir, command.Terminal = spec.Job.Namespace.Create, spec.WorkDir, false
	command.Env = maps.Clone(command.Env)
	maps.Copy(command.Env, namespace.Env)
	// Wall-clock time, not the daemon's clock, which stamps what it records. The budget bounds when an attempt starts, not how long it runs.
	retries := time.Now().Add(cmp.Or(d.params.NamespaceBudget, namespaceBudget))
	retry := cmp.Or(d.params.NamespaceRetry, namespaceRetry)
	why := ""
	for {
		if d.isStopping() {
			return &run.Error{Code: run.CodeStopping, Message: "the run daemon is stopping"}
		}
		said := newOutput(d.params.Now)
		command.Output = said.write
		code, err := d.runUntil(command, namespaceHang)
		last := said.end()
		if err == nil && code == 0 {
			break
		}
		// The last attempt's words, else an earlier one's: one stopped may have said nothing.
		switch {
		case len(last) > 0:
			why = last[len(last)-1]
		case err != nil && !d.isStopping():
			why = fmt.Sprintf("still running after %s, stopped", namespaceHang)
		}
		// No attempt starts once the budget is spent.
		if time.Until(retries) <= retry {
			why = cmp.Or(why, fmt.Sprintf("exit %d", code))
			return &run.Error{Code: run.CodeNamespace, Message: fmt.Sprintf("job %s: could not create namespace %s: %s (the create runs on every start, so it must be safe to run again)", spec.Job.Name, namespace.Name, why), ExitCode: &code}
		}
		select {
		case <-time.After(retry):
		case <-d.quit:
		}
	}

	if service.output != nil {
		service.output.note(fmt.Sprintf("[wtm] namespace %s ready for %s", namespace.Name, spec.Env[env.WorktreeKey]))
	}
	return nil
}

// errDeadline is the error of a command stopped once it ran too long.
var errDeadline = errors.New("stopped once it ran too long")

// runUntil runs command and returns its exit code; one still running after
// limit, or once the daemon stops, is stopped, its whole group, and fails with
// errDeadline: a create that hangs never holds its start, nor the daemon's
// shutdown.
func (d *Daemon) runUntil(command run.Command, limit time.Duration) (int, error) {
	process, err := d.params.Spawn(command)
	if err != nil {
		return -1, err
	}
	exited := make(chan int, 1)
	go func() { exited <- process.Wait() }()
	select {
	case code := <-exited:
		return code, nil
	case <-d.params.After(limit):
	case <-d.quit:
	}
	select {
	case code := <-exited:
		// It ended as its limit fell.
		return code, nil
	default:
	}
	process.Stop(d.params.Grace)
	return <-exited, errDeadline
}

// release lets a worktree go of the shared job j (a claim, or the job itself
// when its main checkout lets go), for the command of correlation: the job
// stops only once nobody holds it, no claim left and its main checkout not
// holding it, [run.OutcomeReleased] otherwise. Its namespace stays: stopping
// is not destroying.
func (d *Daemon) release(j *job, correlation string) (run.Outcome, error) {
	return d.letGo(letGoParams{Job: j, CorrelationID: correlation})
}

// letGoParams is a hold let go of, for the command of CorrelationID; Stands,
// when set, tells under the lock whether the hold is still the caller's to
// let go.
type letGoParams struct {
	Job           *job
	CorrelationID string
	Stands        func() bool
}

// letGo lets go of a hold on a shared job, as release says.
func (d *Daemon) letGo(params letGoParams) (run.Outcome, error) {
	j, correlation := params.Job, params.CorrelationID
	if j == nil {
		return run.OutcomeNotRunning, nil
	}
	name := j.record.Spec.Job.Name
	d.mu.Lock()
	if params.Stands != nil && !params.Stands() {
		d.mu.Unlock()
		return run.OutcomeNotRunning, nil
	}
	announce := j.record.Claim && !j.unannounced
	served := key{workDir: j.record.SharedDir, name: name}
	if j.record.Claim {
		// Gone already, let go of by another release, or replaced by a newer claim of the same worktree.
		k := keyOf(j.record.Spec)
		if d.jobs[k] != j {
			d.mu.Unlock()
			return run.OutcomeNotRunning, nil
		}
		delete(d.jobs, k)
	} else {
		j.record.MainHolds = false
		served = keyOf(j.record.Spec)
	}
	service := d.realLocked(served)
	held := service != nil && (service.record.MainHolds || d.claimsLocked(served) > 0)
	// Marked stopping under the lock that found nobody holding it: no claim lands on it now. A daemon stopping halts, never releases: the job stays for the next one.
	halting := service != nil && !held && !d.stopping
	if halting {
		service.stopped, service.stopID = true, correlation
	}
	d.mu.Unlock()

	d.save()
	if announce {
		d.publish(j, publishParams{Event: domain.JobStopped{JobChange: d.change(j, nil)}, CorrelationID: correlation})
	}
	switch {
	case service == nil:
		return run.OutcomeStopped, nil
	case !halting:
		return run.OutcomeReleased, nil
	}

	outcome, err := d.halt(service, correlation)
	if outcome == run.OutcomeNotRunning {
		// It ended on its own since it was marked: let go all the same.
		return run.OutcomeStopped, err
	}
	return outcome, err
}

// realLocked returns the shared job k names when it is up, the job itself and
// not a claim, nor one whose stop runs; the lock is held.
func (d *Daemon) realLocked(k key) *job {
	j := d.jobs[k]
	if j == nil || j.record.Claim || !j.up() || j.stopped {
		return nil
	}
	return j
}

// claimsLocked counts the claims standing on the shared job k names; the lock
// is held.
func (d *Daemon) claimsLocked(k key) int {
	count := 0
	for at, j := range d.jobs {
		if j.record.Claim && at.name == k.name && j.record.SharedDir == k.workDir {
			count++
		}
	}
	return count
}

// holdersLocked returns the worktrees holding the shared job j: its main
// checkout when it holds it, and each worktree claiming it; the lock is held.
func (d *Daemon) holdersLocked(j *job) []domain.JobWorktree {
	k := keyOf(j.record.Spec)
	var holders []domain.JobWorktree
	if j.record.MainHolds {
		holders = append(holders, worktreeOf(j))
	}
	for at, claim := range d.jobs {
		if claim.record.Claim && at.name == k.name && claim.record.SharedDir == k.workDir {
			holders = append(holders, worktreeOf(claim))
		}
	}
	slices.SortFunc(holders, func(a, b domain.JobWorktree) int { return cmp.Compare(a.Path, b.Path) })
	return holders
}

// heldLocked returns the worktrees holding j when it is a shared job, nil
// otherwise; the lock is held.
func (d *Daemon) heldLocked(j *job) []domain.JobWorktree {
	if j.record.Spec.Job.Scope != project.Shared || j.record.Claim {
		return nil
	}
	return d.holdersLocked(j)
}

// dropDanglingClaimsLocked drops the claims on a shared job that is not up, a
// daemon taken over from having lost it: a claim pointing at nothing would
// tell the next worktree its service runs. The lock is held.
func (d *Daemon) dropDanglingClaimsLocked() {
	for at, j := range d.jobs {
		if j.record.Claim && d.realLocked(key{workDir: j.record.SharedDir, name: at.name}) == nil {
			delete(d.jobs, at)
		}
	}
}

// isShared reports whether the job of k is of shared scope.
func (d *Daemon) isShared(k key) bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	j := d.jobs[k]
	return j != nil && j.record.Spec.Job.Scope == project.Shared
}

// jobOf returns the job of k, nil when there is none.
func (d *Daemon) jobOf(k key) *job {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.jobs[k]
}

// sharedDir is the main checkout a job of spec runs in, when it is shared.
func sharedDir(spec run.Spec) string {
	if spec.Job.Scope != project.Shared || spec.Shared == nil {
		return ""
	}
	return spec.Shared.WorkDir
}
