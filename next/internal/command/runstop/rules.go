package runstop

import (
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what run stop decides from, as Observe read them: what a stop decides from (runjobs.Running).
type Facts struct {
	runjobs.Running
}

// A Target is a worktree whose job stops, with what the stop does to it: none when the job is not up there.
type Target = runjobs.StopTarget

// ----------------------------------------------------------------------------
// Decide

// The codes of what a plan cannot see to: a daemon of another build answered, whose listing may not say whether the main checkout holds a shared service (its fate is then approximate); run.toml does not load (the job is taken by its name).
const (
	CodeDaemonSkew       = Name + domain.DaemonSkew
	CodeFatesApproximate = Name + runjobs.FatesApproximate
	CodeRunInvalid       = Name + ".run_invalid"
)

// DecideParams are what Decide decides from besides the facts and the request: the follow-up that restarts a daemon of another build.
type DecideParams struct {
	Restart *domain.FollowUp
}

// Decide turns the facts and the resolved request into the plan and its warnings: in each worktree, what the stop does to the job.
func Decide(f Facts, req Request, params DecideParams) (Plan, []domain.Warning) {
	targets, _ := f.Entries(req.Worktrees)
	planned := Plan{Job: req.Job, Worktrees: f.Stopping(targets, []string{req.Job})}

	warnings := f.StopWarnings(runjobs.StopWarningsParams{Command: Name, Targets: planned.Worktrees, Restart: params.Restart})
	if f.RunErr != nil {
		warnings = append(warnings, domain.Warning{Code: CodeRunInvalid, Message: "invalid run.toml: " + f.RunErr.Error() + " — the job is taken by its name"})
	}
	return planned, warnings
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: the job, and in each worktree what the stop does to it; then the recap, which the fingerprint leaves out.
type Plan struct {
	Job       string   `json:"job"`
	Worktrees []Target `json:"worktrees"`
	domain.Recapped
	correlationID string
}

// Locks are the locks the plan needs: the repository shared and each worktree it stops the job in exclusive, so a stop and a start of one worktree take turns.
func (p Plan) Locks() []domain.Lock {
	names := make([]string, 0, len(p.Worktrees))
	for _, t := range p.Worktrees {
		names = append(names, t.Worktree)
	}
	return domain.WorktreeLocks(names)
}

// Locks are the locks taken before planning: the repository shared and each worktree asked by its branch exclusive; one asked by a path is locked once the plan resolves it.
func (r Request) Locks() []domain.Lock {
	return domain.WorktreeLocks(slices.DeleteFunc(slices.Clone(r.Worktrees), func(name string) bool { return worktree.BranchProblem(name) != nil }))
}

// Fingerprinted is what the plan's fingerprint covers: the plan, its recap left out.
func (p Plan) Fingerprinted() any {
	p.Recapped = domain.Recapped{}
	return p
}

// unchanged reports whether the plan stops nothing: the job is up in none of the worktrees.
func (p Plan) unchanged() bool {
	return !slices.ContainsFunc(p.Worktrees, func(t Target) bool { return len(t.Jobs) > 0 })
}

// ----------------------------------------------------------------------------
// The recap

// recap is the plan row by row: the worktrees, the job, then what the stop does to it in each worktree.
func (p Plan) recap() []domain.RecapRow {
	names := make([]string, 0, len(p.Worktrees))
	for _, t := range p.Worktrees {
		names = append(names, t.Worktree)
	}
	rows := []domain.RecapRow{
		{Key: "worktrees", Label: "Worktrees", Path: domain.PathWorktree, Values: domain.RecapValues(names)},
		{Key: "job", Label: "Job", Path: PathJob, Values: domain.RecapValues([]string{p.Job})},
	}
	for _, t := range p.Worktrees {
		words := p.Job + " (not running)"
		if len(t.Jobs) > 0 {
			words = runjobs.StopWords(t.Jobs[0])
		}
		row := domain.RecapRow{Key: "stop", Label: "Stops", Values: domain.RecapValues([]string{words})}
		if len(p.Worktrees) > 1 {
			row.Key, row.Label = "stop:"+t.Worktree, "Stops in "+t.Worktree
		}
		rows = append(rows, row)
	}
	return rows
}

// ----------------------------------------------------------------------------
// The outcome

// notRunning is item, a worktree the job is not up in, saying so.
func (p Plan) notRunning(item Item) Item {
	if len(item.Detail.Jobs) == 0 {
		item.Detail.Jobs = append(item.Detail.Jobs, runjobs.StopResult{Name: p.Job, Status: runjobs.StopNotRunning})
	}
	return item
}

// CodeStart is the code of the follow-up that starts the job again where the stop stopped or released it, the web's undo.
const CodeStart = Name + ".start"

// pathRunStartJob is the path of run start's job, which a follow-up of run start answers.
const pathRunStartJob = "job"

// next are the follow-ups of a stop: run start of the job again in each worktree still there that stopped or released it, run start taking one worktree.
func (p Plan) next(items []Item) []domain.FollowUp {
	var followUps []domain.FollowUp
	for i, t := range p.Worktrees {
		if t.Removed || !runjobs.StoppedOrReleased(items[i]) {
			continue
		}
		followUps = append(followUps, domain.FollowUp{
			Command: domain.CommandRunStart,
			Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{t.Worktree}}, {Path: pathRunStartJob, Values: []string{p.Job}}},
			Code:    CodeStart,
			Message: "start it again",
		})
	}
	return followUps
}
