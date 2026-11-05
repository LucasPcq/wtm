package rundown

import (
	"cmp"
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what run down decides from, as Observe read them: the project's config, its run.toml (with why it does not load), the repository's snapshot, the worktrees whose directory is gone, where the caller stands, where each name asked as a path leads, and the jobs the run daemon runs (with a daemon of another build that answered, or Unknown: none answers and its index holds processes whose fate only a daemon can tell).
type Facts struct {
	worktree.Repository
	Gone    map[string]bool   // by path
	Cwd     string            // where the caller stands, through its symbolic links
	Located map[string]string // where each name asked that is no worktree's name leads, from where the caller stands, as git spells the worktree root it is; whether or not something is there
	Jobs    []run.Job
	Skewed  *run.Skewed
	Unknown bool
}

// standing is the worktree the caller stands in, by its name; none when it stands in none.
func (f Facts) standing() string {
	worktrees := f.standingWorktrees()
	if at := worktree.Holding(worktrees, f.Cwd, func(w worktree.Worktree) string { return w.Path }); at >= 0 {
		return worktrees[at].Name()
	}
	return ""
}

func (f Facts) standingWorktrees() []worktree.Worktree {
	return slices.DeleteFunc(f.MainFirst(), func(w worktree.Worktree) bool { return f.Gone[w.Path] })
}

// worktreeOptions are the worktrees run down may stop the jobs of: each one whose directory stands, the main one first, the caller's tagged current, each with what it runs.
func (f Facts) worktreeOptions() []domain.Option {
	worktrees := f.standingWorktrees()
	return domain.RunningIn(domain.WorktreeOptions(worktrees, domain.WorktreeQuery{Cwd: f.Cwd}), worktrees, f.Jobs)
}

// nameAt is how the worktree at path is named: its name, else the path itself (a worktree removed whose jobs still run).
func (f Facts) nameAt(path string) string {
	if at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Path == path }); at >= 0 {
		return f.Worktrees[at].Name()
	}
	return path
}

// ----------------------------------------------------------------------------
// The decision of each worktree

// A Target is a worktree whose jobs stop: its name, its branch (none on no branch, or for a worktree removed), its root as git spells it, whether git no longer lists it (Removed: a worktree removed whose jobs still run, named by its path), the profile its jobs were started under, the jobs the stop reaches, and, with a profile, the profile's jobs not running there.
type Target struct {
	Worktree   string    `json:"worktree"`
	Branch     string    `json:"branch,omitempty"`
	Path       string    `json:"path"`
	Removed    bool      `json:"removed,omitempty"`
	Profile    string    `json:"profile,omitempty"`
	Jobs       []JobPlan `json:"jobs"`
	NotRunning []string  `json:"not_running,omitempty"`
}

// A JobPlan is how one job of a worktree stops: its kind, what of it the daemon holds, its fate, the worktrees that keep holding a shared service it releases, the command a stack stops with as the daemon recorded it, and the main checkout a shared service runs in.
type JobPlan struct {
	Name        string          `json:"name"`
	Kind        project.JobKind `json:"kind,omitempty"`
	Hold        run.Hold        `json:"hold"`
	Fate        run.Fate        `json:"fate"`
	KeptBy      []string        `json:"kept_by,omitempty"`
	StopCommand string          `json:"stop_command,omitempty"`
	SharedIn    string          `json:"shared_in,omitempty"`
}

// shared reports whether the job is a hold on a shared service, let go of after the worktree's own jobs.
func (j JobPlan) shared() bool {
	return j.SharedIn != ""
}

// service is the shared service the job holds: its name in the main checkout it runs in.
func (j JobPlan) service() string {
	return j.SharedIn + " " + j.Name
}

// entries decides each worktree names asks: a Target, or the problem of its entry: blank, given twice (under one spelling or two), or naming neither a worktree of the repository (by its name, else the path it leads to) nor a worktree removed whose jobs still run.
func (f Facts) entries(names []string) (targets []Target, problems []domain.FieldError) {
	for index, name := range names {
		path := entryPath(index)
		target, found := f.target(name)
		switch {
		case strings.TrimSpace(name) == "":
			problems = append(problems, domain.Required(path))
		case slices.Contains(names[:index], name) || found && slices.ContainsFunc(targets, func(t Target) bool { return t.Path == target.Path }):
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeDistinct, Template: "{path}: {value} is given twice", Params: domain.Params{"value": name}})
		case found:
			targets = append(targets, target)
		case f.unattributed(f.Located[name]):
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeNotFound, Template: "{value} names no worktree: the run daemon runs jobs there without saying which repository they were started for", Params: domain.Params{"value": name}})
		default:
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeNotFound, Template: "{value} names no worktree", Params: domain.Params{"value": name}})
		}
	}
	return targets, problems
}

// target is the worktree name names, by its name, else the path it leads to: one of the repository, or one removed whose jobs still run.
func (f Facts) target(name string) (Target, bool) {
	located := f.Located[name]
	if at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Name() == name || located != "" && w.Path == located }); at >= 0 {
		w := f.Worktrees[at]
		return Target{Worktree: w.Name(), Branch: w.Branch(), Path: w.Path}, true
	}
	if f.removed(located) {
		return Target{Worktree: located, Path: located, Removed: true}, true
	}
	return Target{}, false
}

// removed reports whether path is a worktree of this repository git no longer lists whose jobs still run: the daemon runs a job there started for this repository. A path of another repository's is never one, the daemon's listing being the machine's.
func (f Facts) removed(path string) bool {
	return path != "" && slices.ContainsFunc(f.Jobs, func(job run.Job) bool { return job.WorkDir == path && job.Repo == f.MainPath })
}

// unattributed reports whether the run daemon runs a job at path whose start named no repository (indexed by a wtm that recorded none): run down cannot tell it is this repository's.
func (f Facts) unattributed(path string) bool {
	return path != "" && slices.ContainsFunc(f.Jobs, func(job run.Job) bool { return job.WorkDir == path && job.Repo == "" })
}

// every is each worktree of the repository the run daemon lists a job of, the main one first, then each removed one whose jobs still run, by path.
func (f Facts) every() []Target {
	var targets []Target
	for _, w := range f.MainFirst() {
		if slices.ContainsFunc(f.Jobs, func(job run.Job) bool { return job.WorkDir == w.Path }) {
			targets = append(targets, Target{Worktree: w.Name(), Branch: w.Branch(), Path: w.Path})
		}
	}

	var removed []string
	for _, job := range f.Jobs {
		listed := slices.ContainsFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Path == job.WorkDir })
		if !listed && !slices.Contains(removed, job.WorkDir) && f.removed(job.WorkDir) {
			removed = append(removed, job.WorkDir)
		}
	}
	slices.Sort(removed)
	for _, path := range removed {
		targets = append(targets, Target{Worktree: path, Path: path, Removed: true})
	}
	return targets
}

// stopping is what the stop of targets does to each of their jobs, by worktree path, the profile's alone when profile is set: decided over every target at once, so the holds of several on one shared service are let go of together.
func (f Facts) stopping(targets []Target, profile string) map[string][]run.Stopping {
	dirs := make([]string, 0, len(targets))
	for _, t := range targets {
		dirs = append(dirs, t.Path)
	}
	byDir := map[string][]run.Stopping{}
	for _, s := range run.StoppingIn(f.Jobs, dirs, f.profileJobs(profile)) {
		byDir[s.Job.WorkDir] = append(byDir[s.Job.WorkDir], s)
	}
	return byDir
}

// profileJobs are the jobs of the profile of name, nil for none.
func (f Facts) profileJobs(name string) []string {
	if at := slices.IndexFunc(f.Run.Profiles, func(p project.Profile) bool { return p.Name == name }); at >= 0 {
		return append([]string{}, f.Run.Profiles[at].Jobs...)
	}
	return nil
}

// plan is how the jobs of t stop: each of stopping with its fate, and the jobs of profile it does not reach.
func (f Facts) plan(t Target, stopping []run.Stopping, profile string) Target {
	t.Profile, t.Jobs = run.ProfileIn(f.Jobs, t.Path), []JobPlan{}
	for _, s := range stopping {
		job := s.Job
		planned := JobPlan{Name: job.Name, Kind: job.Kind, Hold: job.Hold, Fate: s.Fate, StopCommand: job.StopCommand}
		for _, holder := range s.KeptBy {
			planned.KeptBy = append(planned.KeptBy, f.nameAt(holder))
		}
		if job.SharedDir != "" {
			planned.SharedIn = f.nameAt(job.SharedDir)
		}
		t.Jobs = append(t.Jobs, planned)
		// A worktree whose only jobs stopped are its holds on shared services is started again under their profile.
		t.Profile = cmp.Or(t.Profile, job.Profile)
	}
	up := run.UpIn(f.Jobs, t.Path)
	for _, name := range f.profileJobs(profile) {
		if !slices.Contains(up, name) && !slices.ContainsFunc(t.Jobs, func(job JobPlan) bool { return job.Name == name }) {
			t.NotRunning = append(t.NotRunning, name)
		}
	}
	return t
}

// ----------------------------------------------------------------------------
// Decide

// The codes of what a plan cannot see to: a run daemon of another build answered, whose listing may not say which worktrees hold a shared service (the fates of the shared services are then approximate); run.toml does not load.
const (
	CodeDaemonSkew       = Name + domain.DaemonSkew
	CodeFatesApproximate = Name + ".fates_approximate"
	CodeRunInvalid       = Name + ".run_invalid"
)

// DecideParams are what Decide decides from besides the facts and the request: the follow-up that restarts a daemon of another build.
type DecideParams struct {
	Restart *domain.FollowUp
}

// Decide turns the facts and the resolved request into the plan and its warnings: each worktree's jobs, with what the stop does to each.
func Decide(f Facts, req Request, params DecideParams) (Plan, []domain.Warning) {
	targets, _ := f.entries(req.Worktrees)
	if req.All {
		targets = f.every()
	}
	planned := Plan{Profile: req.Profile, Worktrees: make([]Target, 0, len(targets))}
	stopping := f.stopping(targets, req.Profile)
	for _, t := range targets {
		planned.Worktrees = append(planned.Worktrees, f.plan(t, stopping[t.Path], req.Profile))
	}

	var warnings []domain.Warning
	if f.Skewed != nil {
		warnings = append(warnings, repository.SkewWarning(repository.SkewWarningParams{Command: Name, Skewed: f.Skewed, Restart: params.Restart}))
		if f.approximate(targets) {
			warnings = append(warnings, domain.Warning{Code: CodeFatesApproximate, Message: "a run daemon of another build does not say whether the main checkout holds a shared service: which ones stop and which run on is approximate"})
		}
	}
	if f.RunErr != nil {
		warnings = append(warnings, domain.Warning{Code: CodeRunInvalid, Message: "invalid run.toml: " + f.RunErr.Error() + " — the jobs the run daemon runs stop all the same"})
	}
	return planned, warnings
}

// approximate reports whether a worktree of targets holds a shared service whether the main checkout holds which cannot be told (run.Job.MainHoldsUnknown): which holds are left, and so the fates, are then guessed.
func (f Facts) approximate(targets []Target) bool {
	return slices.ContainsFunc(f.Jobs, func(service run.Job) bool {
		return service.Up && service.MainHoldsUnknown && slices.ContainsFunc(f.Jobs, func(hold run.Job) bool {
			return hold.Up && hold.Name == service.Name && hold.SharedDir == service.SharedDir && slices.ContainsFunc(targets, func(t Target) bool { return t.Path == hold.WorkDir })
		})
	})
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
			jobs = append(jobs, jobWords(job))
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

// jobWords is how the recap names one job: its name, and what the stop does to it unless it just stops.
func jobWords(job JobPlan) string {
	switch {
	case job.Fate == run.FateRelease:
		return job.Name + " (released, kept by " + strings.Join(job.KeptBy, ", ") + ")"
	case job.Fate == run.FateClear:
		return job.Name + " (crashed, cleared)"
	case job.shared():
		return job.Name + " (stops in " + job.SharedIn + ": no worktree keeps it)"
	}
	return job.Name
}

// ----------------------------------------------------------------------------
// The outcome

// JobStatus is what became of one job of the stop.
type JobStatus string

// The statuses of a job of the stop.
const (
	JobStopped    JobStatus = "stopped"     // the job was up and is stopped, a shared service by its last hold included
	JobReleased   JobStatus = "released"    // the worktree let go of a shared service others still hold, which runs on
	JobCleared    JobStatus = "cleared"     // a job that had crashed, cleared from the listing
	JobNotRunning JobStatus = "not_running" // a job not up there: ended since the plan, or a job of the profile that never ran
	JobFailed     JobStatus = "failed"      // a stop the daemon could not make: the job runs on
	JobNotReached JobStatus = "not_reached" // a stop never sent: the run was cancelled first
)

// JobStatuses returns every JobStatus.
func JobStatuses() []JobStatus {
	return []JobStatus{JobStopped, JobReleased, JobCleared, JobNotRunning, JobFailed, JobNotReached}
}

// statusOf is what a stop that answered outcome did to a job planned to meet fate: what the daemon answered, a crash it cleared (answered not running) said cleared.
func statusOf(fate run.Fate, outcome run.Outcome) JobStatus {
	switch {
	case outcome == run.OutcomeStopped:
		return JobStopped
	case fate == run.FateClear:
		return JobCleared
	case outcome == run.OutcomeNotRunning:
		return JobNotRunning
	case outcome == run.OutcomeReleased:
		return JobReleased
	}
	return JobStopped
}

// settled is items once the holds the run let go of together are told apart: of the holds on one shared service planned to stop it, whichever lets go first answers released, the service stopping at the last; each reads stopped once one of them answered stopped. A service someone else came to hold meanwhile was released by them all.
func (p Plan) settled(items []Item) []Item {
	stopped := map[string]bool{}
	for i, t := range p.Worktrees {
		for j, job := range t.Jobs {
			if job.shared() && items[i].Detail.Jobs[j].Status == JobStopped {
				stopped[job.service()] = true
			}
		}
	}
	for i, t := range p.Worktrees {
		for j, job := range t.Jobs {
			if result := &items[i].Detail.Jobs[j]; job.shared() && job.Fate == run.FateStop && result.Status == JobReleased && stopped[job.service()] {
				result.Status = JobStopped
			}
		}
	}
	return items
}

// notReached is the detail of t before its stop: each job of its plan, none stopped, and those of the profile not running.
func (t Target) notReached() *Detail {
	detail := &Detail{Path: t.Path, Jobs: make([]JobResult, 0, len(t.Jobs)+len(t.NotRunning))}
	for _, job := range t.Jobs {
		detail.Jobs = append(detail.Jobs, JobResult{Name: job.Name, Status: JobNotReached})
	}
	for _, name := range t.NotRunning {
		detail.Jobs = append(detail.Jobs, JobResult{Name: name, Status: JobNotRunning})
	}
	return detail
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
		if t.Removed || !slices.ContainsFunc(items[i].Detail.Jobs, func(job JobResult) bool { return job.Status == JobStopped || job.Status == JobReleased }) {
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
