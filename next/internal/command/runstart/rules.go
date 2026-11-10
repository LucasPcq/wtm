package runstart

import (
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what run start decides from, as Observe read them: what a start decides from (runjobs.Facts).
type Facts struct {
	runjobs.Facts
}

// ----------------------------------------------------------------------------
// Decide

// The codes of the refusals of a request: a job that changes foreign data (CodeUnsafe, followed by CodeForce, the request forced), a port another worktree holds (CodePortClash, followed by CodeExclusive, the request stopping the others first), a daemon of another build holding jobs (CodeDaemonSkew, followed by the restart).
const (
	CodeUnsafe      = Name + runjobs.Unsafe
	CodeForeignData = Name + runjobs.ForeignData
	CodeForce       = Name + runjobs.Force
	CodePortClash   = Name + runjobs.PortClash
	CodeExclusive   = Name + runjobs.ExclusiveStart
	CodeDaemonSkew  = Name + domain.DaemonSkew
)

// CodeJobsUnknown warns of a plan wtm's daemon could not tell the jobs up of.
const CodeJobsUnknown = Name + domain.JobsUnknown

// DecideParams are what Decide decides from besides the facts and the request: the follow-up that restarts a daemon of another build.
type DecideParams struct {
	Restart *domain.FollowUp
}

// Decide turns the facts and the resolved request into the plan and its warnings, or the refusal that stops it (runjobs.Facts.Refusal): a daemon of another build holding jobs, a port another worktree holds unless its jobs are stopped first, a job that changes data the worktree does not own unless forced. The job's action is run up's (start, up, join), restart for one up there that --restart starts again.
func Decide(f Facts, req Request, params DecideParams) (Plan, []domain.Warning, error) {
	targets, _ := f.Entries(Name, []string{req.Worktree})
	job, _ := f.Declared(req.Job)
	jobs := []project.Job{job}
	exclusive := f.exclusive(req, targets, job).Value
	planned := Plan{Job: req.Job, Restart: req.Restart, Exclusive: exclusive, Worktrees: []runjobs.Target{}, Others: []runjobs.Other{}, NoProbe: req.NoProbe, facts: f, asked: req}
	for _, t := range targets {
		planned.Worktrees = append(planned.Worktrees, f.restarting(f.Planned(t, jobs, f.OrdinalOf(t)), req))
	}
	if exclusive {
		planned.Others = append(planned.Others, f.Others(targets)...)
	}
	planned.HeldBy = f.heldBy(planned)

	refusal := runjobs.RefusalParams{Command: Name, Restart: params.Restart, Targets: planned.Worktrees, Jobs: jobs, Exclusive: exclusive, Force: req.Force, JobsPath: PathJob, Stopping: stopping(req), Forced: forced(req)}
	if err := f.Refusal(refusal); err != nil {
		return Plan{}, nil, err
	}

	planned.Owed = f.debts(planned)
	var warnings []domain.Warning
	if f.JobsErr != nil {
		warnings = append(warnings, domain.Warning{Code: CodeJobsUnknown, Message: run.UnknownMessage(f.JobsErr) + " — the start leaves the other worktrees' jobs alone"})
	}
	return planned, warnings, nil
}

// holds reports whether a worktree of targets holds the shared service name: a claim on it, or the main checkout's own hold.
func (f Facts) holds(targets []runjobs.Target, name string) bool {
	return slices.ContainsFunc(targets, func(t runjobs.Target) bool { return slices.Contains(f.holders(name, t.Path), t.Path) })
}

// upAnywhere reports whether the job name is up in some worktree: a shared service up nowhere is started, restart or not.
func (f Facts) upAnywhere(name string) bool {
	// the daemon's listing is the machine's: another repository's service of that name is not this one's
	return slices.ContainsFunc(f.Jobs, func(job run.Job) bool { return job.Name == name && job.Up && job.SharedDir == f.MainPath })
}

// holders are the worktrees, by path, holding the shared service name that the worktree at dir runs or claims: none when it has none of it up. A daemon of another build that cannot tell whether the main checkout holds it is taken as holding it, as run stop takes it.
func (f Facts) holders(name, dir string) []string {
	at := slices.IndexFunc(f.Jobs, func(job run.Job) bool { return job.Name == name && job.WorkDir == dir && job.Up })
	if at < 0 {
		return nil
	}
	held := run.Holders(f.Jobs, f.Jobs[at])
	shared := f.Jobs[at].SharedDir
	unknown := slices.ContainsFunc(f.Jobs, func(job run.Job) bool {
		return job.Name == name && job.WorkDir == shared && job.SharedDir == shared && job.Up && job.MainHoldsUnknown
	})
	if unknown && !slices.Contains(held, shared) {
		held = append(held, shared)
		slices.Sort(held)
	}
	return held
}

// heldBy are the worktrees, by name, a restart of the shared service the plan starts again stops for a moment: every one holding it; none for any other start.
func (f Facts) heldBy(p Plan) []string {
	if !p.Restarts() || !f.Shared(p.Job) {
		return nil
	}
	var names []string
	for _, t := range p.Worktrees {
		for _, path := range f.holders(p.Job, t.Path) {
			names = append(names, f.nameAt(path))
		}
	}
	return names
}

// nameAt is the name of the worktree at path, its path when git lists none there.
func (f Facts) nameAt(path string) string {
	if at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Path == path }); at >= 0 {
		return f.Worktrees[at].Name()
	}
	return path
}

// restarting is t with its job restarted when req asks it and the job is up there; a shared service only where the worktree holds it (Resolve refuses it elsewhere), the daemon restarting it in place for every worktree holding it.
func (f Facts) restarting(t runjobs.Target, req Request) runjobs.Target {
	if !req.Restart {
		return t
	}
	for i, job := range t.Jobs {
		own := !f.Shared(job.Name) && slices.Contains(run.UpIn(f.Jobs, t.Path), job.Name)
		if own || f.Shared(job.Name) && slices.Contains(f.holders(job.Name, t.Path), t.Path) {
			t.Jobs[i].Action = runjobs.ActionRestart
		}
	}
	return t
}

// stopping are the answers of req with the other worktrees' jobs stopped first.
func stopping(req Request) []domain.Answered {
	yes := true
	req.Exclusive, req.Parallel, req.Caller = &yes, false, domain.Caller{}
	return domain.AnswersOf(Questions{}.Form(), req)
}

// forced are the answers of req forced.
func forced(req Request) []domain.Answered {
	req.Force, req.Caller = true, domain.Caller{}
	return domain.AnswersOf(Questions{}.Form(), req)
}

// debts are the drops removed worktrees owe that the start pays once its job is up: each whose shared service it starts or joins.
func (f Facts) debts(p Plan) []teardown.Debt {
	if !f.Shared(p.Job) {
		return nil
	}
	return f.teardown().Payable([]string{p.Job})
}

// teardown are the facts as teardown pays the drops owed from them.
func (f Facts) teardown() teardown.Facts {
	return teardown.Facts{Repository: f.Repository, Records: f.Records, Jobs: f.Jobs, Owed: f.Owed}
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: the job started, in its worktree, with what the start does to it; whether --restart asked it started again; whether the other worktrees' jobs are stopped first, and those worktrees; whether its ports go unchecked; the drops owed it pays; then the recap, which the fingerprint leaves out.
type Plan struct {
	Job       string           `json:"job"`
	Restart   bool             `json:"restart"`
	HeldBy    []string         `json:"held_by,omitempty"`
	Worktrees []runjobs.Target `json:"worktrees"`
	Exclusive bool             `json:"exclusive"`
	Others    []runjobs.Other  `json:"others"`
	NoProbe   bool             `json:"no_probe"`
	Owed      []teardown.Debt  `json:"owed,omitempty"`
	domain.Recapped
	correlationID string
	facts         Facts
	asked         Request
}

// Locks are the locks the plan needs: the repository shared and the worktree it starts in exclusive, so two starts of one worktree take turns and nothing removes it under a start.
func (p Plan) Locks() []domain.Lock {
	names := make([]string, 0, len(p.Worktrees))
	for _, t := range p.Worktrees {
		names = append(names, t.Worktree)
	}
	return domain.WorktreeLocks(names)
}

// Locks are the locks taken before planning: the repository shared and the worktree asked by its branch exclusive; one asked by its path is locked once the plan resolves it.
func (r Request) Locks() []domain.Lock {
	var names []string
	if worktree.BranchProblem(r.Worktree) == nil {
		names = append(names, r.Worktree)
	}
	return domain.WorktreeLocks(names)
}

// Fingerprinted is what the plan's fingerprint covers: the plan, the job of run.toml as declared, and the ordinal and isolation the worktree's environment reads; its recap left out.
func (p Plan) Fingerprinted() any {
	p.Recapped = domain.Recapped{}
	type runEnv struct {
		Ordinal   int               `json:"ordinal"`
		Isolation project.Isolation `json:"isolation"`
	}
	envs := map[string]runEnv{}
	for _, t := range p.Worktrees {
		record := p.facts.Records[t.Branch]
		envs[t.Worktree] = runEnv{Ordinal: record.Ordinal, Isolation: record.Isolation.Effective()}
	}
	declared, _ := p.facts.Declared(p.Job)
	return struct {
		Plan
		Declared project.Job       `json:"declared"`
		RunEnvs  map[string]runEnv `json:"run_envs"`
	}{p, declared, envs}
}

// Restarts reports whether the plan starts a job up there again: --restart asked, and the job is up.
func (p Plan) Restarts() bool {
	return slices.ContainsFunc(p.Worktrees, func(t runjobs.Target) bool {
		return slices.ContainsFunc(t.Jobs, func(job runjobs.JobPlan) bool { return job.Action == runjobs.ActionRestart })
	})
}

// unchanged reports whether the plan starts nothing and stops nothing: the job is up already, and --restart did not ask it again.
func (p Plan) unchanged() bool {
	for _, t := range p.Worktrees {
		if slices.ContainsFunc(t.Jobs, func(job runjobs.JobPlan) bool { return job.Action != runjobs.ActionUp }) {
			return false
		}
	}
	return len(p.Others) == 0 && len(p.Owed) == 0
}

// ----------------------------------------------------------------------------
// The recap

// RecapOwed keys the recap's row of the drops owed the plan pays; RecapHeldBy the worktrees a shared service's restart stops for a moment.
const (
	RecapOwed   = teardown.RecapOwed
	RecapHeldBy = "held_by"
)

// recap is the plan row by row: the worktree, the job with what the start does to it, the worktrees whose jobs stop first, and the drops owed it pays.
func (p Plan) recap() []domain.RecapRow {
	var names, jobs []string
	for _, t := range p.Worktrees {
		names = append(names, t.Worktree)
		for _, job := range t.Jobs {
			jobs = append(jobs, jobWords(job))
		}
	}
	rows := []domain.RecapRow{
		{Key: "worktree", Label: "Worktree", Path: domain.PathWorktree, Values: domain.RecapValues(names)},
		{Key: "job", Label: "Job", Path: PathJob, Values: domain.RecapValues(jobs)},
	}
	if len(p.Others) > 0 {
		var stopped []string
		for _, other := range p.Others {
			stopped = append(stopped, other.Worktree)
		}
		rows = append(rows, domain.RecapRow{Key: "stops", Label: "Stops first", Path: PathExclusive, Values: domain.RecapValues(stopped)})
	}
	if len(p.HeldBy) > 0 {
		rows = append(rows, domain.RecapRow{Key: RecapHeldBy, Label: "Stops for a moment in", Values: domain.RecapValues(p.HeldBy)})
	}
	if len(p.Owed) > 0 {
		rows = append(rows, domain.RecapRow{Key: RecapOwed, Label: "Owed", Values: teardown.RecappedDebts(p.Owed)})
	}
	return rows
}

// jobWords is how the recap names the job: its name, and what the start does to it unless it starts.
func jobWords(job runjobs.JobPlan) string {
	switch job.Action {
	case runjobs.ActionUp:
		return job.Name + " (up, left as it is)"
	case runjobs.ActionJoin:
		return job.Name + " (joined, in " + job.SharedIn + ")"
	case runjobs.ActionRestart:
		return job.Name + " (stopped, then started again)"
	}
	return job.Name
}

// notStarted is the detail of t before its start: its job, not started.
func notStarted(t runjobs.Target) *Detail {
	return &Detail{Path: t.Path, Jobs: t.NotStarted()}
}

// ----------------------------------------------------------------------------
// The outcome

// The codes of the follow-ups of a start: the job's output, the stop of the worktree's jobs it left up, and the start again of a job a restart cut short left stopped.
const (
	CodeLogs  = Name + ".logs"
	CodeDown  = Name + ".down"
	CodeStart = Name + ".start"
)

// next are the follow-ups of a start that left its job up: the worktree's output, and the stop of its jobs; none when nothing is left up (B5); the start again of a job a restart cut short left stopped.
func (p Plan) next(items []Item) []domain.FollowUp {
	var followUps []domain.FollowUp
	for _, item := range items {
		if item.Detail != nil && slices.ContainsFunc(item.Detail.Jobs, func(job runjobs.JobResult) bool { return job.Status == runjobs.JobStopped }) {
			again := domain.AnswersOf(Questions{}.Form(), Request{Worktree: item.Subject, Job: p.Job})
			followUps = append(followUps, domain.FollowUp{Command: Name, Answers: again, Code: CodeStart, Message: "start " + p.Job + " again: the restart was cut short once it stopped"})
		}
		if item.Detail == nil || !item.Detail.LeftUp() {
			continue
		}
		on := []domain.Answered{{Path: domain.PathWorktree, Values: []string{item.Subject}}}
		followUps = append(followUps,
			domain.FollowUp{Command: domain.CommandRunLogs, Answers: on, Code: CodeLogs, Message: "see " + item.Subject + "'s output"},
			domain.FollowUp{Command: domain.CommandRunDown, Answers: on, Code: CodeDown, Message: "stop " + item.Subject + "'s jobs"},
		)
	}
	return followUps
}
