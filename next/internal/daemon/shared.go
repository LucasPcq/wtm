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

	d.mu.Lock()
	if d.stopping {
		d.mu.Unlock()
		return run.Job{}, &run.Error{Code: run.CodeStopping, Message: "the run daemon is stopping"}
	}
	if held := d.jobs[own]; held != nil && held.up() && (!mainAsks || held.record.MainHolds) {
		d.mu.Unlock()
		return run.Job{}, &run.Error{Code: run.CodeJobRunning, Message: fmt.Sprintf("job %s is already running in %s", spec.Job.Name, spec.WorkDir)}
	}
	up := d.realLocked(served) != nil
	d.mu.Unlock()

	if !up {
		runs := spec
		runs.WorkDir, runs.Env, runs.LogDir = shared.WorkDir, shared.Env, shared.LogDir
		if spec.Origin != nil {
			origin := *spec.Origin
			origin.Branch = shared.Env[env.BranchKey]
			runs.Origin = &origin
		}
		var refusal *run.Error
		if _, err := d.start(runs, out); err != nil && !(errors.As(err, &refusal) && refusal.Code == run.CodeJobRunning) {
			return run.Job{}, err
		}
	}

	d.mu.Lock()
	service := d.realLocked(served)
	if service == nil {
		d.mu.Unlock()
		return run.Job{}, &run.Error{Code: run.CodeLaunchFailed, Message: fmt.Sprintf("the shared job %s is not up in %s", spec.Job.Name, shared.WorkDir)}
	}
	if mainAsks {
		service.record.MainHolds = true
	} else {
		d.jobs[own] = &job{record: run.Record{Spec: spec, StartedAt: d.params.Now(), Claim: true, SharedDir: shared.WorkDir}, done: closed()}
	}
	d.mu.Unlock()

	if err := d.createNamespace(spec, service); err != nil {
		// Never announced, the hold leaves in silence.
		_, _ = d.letGo(letGoParams{Job: d.jobOf(own)})
		return run.Job{}, err
	}
	d.save()
	claim := d.jobOf(own)
	if !mainAsks {
		d.publish(claim, publishParams{Event: domain.JobStarted{JobChange: d.change(claim, nil)}})
	}

	return d.view(claim), nil
}

// createNamespace creates the namespace of spec's job for its worktree, if it
// has one, retrying its create within a budget, and notes it in the log of the
// service that serves it.
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
	// A budget of wall-clock time, not the daemon's clock, which stamps what it records.
	deadline := time.Now().Add(cmp.Or(d.params.NamespaceBudget, namespaceBudget))
	for {
		said := newOutput(d.params.Now)
		command.Output = said.write
		code, err := d.run(command)
		last := said.end()
		if err == nil && code == 0 {
			break
		}
		if time.Now().After(deadline) {
			why := fmt.Sprintf("exit %d", code)
			if len(last) > 0 {
				why = last[len(last)-1]
			}
			return &run.Error{Code: run.CodeNamespace, Message: fmt.Sprintf("job %s: could not create namespace %s: %s (the create runs on every start, so it must be safe to run again)", spec.Job.Name, namespace.Name, why), ExitCode: &code}
		}
		time.Sleep(cmp.Or(d.params.NamespaceRetry, namespaceRetry))
	}

	if service.output != nil {
		service.output.note(fmt.Sprintf("[wtm] namespace %s ready for %s", namespace.Name, spec.Env[env.WorktreeKey]))
	}
	return nil
}

// run runs command to its end and returns its exit code.
func (d *Daemon) run(command run.Command) (int, error) {
	process, err := d.params.Spawn(command)
	if err != nil {
		return -1, err
	}
	return process.Wait(), nil
}

// release lets a worktree go of the shared job j (a claim, or the job itself
// when its main checkout lets go), for the command of correlation: the job
// stops only once nobody holds it, no claim left and its main checkout not
// holding it, [run.OutcomeReleased] otherwise. Its namespace stays: stopping
// is not destroying.
func (d *Daemon) release(j *job, correlation string) (run.Outcome, error) {
	return d.letGo(letGoParams{Job: j, CorrelationID: correlation, Announce: true})
}

// letGoParams is a hold let go of, for the command of CorrelationID, and
// whether a claim's release is announced: not for one never announced.
type letGoParams struct {
	Job           *job
	CorrelationID string
	Announce      bool
}

// letGo lets go of a hold on a shared job, as release says.
func (d *Daemon) letGo(params letGoParams) (run.Outcome, error) {
	j, correlation := params.Job, params.CorrelationID
	if j == nil {
		return run.OutcomeNotRunning, nil
	}
	name := j.record.Spec.Job.Name
	d.mu.Lock()
	served := key{workDir: j.record.SharedDir, name: name}
	if j.record.Claim {
		delete(d.jobs, keyOf(j.record.Spec))
	} else {
		j.record.MainHolds = false
		served = keyOf(j.record.Spec)
	}
	service := d.realLocked(served)
	held := service != nil && (service.record.MainHolds || d.claimsLocked(served) > 0)
	d.mu.Unlock()

	d.save()
	if j.record.Claim && params.Announce {
		d.publish(j, publishParams{Event: domain.JobStopped{JobChange: d.change(j, nil)}, CorrelationID: correlation})
	}
	switch {
	case service == nil:
		return run.OutcomeStopped, nil
	case held:
		return run.OutcomeReleased, nil
	}

	return d.halt(service, correlation)
}

// realLocked returns the shared job k names when it is up, the job itself and
// not a claim; the lock is held.
func (d *Daemon) realLocked(k key) *job {
	j := d.jobs[k]
	if j == nil || j.record.Claim || !j.up() {
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
