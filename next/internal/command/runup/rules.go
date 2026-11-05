package runup

import (
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what run up decides from, as Observe read them: what a start decides from (runjobs.Facts), the profiles of run.toml among them.
type Facts struct {
	runjobs.Facts
}

// defaultProfile is the profile run.toml marks default, else its only one.
func (f Facts) defaultProfile() (project.Profile, bool) {
	if at := slices.IndexFunc(f.Run.Profiles, func(p project.Profile) bool { return p.Default }); at >= 0 {
		return f.Run.Profiles[at], true
	}
	if len(f.Run.Profiles) == 1 {
		return f.Run.Profiles[0], true
	}
	return project.Profile{}, false
}

// jobsOf are the jobs the profile of name starts, in its order; every job of run.toml when it declares no profile.
func (f Facts) jobsOf(name string) []project.Job {
	if len(f.Run.Profiles) == 0 {
		return f.Run.Jobs
	}
	at := slices.IndexFunc(f.Run.Profiles, func(p project.Profile) bool { return p.Name == name })
	if at < 0 {
		return nil
	}
	var jobs []project.Job
	for _, job := range f.Run.Profiles[at].Jobs {
		if declared, ok := f.Declared(job); ok {
			jobs = append(jobs, declared)
		}
	}
	return jobs
}

// ----------------------------------------------------------------------------
// Decide

// The codes of the refusals of a request: a job that changes foreign data (CodeUnsafe, followed by CodeForce, the request forced), a port another worktree holds (CodePortClash, followed by CodeExclusive, the request stopping the others first), a job a runner of the same start starts too (CodeRunnerConflict), a run daemon of another build holding jobs (CodeDaemonSkew, followed by the restart), and every job of the request already up as asked is no refusal.
const (
	CodeUnsafe         = Name + runjobs.Unsafe
	CodeForeignData    = Name + runjobs.ForeignData
	CodeForce          = Name + runjobs.Force
	CodePortClash      = Name + runjobs.PortClash
	CodeExclusive      = Name + runjobs.ExclusiveStart
	CodeRunnerConflict = Name + runjobs.RunnerConflict
	CodeDaemonSkew     = Name + domain.DaemonSkew
)

// The codes of the warnings of a plan: the jobs up unknown, the run.toml's exclusive set aside for a start of several worktrees.
const (
	CodeJobsUnknown      = Name + domain.JobsUnknown
	CodeExclusiveIgnored = Name + ".exclusive_ignored"
)

// DecideParams are what Decide decides from besides the facts and the request: the follow-up that restarts a daemon of another build.
type DecideParams struct {
	Restart *domain.FollowUp
}

// Decide turns the facts and the resolved request into the plan and its warnings, or the refusal that stops it (runjobs.Facts.Refusal): a run daemon of another build holding jobs, a job a runner of the start starts too, a port another worktree holds unless its jobs are stopped first, a job that changes data a worktree does not own unless forced.
func Decide(f Facts, req Request, params DecideParams) (Plan, []domain.Warning, error) {
	targets, _ := f.Entries(Name, req.Worktrees)
	jobs := f.jobsOf(req.Profile)
	exclusive := f.exclusive(req, targets).Value
	planned := Plan{Profile: req.Profile, Exclusive: exclusive, Worktrees: []runjobs.Target{}, Others: []runjobs.Other{}, NoProbe: req.NoProbe, facts: f, asked: req}
	for _, t := range targets {
		planned.Worktrees = append(planned.Worktrees, f.Planned(t, jobs, f.OrdinalOf(t)))
	}
	if exclusive {
		planned.Others = append(planned.Others, f.Others(targets)...)
	}

	refusal := runjobs.RefusalParams{Command: Name, Restart: params.Restart, Targets: planned.Worktrees, Jobs: jobs, Exclusive: exclusive, Force: req.Force, JobsPath: PathProfile, Forced: forced(req)}
	if len(targets) == 1 {
		// --exclusive starts one worktree only: offered beside several, it would be refused in turn.
		refusal.Stopping = stopping(req)
	}
	if err := f.Refusal(refusal); err != nil {
		return Plan{}, nil, err
	}

	planned.Owed = f.debts(planned)
	return planned, f.warnings(planned, req), nil
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

// debts are the drops removed worktrees owe that the start pays once its jobs are up: each whose shared service is up already, or which it starts or joins.
func (f Facts) debts(p Plan) []teardown.Debt {
	var starting []string
	for _, t := range p.Worktrees {
		for _, job := range t.Jobs {
			if f.Shared(job.Name) {
				starting = append(starting, job.Name)
			}
		}
	}
	return f.teardown().Payable(starting)
}

// teardown are the facts as teardown pays the drops owed from them.
func (f Facts) teardown() teardown.Facts {
	return teardown.Facts{Repository: f.Repository, Records: f.Records, Jobs: f.Jobs, Owed: f.Owed}
}

func (f Facts) warnings(p Plan, req Request) []domain.Warning {
	var warnings []domain.Warning
	if f.JobsErr != nil {
		warnings = append(warnings, domain.Warning{Code: CodeJobsUnknown, Message: run.UnknownMessage(f.JobsErr) + " — the start leaves the other worktrees' jobs alone"})
	}
	if req.Exclusive == nil && f.Run.Concurrency == project.Exclusive && len(p.Worktrees) > 1 && len(f.Others(p.Worktrees)) > 0 {
		warnings = append(warnings, domain.Warning{Code: CodeExclusiveIgnored, Message: "run.toml stops the other worktrees' jobs first, which cannot start several: they keep running"})
	}
	return warnings
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: the profile started (none when run.toml declares none: every job), in each worktree, in order; whether the other worktrees' jobs are stopped first, and those worktrees; whether the ports go unchecked; then the recap, which the fingerprint leaves out.
type Plan struct {
	Profile   string           `json:"profile"`
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

// Locks are the locks the plan needs: the repository shared and each worktree it starts in exclusive, so two starts of one worktree take turns and nothing removes one under a start.
func (p Plan) Locks() []domain.Lock {
	names := make([]string, 0, len(p.Worktrees))
	for _, t := range p.Worktrees {
		names = append(names, t.Worktree)
	}
	return domain.WorktreeLocks(names)
}

// Locks are the locks taken before planning: the repository shared and each worktree asked exclusive.
func (r Request) Locks() []domain.Lock {
	return domain.WorktreeLocks(slices.DeleteFunc(slices.Clone(r.Worktrees), func(name string) bool { return strings.TrimSpace(name) == "" }))
}

// Fingerprinted is what the plan's fingerprint covers: the plan, the jobs of run.toml as declared, and the ordinal and isolation each worktree's environment reads; its recap left out.
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
	return struct {
		Plan
		Jobs    []project.Job     `json:"jobs"`
		RunEnvs map[string]runEnv `json:"run_envs"`
	}{p, p.facts.Run.Jobs, envs}
}

// ----------------------------------------------------------------------------
// The recap

// RecapOwed keys the recap's row of the drops owed the plan pays.
const RecapOwed = teardown.RecapOwed

// recap is the plan row by row: the worktrees, the profile, the jobs each worktree starts, in order, the worktrees whose jobs stop first, and the drops owed it pays.
func (p Plan) recap() []domain.RecapRow {
	names := make([]string, 0, len(p.Worktrees))
	for _, t := range p.Worktrees {
		names = append(names, t.Worktree)
	}
	rows := []domain.RecapRow{{Key: "worktrees", Label: "Worktrees", Path: domain.PathWorktree, Values: domain.RecapValues(names)}}
	if p.Profile != "" {
		rows = append(rows, domain.RecapRow{Key: "profile", Label: "Profile", Path: PathProfile, Values: domain.RecapValues([]string{p.Profile})})
	}
	for _, t := range p.Worktrees {
		var jobs []string
		for _, job := range t.Jobs {
			jobs = append(jobs, jobWords(job))
		}
		row := domain.RecapRow{Key: "jobs", Label: "Jobs", Values: domain.RecapValues(jobs)}
		if len(p.Worktrees) > 1 {
			row.Key, row.Label = "jobs:"+t.Worktree, "Jobs in "+t.Worktree
		}
		rows = append(rows, row)
	}
	if len(p.Others) > 0 {
		var stopped []string
		for _, other := range p.Others {
			stopped = append(stopped, other.Worktree)
		}
		rows = append(rows, domain.RecapRow{Key: "stops", Label: "Stops first", Path: PathExclusive, Values: domain.RecapValues(stopped)})
	}
	if len(p.Owed) > 0 {
		rows = append(rows, domain.RecapRow{Key: RecapOwed, Label: "Owed", Values: teardown.RecappedDebts(p.Owed)})
	}
	return rows
}

// notStarted is the detail of t before its start: each job of its plan, none started.
func (p Plan) notStarted(t runjobs.Target) *Detail {
	return &Detail{Path: t.Path, Profile: p.Profile, Jobs: t.NotStarted()}
}

// unchanged reports whether the plan starts nothing and stops nothing: every job is up already.
func (p Plan) unchanged() bool {
	for _, t := range p.Worktrees {
		if slices.ContainsFunc(t.Jobs, func(job runjobs.JobPlan) bool { return job.Action != runjobs.ActionUp }) {
			return false
		}
	}
	return len(p.Others) == 0 && len(p.Owed) == 0
}

// jobWords is how the recap names one job: its name, and what it does unless it starts.
func jobWords(job runjobs.JobPlan) string {
	switch job.Action {
	case runjobs.ActionUp:
		return job.Name + " (up)"
	case runjobs.ActionJoin:
		return job.Name + " (joined, in " + job.SharedIn + ")"
	}
	return job.Name
}

// ----------------------------------------------------------------------------
// The outcome

// The codes of the follow-ups of a start: its jobs' output, the stop of what it left up, and the start again where it did not pass.
const (
	CodeLogs  = Name + ".logs"
	CodeDown  = Name + ".down"
	CodeRetry = Name + ".retry"
)

// next are the follow-ups of a start: the start again where it did not pass, then each worktree's output and the stop of what it left up.
func (p Plan) next(items []Item) []domain.FollowUp {
	var followUps []domain.FollowUp
	if retry := p.retry(items); retry != nil {
		followUps = append(followUps, *retry)
	}
	for _, item := range items {
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

// retry starts the profile again in each worktree of items that did not pass; none when every one passed.
func (p Plan) retry(items []Item) *domain.FollowUp {
	var again []string
	for _, item := range items {
		if item.Status != domain.StatusDone {
			again = append(again, item.Subject)
		}
	}
	if len(again) == 0 {
		return nil
	}

	req := Request{Worktrees: again, Profile: p.asked.Profile, Force: p.asked.Force, NoProbe: p.asked.NoProbe}
	return &domain.FollowUp{Command: Name, Answers: domain.AnswersOf(Questions{}.Form(), req), Code: CodeRetry, Message: "start it again where it did not pass"}
}
