package rundown

import (
	"cmp"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/runjobs"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what run down decides from, as Observe read them: what a stop decides from (runjobs.Running).
type Facts struct {
	runjobs.Running
}

// profileJobs are the jobs of the profile of name, nil for none.
func (f Facts) profileJobs(name string) []string {
	if at := slices.IndexFunc(f.Run.Profiles, func(p project.Profile) bool { return p.Name == name }); at >= 0 {
		return append([]string{}, f.Run.Profiles[at].Jobs...)
	}
	return nil
}

// ----------------------------------------------------------------------------
// The decision of each worktree

// A Target is a worktree whose jobs stop (runjobs.StopTarget), with the profile its jobs were started under and, with a profile, the profile's jobs not running there.
type Target struct {
	runjobs.StopTarget
	Profile    string   `json:"profile,omitempty"`
	NotRunning []string `json:"not_running,omitempty"`
}

// plan is t as the stop of profile reaches it: the profile its jobs ran, and the jobs of profile it does not reach.
func (f Facts) plan(t runjobs.StopTarget, profile string) Target {
	planned := Target{StopTarget: t, Profile: f.ProfileOf(t)}
	up := run.UpIn(f.Jobs, t.Path)
	for _, name := range f.profileJobs(profile) {
		if !slices.Contains(up, name) && !slices.ContainsFunc(t.Jobs, func(job runjobs.StopJob) bool { return job.Name == name }) {
			planned.NotRunning = append(planned.NotRunning, name)
		}
	}
	return planned
}

// ----------------------------------------------------------------------------
// Decide

// The codes of what a plan cannot see to: a run daemon of another build answered, whose listing may not say whether the main checkout holds a shared service (the fates of the shared services are then approximate); run.toml does not load.
const (
	CodeDaemonSkew       = Name + domain.DaemonSkew
	CodeFatesApproximate = Name + runjobs.FatesApproximate
	CodeRunInvalid       = Name + ".run_invalid"
)

// DecideParams are what Decide decides from besides the facts and the request: the follow-up that restarts a daemon of another build.
type DecideParams struct {
	Restart *domain.FollowUp
}

// Decide turns the facts and the resolved request into the plan and its warnings: each worktree's jobs, with what the stop does to each.
func Decide(f Facts, req Request, params DecideParams) (Plan, []domain.Warning) {
	targets, _ := f.Entries(req.Worktrees)
	if req.All {
		targets = f.Every()
	}
	targets = f.Stopping(targets, f.profileJobs(req.Profile))
	planned := Plan{Profile: req.Profile, Worktrees: make([]Target, 0, len(targets))}
	for _, t := range targets {
		planned.Worktrees = append(planned.Worktrees, f.plan(t, req.Profile))
	}

	warnings := f.StopWarnings(runjobs.StopWarningsParams{Command: Name, Targets: targets, Restart: params.Restart})
	if f.RunErr != nil {
		warnings = append(warnings, domain.Warning{Code: CodeRunInvalid, Message: "invalid run.toml: " + f.RunErr.Error() + " — the jobs the run daemon runs stop all the same"})
	}
	return planned, warnings
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: the profile whose jobs alone stop (none: every job), and each worktree's jobs, with what the stop does to each; then the recap, which the fingerprint leaves out.
type Plan struct {
	Profile   string   `json:"profile,omitempty"`
	Worktrees []Target `json:"worktrees"`
	domain.Recapped
	correlationID string
}

// Locks are the locks the plan needs: the repository shared and each worktree it stops the jobs of exclusive, so a stop and a start of one worktree take turns.
func (p Plan) Locks() []domain.Lock {
	names := make([]string, 0, len(p.Worktrees))
	for _, t := range p.Worktrees {
		names = append(names, t.Worktree)
	}
	return domain.WorktreeLocks(names)
}

// Locks are the locks taken before planning: the repository shared and each worktree asked by its branch exclusive; one asked by a path (`.`, `../feat-a`) is locked once the plan resolves it.
func (r Request) Locks() []domain.Lock {
	return domain.WorktreeLocks(slices.DeleteFunc(slices.Clone(r.Worktrees), func(name string) bool { return worktree.BranchProblem(name) != nil }))
}

// Fingerprinted is what the plan's fingerprint covers: the plan, its recap left out.
func (p Plan) Fingerprinted() any {
	p.Recapped = domain.Recapped{}
	return p
}

// unchanged reports whether the plan stops nothing: no worktree runs a job it reaches.
func (p Plan) unchanged() bool {
	return !slices.ContainsFunc(p.Worktrees, func(t Target) bool { return len(t.Jobs) > 0 })
}

// targets are the worktrees of the plan as a stop takes them.
func (p Plan) targets() []runjobs.StopTarget {
	targets := make([]runjobs.StopTarget, 0, len(p.Worktrees))
	for _, t := range p.Worktrees {
		targets = append(targets, t.StopTarget)
	}
	return targets
}

// ----------------------------------------------------------------------------
// The recap

// recap is the plan row by row: the worktrees, the profile, then what stops in each worktree.
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
		jobs := []string{}
		for _, job := range t.Jobs {
			jobs = append(jobs, runjobs.StopWords(job))
		}
		if len(jobs) == 0 {
			jobs = append(jobs, "nothing running")
		}
		row := domain.RecapRow{Key: "jobs", Label: "Stops", Values: domain.RecapValues(jobs)}
		if len(p.Worktrees) > 1 {
			row.Key, row.Label = "jobs:"+t.Worktree, "Stops in "+t.Worktree
		}
		rows = append(rows, row)
	}
	return rows
}

// ----------------------------------------------------------------------------
// The outcome

// withNotRunning is item with the jobs of the profile t does not run, after those its stop reached.
func (t Target) withNotRunning(item Item) Item {
	for _, name := range t.NotRunning {
		item.Detail.Jobs = append(item.Detail.Jobs, runjobs.StopResult{Name: name, Status: runjobs.StopNotRunning})
	}
	return item
}

// CodeUp is the code of the follow-up that starts again what the stop stopped.
const CodeUp = Name + ".up"

// pathRunUpProfile is the path of run up's profile, which a follow-up of run up answers.
const pathRunUpProfile = "profile"

// next are the follow-ups of a stop: run up again in the worktrees still there that stopped or released a job, one per profile: the one the stop narrowed to, else the one each ran.
func (p Plan) next(items []Item) []domain.FollowUp {
	var profiles []string
	byProfile := map[string][]string{}
	for i, t := range p.Worktrees {
		if t.Removed || !runjobs.StoppedOrReleased(items[i]) {
			continue
		}
		profile := cmp.Or(p.Profile, t.Profile)
		if _, seen := byProfile[profile]; !seen {
			profiles = append(profiles, profile)
		}
		byProfile[profile] = append(byProfile[profile], t.Worktree)
	}

	followUps := make([]domain.FollowUp, 0, len(profiles))
	for _, profile := range profiles {
		answers := []domain.Answered{{Path: domain.PathWorktree, Values: byProfile[profile]}}
		if profile != "" {
			answers = append(answers, domain.Answered{Path: pathRunUpProfile, Values: []string{profile}})
		}
		followUps = append(followUps, domain.FollowUp{Command: domain.CommandRunUp, Answers: answers, Code: CodeUp, Message: "start them again"})
	}
	return followUps
}
