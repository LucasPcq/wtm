package runjobs

import (
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// A Target is a worktree a start runs jobs in: its name, its branch (none on no branch) and its root as git spells it; whether it is the main one; whether its ports are known (Numbered: one whose ports derive from an ordinal it lacks is numbered first); and its jobs, in the order they start.
type Target struct {
	Worktree string    `json:"worktree"`
	Branch   string    `json:"branch,omitempty"`
	Path     string    `json:"path"`
	Main     bool      `json:"main,omitempty"`
	Numbered bool      `json:"numbered"`
	Jobs     []JobPlan `json:"jobs"`
	entry    int
}

// A JobPlan is how one job starts in its worktree: what the start does (Action); its Step among the jobs the worktree starts, none for a shared job it joins; the tasks it waits for, which run before it; the main checkout's branch a shared job runs in; the ports it binds, offset for the worktree; and the namespace of its data a shared job makes for the worktree.
type JobPlan struct {
	Name      string          `json:"name"`
	Kind      project.JobKind `json:"kind"`
	Action    Action          `json:"action"`
	Step      int             `json:"step,omitempty"`
	WaitsFor  []string        `json:"waits_for,omitempty"`
	SharedIn  string          `json:"shared_in,omitempty"`
	Ports     map[string]int  `json:"ports,omitempty"`
	Namespace string          `json:"namespace,omitempty"`
}

// Action is what a start does to one job.
type Action string

// The actions of a start.
const (
	ActionStart   Action = "start"   // the job starts: a service not up, a task, which runs each time
	ActionUp      Action = "up"      // a service already up in the worktree, left as it is
	ActionJoin    Action = "join"    // a shared service the worktree holds, run once in the main checkout, started there when it is not up
	ActionRestart Action = "restart" // a job up in the worktree, stopped then started again, its log continued behind the run mark
)

// Actions returns every Action.
func Actions() []Action {
	return []Action{ActionStart, ActionUp, ActionJoin, ActionRestart}
}

// Planned is how t starts jobs: each in order, a shared one joined from a worktree other than the main one, a service up left as it is, the others numbered, each waiting for the tasks before it; the ports and namespaces from the environment ordinal gives, none while t is not numbered.
func (f Facts) Planned(t Target, jobs []project.Job, ordinal int) Target {
	params := f.jobParams(t, ordinal)
	jobEnv := env.HookEnv(params, nil)
	up := run.UpIn(f.Jobs, t.Path)
	main := f.mainTarget().Worktree
	step := 0
	var tasks []string
	t.Jobs = []JobPlan{}
	for _, job := range jobs {
		planned := JobPlan{Name: job.Name, Kind: job.Kind, Action: ActionStart, WaitsFor: slices.Clone(tasks)}
		if t.Numbered {
			planned.Ports = nonEmpty(params.JobPorts(f.effective(job)))
		}
		switch {
		// The main checkout joins a shared service it runs for the others alone, which takes its own hold on it.
		case job.Scope == project.Shared && (!t.Main || f.runsForOthers(t.Path, job.Name)):
			planned.Action, planned.SharedIn, planned.Ports = ActionJoin, main, nil
			if namespace, ok, _ := env.NamespaceFor(job, jobEnv); ok && t.Numbered {
				planned.Namespace = namespace.Name
			}
		case job.Kind != project.Task && slices.Contains(up, job.Name):
			planned.Action = ActionUp
		}
		if planned.Action != ActionJoin {
			step++
			planned.Step = step
		}
		if job.Kind == project.Task {
			tasks = append(tasks, job.Name)
		}
		t.Jobs = append(t.Jobs, planned)
	}
	return t
}

// runsForOthers reports whether the main checkout at dir runs the shared service name for the other worktrees alone, holding none of it.
func (f Facts) runsForOthers(dir, name string) bool {
	return slices.ContainsFunc(f.Jobs, func(job run.Job) bool {
		return job.Name == name && job.WorkDir == dir && job.Up && !job.MainHolds && !job.MainHoldsUnknown
	})
}

// NotStarted is each job of t's plan as its result reports it before its start: none started.
func (t Target) NotStarted() []JobResult {
	jobs := make([]JobResult, 0, len(t.Jobs))
	for _, job := range t.Jobs {
		jobs = append(jobs, JobResult{Name: job.Name, Kind: job.Kind, Status: JobNotStarted, Namespace: job.Namespace})
	}
	return jobs
}

// nonEmpty is ports, nil when it holds none.
func nonEmpty(ports map[string]int) map[string]int {
	if len(ports) == 0 {
		return nil
	}
	return ports
}
