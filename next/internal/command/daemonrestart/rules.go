package daemonrestart

import (
	"cmp"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// subject is the one item of a restart: the daemon.
const subject = "daemon"

// ----------------------------------------------------------------------------
// Facts

// Facts are what a restart decides from: the daemon as it stands, the records of its index, and this wtm's build.
type Facts struct {
	Daemon  run.Daemon
	Records []run.Record
	Version string
}

// recordOf is the record of job, how it was started; nil when the index has none.
func (f Facts) recordOf(job run.Job) *run.Record {
	at := slices.IndexFunc(f.Records, func(r run.Record) bool { return r.Spec.WorkDir == job.WorkDir && r.Spec.Job.Name == job.Name })
	if at < 0 {
		return nil
	}
	return &f.Records[at]
}

// startOf is how the job of record starts again: its spec, a shared job's naming where it runs as the record of its own run in the main checkout says, the index keeping no more of it than that checkout; none without a record, or for a shared job whose own run has none.
func (f Facts) startOf(record *run.Record) (run.Spec, bool) {
	if record == nil {
		return run.Spec{}, false
	}
	spec := record.Spec
	if spec.Origin == nil {
		// The legacy wtm recorded a claim with no origin: the worktree's other jobs name it.
		if at := slices.IndexFunc(f.Records, func(r run.Record) bool { return r.Spec.Origin != nil && r.Spec.WorkDir == spec.WorkDir }); at >= 0 {
			spec.Origin = f.Records[at].Spec.Origin
		}
	}
	if spec.Job.Scope != project.Shared || spec.Shared != nil {
		return spec, true
	}
	at := slices.IndexFunc(f.Records, func(r run.Record) bool {
		return !r.Claim && r.Spec.WorkDir == record.SharedDir && r.Spec.Job.Name == spec.Job.Name
	})
	if at < 0 {
		return run.Spec{}, false
	}
	runs := f.Records[at].Spec
	spec.Shared = &run.Shared{WorkDir: runs.WorkDir, Env: runs.Env, LogDir: runs.LogDir}
	return spec, true
}

// jobs are the jobs the daemon holds; for one whose listing cannot be read (another protocol), those its index records, every one up.
func (f Facts) jobs() []run.Job {
	if !f.Daemon.Answering || f.Daemon.Skew != run.SkewProtocol {
		return f.Daemon.Jobs
	}
	jobs := make([]run.Job, 0, len(f.Records))
	for _, r := range f.Records {
		job := run.Job{Name: r.Spec.Job.Name, WorkDir: r.Spec.WorkDir, Up: true, Kind: r.Spec.Job.Kind, Hold: r.Hold(), SharedDir: r.SharedDir, MainHolds: r.MainHolds}
		if r.Spec.Origin != nil {
			job.Repo, job.Branch = r.Spec.Origin.Root, r.Spec.Origin.Branch
		}
		jobs = append(jobs, job)
	}
	return jobs
}

// ----------------------------------------------------------------------------
// Decide

// CodeIndexFrozen warns of an index a newer wtm wrote: the daemon recorded nothing of what it started, so nothing of it comes back.
const CodeIndexFrozen = Name + domain.IndexFrozen

// Decide turns the facts into the plan and its warnings: of the jobs the shutdown stops, those that come back (a service its index records, a shared one through the worktrees that hold it), the task not run again and the job with no record (a shared one whose own run has none, or that no worktree holding it brings back); the stacks taken over; the daemon's build and the new one's.
func Decide(f Facts) (Plan, []domain.Warning) {
	d := f.Daemon
	plan := Plan{Running: d.Answering, From: d.Stamp.Version, Legacy: d.Answering && d.Skew == run.SkewLegacy, To: f.Version}
	shutdown := run.ShutdownOf(f.jobs())

	var back, tasks, unrecorded, held []run.Job
	var own, holds []run.Spec
	for _, job := range shutdown.Cut {
		record := f.recordOf(job)
		spec, startable := f.startOf(record)
		switch {
		case job.Kind == project.Task:
			tasks = append(tasks, job)
		case !startable:
			unrecorded = append(unrecorded, job)
		case job.SharedDir == job.WorkDir && !job.MainHolds:
			// A shared job's own run its main checkout does not hold comes back with the worktrees that hold it.
			held = append(held, job)
		default:
			back = append(back, job)
			own = append(own, spec)
		}
	}
	served := map[string]bool{}
	for _, claim := range shutdown.Lost {
		spec, startable := f.startOf(f.recordOf(claim))
		if !startable {
			unrecorded = append(unrecorded, claim)
			continue
		}
		back = append(back, claim)
		holds = append(holds, spec)
		served[claim.SharedDir+"\x00"+claim.Name] = true
	}
	for _, job := range held {
		if served[job.WorkDir+"\x00"+job.Name] {
			back = append(back, job)
		} else {
			unrecorded = append(unrecorded, job)
		}
	}

	plan.Back, plan.TakenOver, plan.Tasks, plan.Unrecorded = run.Held(back), run.Held(shutdown.Kept), run.Held(tasks), run.Held(unrecorded)
	plan.starts = append(own, holds...)
	plan.Confirm = len(shutdown.Cut) > 0
	plan.Recap = plan.recap()

	var warnings []domain.Warning
	if d.IndexFrozen {
		warnings = append(warnings, domain.IndexFrozenWarning(Name, d.Index))
	}
	return plan, warnings
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: whether a daemon runs to restart (none: one starts), its build (From, the legacy wtm's when Legacy) and the one it restarts as (To); the jobs that come back on the new daemon (Back), the stacks it takes over (TakenOver), the tasks the shutdown cuts, not run again (Tasks), the jobs its index has no record of, which cannot come back (Unrecorded); and whether a surface asks to confirm it (Confirm: a job is cut, if only for a moment); then the recap, which the fingerprint leaves out.
type Plan struct {
	Running    bool           `json:"running"`
	From       string         `json:"from,omitempty"`
	Legacy     bool           `json:"legacy,omitempty"`
	To         string         `json:"to"`
	Back       []run.HeldRepo `json:"back"`
	TakenOver  []run.HeldRepo `json:"taken_over"`
	Tasks      []run.HeldRepo `json:"tasks"`
	Unrecorded []run.HeldRepo `json:"unrecorded"`
	Confirm    bool           `json:"confirm"`
	domain.Recapped
	starts        []run.Spec
	correlationID string
}

// Locks are none: the daemon is no repository's.
func (Plan) Locks() []domain.Lock {
	return nil
}

// Fingerprinted is what the plan's fingerprint covers: the plan, its recap left out.
func (p Plan) Fingerprinted() any {
	p.Recapped = domain.Recapped{}
	return p
}

// ----------------------------------------------------------------------------
// The recap

// recap is the plan row by row: the daemon, then what comes back, what is taken over, and what does not come back.
func (p Plan) recap() []domain.RecapRow {
	rows := []domain.RecapRow{{Key: "daemon", Label: "Daemon", Values: domain.RecapValues([]string{p.daemon()})}}
	add := func(key, label string, values []string) {
		if len(values) > 0 {
			rows = append(rows, domain.RecapRow{Key: key, Label: label, Values: domain.RecapValues(values)})
		}
	}
	add("back", "Come back", run.Names(p.Back))
	add("taken_over", "Taken over", run.Names(p.TakenOver))
	var notBack []string
	for _, name := range run.Names(p.Tasks) {
		notBack = append(notBack, name+", a task: not run again")
	}
	for _, name := range run.Names(p.Unrecorded) {
		notBack = append(notBack, name+", unrecorded")
	}
	add("not_back", "Not back", notBack)
	return rows
}

// daemon says what becomes of the daemon: none running starts, one running restarts as this build.
func (p Plan) daemon() string {
	switch {
	case !p.Running:
		return "none runs: one starts, as " + p.To
	case p.Legacy:
		return "the legacy wtm's " + cmp.Or(p.From, "(older)") + " → " + p.To
	}
	return cmp.Or(p.From, "(older)") + " → " + p.To
}

// ----------------------------------------------------------------------------
// The outcome

// A Detail is what came back: each job the restart started again, in its order.
type Detail struct {
	Back []Back `json:"back"`
}

// A Back is one job the restart started again: its name, its worktree's path, whether it is up again, and why not.
type Back struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Back    bool   `json:"back"`
	Message string `json:"message,omitempty"`
}

// notBack is item failed naming each job that did not come back, done when all did.
func notBack(item Item) Item {
	var failed []string
	for _, b := range item.Detail.Back {
		if !b.Back {
			failed = append(failed, b.Name+" ("+b.Path+"): "+b.Message)
		}
	}
	if len(failed) > 0 {
		item.Status, item.Err = domain.StatusFailed, &domain.Error{Kind: domain.KindInternal, Code: CodeNotBack, Message: "didn't come back: " + strings.Join(failed, "; ")}
	}
	return item
}
