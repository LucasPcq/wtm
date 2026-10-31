package runup

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what run up decides from, as Observe read them: the project's config, its run.toml, the repository's snapshot, the meta record of each worktree on a branch and why one that exists does not read, the worktrees whose directory is gone, where the caller stands, and the jobs the run daemon runs (with why it could not tell, or that one of another build answered).
type Facts struct {
	worktree.Repository
	Records    map[string]worktree.Meta // by branch
	RecordErrs map[string]error         // by branch
	Gone       map[string]bool          // by path
	Cwd        string                   // where the caller stands, through its symbolic links
	Jobs       []run.Job
	JobsErr    error
	Skewed     *run.Skewed
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

// worktreeOptions are the worktrees run up may start in: each one whose directory stands, the main one first, the caller's tagged current, each with the jobs up in it and the profile they were started under.
func (f Facts) worktreeOptions() []domain.Option {
	worktrees := f.standingWorktrees()
	options := domain.WorktreeOptions(worktrees, domain.WorktreeQuery{Cwd: f.Cwd})
	for i, w := range worktrees {
		options[i].Up = nonEmptyNames(run.UpIn(f.Jobs, w.Path))
		options[i].Profile = profileOf(f.up(w.Path))
	}
	return options
}

// up are the jobs up in the worktree at path, claims included.
func (f Facts) up(path string) []run.Job {
	return slices.DeleteFunc(slices.Clone(f.Jobs), func(job run.Job) bool { return !job.Up || job.WorkDir != path })
}

// nonEmptyNames is names, nil when it holds none, as an option leaves the fact out.
func nonEmptyNames(names []string) []string {
	if len(names) == 0 {
		return nil
	}
	return names
}

// profileOf is the profile jobs were started under, when they say one.
func profileOf(jobs []run.Job) string {
	for _, job := range jobs {
		if job.Profile != "" {
			return job.Profile
		}
	}
	return ""
}

func (f Facts) named(name string) (worktree.Worktree, bool) {
	at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Name() == name })
	if at < 0 {
		return worktree.Worktree{}, false
	}
	return f.Worktrees[at], true
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
		if declared, ok := f.declared(job); ok {
			jobs = append(jobs, declared)
		}
	}
	return jobs
}

func (f Facts) declared(name string) (project.Job, bool) {
	at := slices.IndexFunc(f.Run.Jobs, func(job project.Job) bool { return job.Name == name })
	if at < 0 {
		return project.Job{}, false
	}
	return f.Run.Jobs[at], true
}

// urlOf is the URL the daemon lists for the job of name up in the worktree at path.
func (f Facts) urlOf(name, path string) string {
	for _, job := range f.up(path) {
		if job.Name == name {
			return job.URL
		}
	}
	return ""
}

// shared reports whether the job of name runs once for every worktree.
func (f Facts) shared(name string) bool {
	job, ok := f.declared(name)
	return ok && job.Scope == project.Shared
}

// children are the declared jobs a runner starts itself, its runners' own included, each once.
func (f Facts) children(runner project.Job) []project.Job {
	seen := map[string]bool{runner.Name: true}
	queue := slices.Clone(runner.Runs)
	var children []project.Job
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		child, ok := f.declared(name)
		if seen[name] || !ok {
			continue
		}
		seen[name] = true
		children = append(children, child)
		queue = append(queue, child.Runs...)
	}
	return children
}

// ----------------------------------------------------------------------------
// The decision of each worktree

// A Target is a worktree the profile starts in: its name, its branch (none on no branch) and its root as git spells it; whether it is the main one; whether its ports are known (Numbered: one whose ports derive from an ordinal it lacks is numbered first); and its jobs, in the order they start.
type Target struct {
	Worktree string    `json:"worktree"`
	Branch   string    `json:"branch,omitempty"`
	Path     string    `json:"path"`
	Main     bool      `json:"main,omitempty"`
	Numbered bool      `json:"numbered"`
	Jobs     []JobPlan `json:"jobs"`
	entry    int
}

// A JobPlan is how one job of the profile starts in its worktree: what the start does (Action); its Step among the jobs the worktree starts, none for a shared job it joins; the tasks it waits for, which run before it; the main checkout's branch a shared job runs in; the ports it binds, offset for the worktree; and the namespace of its data a shared job makes for the worktree.
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
	ActionStart Action = "start" // the job starts: a service not up, a task, which runs each time
	ActionUp    Action = "up"    // a service already up in the worktree, left as it is
	ActionJoin  Action = "join"  // a shared service the worktree holds, run once in the main checkout, started there when it is not up
)

// Actions returns every Action.
func Actions() []Action {
	return []Action{ActionStart, ActionUp, ActionJoin}
}

// entries decides each worktree names asks: a Target, or the problem of its entry: blank, given twice, gone, a record that does not read, or naming no worktree.
func (f Facts) entries(names []string) (targets []Target, problems []domain.FieldError) {
	for index, name := range names {
		path := entryPath(index)
		w, found := f.named(name)
		switch {
		case strings.TrimSpace(name) == "":
			problems = append(problems, domain.Required(path))
		case slices.Contains(names[:index], name):
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeDistinct, Template: "{path}: {value} is given twice", Params: domain.Params{"value": name}})
		case found && f.Gone[w.Path]:
			problems = append(problems, domain.FieldError{Path: path, Code: CodeWorktreeGone, Template: "{value}'s directory is gone: nothing to start in", Params: domain.Params{"value": name, "path": w.Path}})
		case found && f.RecordErrs[w.Branch()] != nil:
			problems = append(problems, domain.FieldError{Path: path, Code: CodeMetaUnreadable, Template: "{value}'s record does not read: its ports are unknown", Params: domain.Params{"value": name, "cause": f.RecordErrs[w.Branch()].Error()}})
		case found:
			record := f.Records[w.Branch()]
			numbered := w.Main || record.Ordinal > 0 || record.Isolation.Effective() == project.Verbatim || !f.Run.IsolationApplies()
			targets = append(targets, Target{Worktree: w.Name(), Branch: w.Branch(), Path: w.Path, Main: w.Main, Numbered: numbered, entry: index})
		default:
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeNotFound, Template: "{value} names no worktree", Params: domain.Params{"value": name}})
		}
	}
	return targets, problems
}

// jobParams is what the job environment of t reads, its record's ordinal and isolation; the main worktree's ordinal is 0.
func (f Facts) jobParams(t Target, ordinal int) env.JobParams {
	record := f.Records[t.Branch]
	return env.JobParams{Branch: t.Branch, ProjectDir: f.MainPath, Ordinal: ordinal, Isolation: record.Isolation.Effective(), Run: f.Run}
}

func (f Facts) ordinalOf(t Target) int {
	return f.Records[t.Branch].Ordinal
}

// plan is how t starts jobs: each in order, a shared one joined from a worktree other than the main one, a service up left as it is, the others numbered, each waiting for the tasks before it; the ports from the environment its ordinal gives, none while it has none.
func (f Facts) plan(t Target, jobs []project.Job) Target {
	params := f.jobParams(t, f.ordinalOf(t))
	jobEnv := env.HookEnv(params, nil)
	up := run.UpIn(f.Jobs, t.Path)
	main := f.mainBranch()
	step := 0
	var tasks []string
	t.Jobs = []JobPlan{}
	for _, job := range jobs {
		planned := JobPlan{Name: job.Name, Kind: job.Kind, Action: ActionStart, WaitsFor: slices.Clone(tasks)}
		if t.Numbered {
			planned.Ports = nonEmpty(params.JobPorts(f.effective(job)))
		}
		switch {
		case job.Scope == project.Shared && !t.Main:
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

func (f Facts) mainBranch() string {
	return f.mainTarget().Worktree
}

// effective is job as it starts: with the ports of the jobs it runs itself, which it hands their environment.
func (f Facts) effective(job project.Job) project.Job {
	children := f.children(job)
	if len(children) == 0 {
		return job
	}
	job.Ports = maps.Clone(job.Ports)
	if job.Ports == nil {
		job.Ports = map[string]int{}
	}
	for _, child := range children {
		for name, base := range child.Ports {
			if _, own := job.Ports[name]; !own {
				job.Ports[name] = base
			}
		}
	}
	return job
}

// nonEmpty is ports, nil when it holds none.
func nonEmpty(ports map[string]int) map[string]int {
	if len(ports) == 0 {
		return nil
	}
	return ports
}

// ----------------------------------------------------------------------------
// The other worktrees

// An Other is a worktree outside the request that runs jobs: by its name and root, with the jobs up in it, which an exclusive start stops.
type Other struct {
	Worktree string   `json:"worktree"`
	Path     string   `json:"path"`
	Jobs     []string `json:"jobs"`
}

// others are the worktrees outside targets with jobs up, in git's order.
func (f Facts) others(targets []Target) []Other {
	var others []Other
	for _, w := range f.MainFirst() {
		if slices.ContainsFunc(targets, func(t Target) bool { return t.Path == w.Path }) {
			continue
		}
		if up := run.UpIn(f.Jobs, w.Path); len(up) > 0 {
			others = append(others, Other{Worktree: w.Name(), Path: w.Path, Jobs: up})
		}
	}
	return others
}

// ----------------------------------------------------------------------------
// Foreign data

// A risk is a job a start runs that changes data its worktree does not own: the job (Via, the runner that starts it, if one does), the Service whose data it changes and whose data that is (Owner).
type risk struct {
	Worktree string
	Job      string
	Via      string
	Service  string
	Owner    owner
	entry    int
}

// owner is whose data a job's touches reach.
type owner string

// The owners of foreign data.
const (
	ownerSource   owner = "source"   // a verbatim worktree's source's: the two share their data
	ownerEveryone owner = "everyone" // a shared service's with no namespace: every worktree's
)

// risks are the jobs the start of jobs in targets runs, those their runners start included, that change data a worktree does not own; the main worktree owns its own.
func (f Facts) risks(targets []Target, jobs []project.Job) []risk {
	var risks []risk
	for _, t := range targets {
		if t.Main {
			continue
		}
		verbatim := f.Records[t.Branch].Isolation.Effective() == project.Verbatim
		for _, started := range f.touchers(jobs) {
			for _, service := range started.job.Touches {
				declared, _ := f.declared(service)
				owner := ownerEveryone
				switch {
				case verbatim:
					owner = ownerSource
				case declared.Scope != project.Shared || declared.Namespace != nil:
					continue
				}
				risks = append(risks, risk{Worktree: t.Worktree, Job: started.job.Name, Via: started.via, Service: service, Owner: owner, entry: t.entry})
			}
		}
	}
	return risks
}

type toucher struct {
	job project.Job
	via string
}

// touchers are the jobs a start runs: those it names, then those their runners start, each once.
func (f Facts) touchers(jobs []project.Job) []toucher {
	seen := map[string]bool{}
	var started []toucher
	for _, job := range jobs {
		if !seen[job.Name] {
			seen[job.Name] = true
			started = append(started, toucher{job: job})
		}
	}
	for _, job := range jobs {
		for _, child := range f.children(job) {
			if !seen[child.Name] {
				seen[child.Name] = true
				started = append(started, toucher{job: child, via: job.Name})
			}
		}
	}
	return started
}

// ----------------------------------------------------------------------------
// Port clashes

// A clash is a port a worktree's job would bind that a job of another worktree holds already, which only a worktree sharing another's ports meets (a verbatim one, beside its source).
type clash struct {
	Port     int
	Worktree string
	Job      string
	HeldBy   string
	HeldJob  string
	Started  bool // HeldBy is another worktree of the request, whose start would bind it too: no exclusive start lifts it
}

// clashes are the ports the targets would bind that a job holds in another worktree, or that two targets would both bind; shared jobs bind theirs once, and clash with nobody.
func (f Facts) clashes(targets []Target) []clash {
	type holder struct{ worktree, job string }
	held := map[int]holder{}
	for _, job := range f.Jobs {
		declared, ok := f.declared(job.Name)
		if !job.Up || job.Hold == run.HoldClaim || (ok && declared.Scope == project.Shared) {
			continue
		}
		if slices.ContainsFunc(targets, func(t Target) bool { return t.Path == job.WorkDir }) {
			continue
		}
		w := f.worktreeAt(job.WorkDir)
		for _, port := range job.Ports {
			held[port] = holder{worktree: w, job: job.Name}
		}
	}

	var clashes []clash
	starting := map[int]holder{}
	for _, t := range targets {
		for _, job := range t.Jobs {
			if job.Action != ActionStart {
				continue
			}
			for _, port := range sortedPorts(job.Ports) {
				if h, ok := held[port]; ok {
					clashes = append(clashes, clash{Port: port, Worktree: t.Worktree, Job: job.Name, HeldBy: h.worktree, HeldJob: h.job})
				}
				if h, ok := starting[port]; ok && h.worktree != t.Worktree {
					clashes = append(clashes, clash{Port: port, Worktree: t.Worktree, Job: job.Name, HeldBy: h.worktree, HeldJob: h.job, Started: true})
				}
				starting[port] = holder{worktree: t.Worktree, job: job.Name}
			}
		}
	}
	return clashes
}

func sortedPorts(ports map[string]int) []int {
	return slices.Sorted(maps.Values(ports))
}

// worktreeAt is the name of the worktree at path, the path itself when no worktree is there.
func (f Facts) worktreeAt(path string) string {
	if at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Path == path }); at >= 0 {
		return f.Worktrees[at].Name()
	}
	return path
}

// ----------------------------------------------------------------------------
// Runners

// A conflict is a job a start would run twice: started by name, and by a Runner the same start runs, which starts it itself.
type conflict struct {
	Job    string
	Runner string
}

// conflicts are the jobs that would run twice once the start is done: one of the profile, or up in a target, that a runner among them starts too.
func (f Facts) conflicts(targets []Target, jobs []project.Job) []conflict {
	starting := map[string]bool{}
	for _, job := range jobs {
		starting[job.Name] = true
	}
	for _, t := range targets {
		for _, name := range run.UpIn(f.Jobs, t.Path) {
			starting[name] = true
		}
	}

	var conflicts []conflict
	for _, runner := range f.Run.Jobs {
		if !starting[runner.Name] {
			continue
		}
		for _, child := range f.children(runner) {
			if starting[child.Name] {
				conflicts = append(conflicts, conflict{Job: child.Name, Runner: runner.Name})
			}
		}
	}
	return conflicts
}

// ----------------------------------------------------------------------------
// Decide

// The codes of the refusals of a request: a job that changes foreign data (CodeUnsafe, followed by CodeForce, the request forced), a port another worktree holds (CodePortClash, followed by CodeExclusive, the request stopping the others first), a job a runner of the same start starts too (CodeRunnerConflict), a run daemon of another build holding jobs (CodeDaemonSkew, followed by the restart), and every job of the request already up as asked is no refusal.
const (
	CodeUnsafe         = Name + ".unsafe"
	CodeForeignData    = Name + ".foreign_data"
	CodeForce          = Name + ".force"
	CodePortClash      = Name + ".port_clash"
	CodeExclusive      = Name + ".exclusive"
	CodeRunnerConflict = Name + ".runner_conflict"
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

// Decide turns the facts and the resolved request into the plan and its warnings, or the refusal that stops it: a run daemon of another build holding jobs, a job a runner of the start starts too, a port another worktree holds unless its jobs are stopped first, a job that changes data a worktree does not own unless forced.
func Decide(f Facts, req Request, params DecideParams) (Plan, []domain.Warning, error) {
	targets, _ := f.entries(req.Worktrees)
	jobs := f.jobsOf(req.Profile)
	exclusive := f.exclusive(req, targets).Value
	planned := Plan{Profile: req.Profile, Exclusive: exclusive, Worktrees: []Target{}, Others: []Other{}, NoProbe: req.NoProbe, facts: f, asked: req}
	for _, t := range targets {
		planned.Worktrees = append(planned.Worktrees, f.plan(t, jobs))
	}
	if exclusive {
		planned.Others = append(planned.Others, f.others(targets)...)
	}

	if f.Skewed != nil && slices.ContainsFunc(f.Jobs, func(job run.Job) bool { return job.Up }) {
		return Plan{}, nil, skewRefusal(f.Skewed, params.Restart)
	}
	if conflicts := f.conflicts(targets, jobs); len(conflicts) > 0 {
		return Plan{}, nil, conflictRefusal(conflicts)
	}
	if clashes := f.clashes(planned.Worktrees); len(clashes) > 0 && !(exclusive && !slices.ContainsFunc(clashes, func(c clash) bool { return c.Started })) {
		return Plan{}, nil, clashRefusal(clashes, req)
	}
	if risks := f.risks(targets, jobs); len(risks) > 0 && !req.Force {
		return Plan{}, nil, foreignRefusal(risks, req)
	}

	return planned, f.warnings(planned, req), nil
}

func (f Facts) warnings(p Plan, req Request) []domain.Warning {
	var warnings []domain.Warning
	if f.JobsErr != nil {
		warnings = append(warnings, domain.Warning{Code: CodeJobsUnknown, Message: run.UnknownMessage(f.JobsErr) + " — the start leaves the other worktrees' jobs alone"})
	}
	if req.Exclusive == nil && f.Run.Concurrency == project.Exclusive && len(p.Worktrees) > 1 && len(f.others(p.Worktrees)) > 0 {
		warnings = append(warnings, domain.Warning{Code: CodeExclusiveIgnored, Message: "run.toml stops the other worktrees' jobs first, which cannot start several: they keep running"})
	}
	return warnings
}

func skewRefusal(skewed *run.Skewed, restart *domain.FollowUp) *domain.Error {
	refusal := &domain.Error{Kind: domain.KindPrecondition, Code: CodeDaemonSkew, Message: "nothing started: " + skewed.Error(), Params: domain.Params{"skew": string(skewed.Skew), "daemon": skewed.Daemon}}
	if restart != nil {
		followUp := *restart
		followUp.Code, followUp.Message = Name+".restart_daemon", "hand its jobs over to this wtm"
		refusal.FollowUp = &followUp
	}
	return refusal
}

func conflictRefusal(conflicts []conflict) *domain.Error {
	fields := make([]domain.FieldError, 0, len(conflicts))
	for _, c := range conflicts {
		fields = append(fields, domain.FieldError{Path: PathProfile, Code: CodeRunnerConflict, Template: "{job} would run twice: {runner} starts it too", Params: domain.Params{"job": c.Job, "runner": c.Runner}})
	}
	return domain.Invalid(fields)
}

func clashRefusal(clashes []clash, req Request) *domain.Error {
	var lines []string
	fields := make([]domain.FieldError, 0, len(clashes))
	lifted := true
	for _, c := range clashes {
		port := strconv.Itoa(c.Port)
		lines = append(lines, port+" ("+c.HeldBy+"'s "+c.HeldJob+")")
		fields = append(fields, domain.FieldError{Path: domain.PathWorktree, Code: CodePortClash, Template: "{worktree}'s {job} would bind {port}, which {held_by}'s {held_job} holds", Params: domain.Params{"worktree": c.Worktree, "job": c.Job, "port": port, "held_by": c.HeldBy, "held_job": c.HeldJob}})
		lifted = lifted && !c.Started
	}
	refusal := &domain.Error{Kind: domain.KindRefused, Code: CodePortClash, Message: "nothing started: ports another worktree holds: " + strings.Join(lines, ", "), Fields: fields}
	if lifted {
		stopping := req
		yes := true
		stopping.Exclusive, stopping.Parallel, stopping.Caller = &yes, false, domain.Caller{}
		refusal.FollowUp = &domain.FollowUp{Command: Name, Answers: domain.AnswersOf(Questions{}.Form(), stopping), Code: CodeExclusive, Message: "stop the other worktrees' jobs first"}
	}
	return refusal
}

// ownerWords say whose data an owner's is, after "changes".
var ownerWords = map[owner]string{ownerSource: "its source's data: it is verbatim", ownerEveryone: "every worktree's data: it has no namespace"}

func foreignRefusal(risks []risk, req Request) *domain.Error {
	fields := make([]domain.FieldError, 0, len(risks))
	var worktrees []string
	for _, r := range risks {
		job := r.Job
		if r.Via != "" {
			job += " (run by " + r.Via + ")"
		}
		fields = append(fields, domain.FieldError{Path: entryPath(r.entry), Code: CodeForeignData, Template: "{value}: {job} changes {service}, " + ownerWords[r.Owner], Params: domain.Params{"value": r.Worktree, "job": job, "service": r.Service, "owner": string(r.Owner)}})
		if !slices.Contains(worktrees, r.Worktree) {
			worktrees = append(worktrees, r.Worktree)
		}
	}
	forced := req
	forced.Force, forced.Caller = true, domain.Caller{}
	return &domain.Error{
		Kind:     domain.KindRefused,
		Code:     CodeUnsafe,
		Message:  "nothing started: a job changes data " + strings.Join(worktrees, ", ") + " does not own",
		Params:   domain.Params{"worktrees": strings.Join(worktrees, ", ")},
		Fields:   fields,
		FollowUp: &domain.FollowUp{Command: Name, Answers: domain.AnswersOf(Questions{}.Form(), forced), Code: CodeForce, Message: "start them anyway"},
	}
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: the profile started (none when run.toml declares none: every job), in each worktree, in order; whether the other worktrees' jobs are stopped first, and those worktrees; whether the ports go unchecked; then the recap, which the fingerprint leaves out.
type Plan struct {
	Profile   string   `json:"profile"`
	Worktrees []Target `json:"worktrees"`
	Exclusive bool     `json:"exclusive"`
	Others    []Other  `json:"others"`
	NoProbe   bool     `json:"no_probe"`
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

// recap is the plan row by row: the worktrees, the profile, the jobs each starts, in order, and the worktrees whose jobs stop first.
func (p Plan) recap() []domain.RecapRow {
	names := make([]string, 0, len(p.Worktrees))
	for _, t := range p.Worktrees {
		names = append(names, t.Worktree)
	}
	rows := []domain.RecapRow{{Key: "worktrees", Label: "Worktrees", Path: domain.PathWorktree, Values: domain.RecapValues(names)}}
	if p.Profile != "" {
		rows = append(rows, domain.RecapRow{Key: "profile", Label: "Profile", Path: PathProfile, Values: domain.RecapValues([]string{p.Profile})})
	}
	if len(p.Worktrees) > 0 {
		var jobs []string
		for _, job := range p.Worktrees[0].Jobs {
			jobs = append(jobs, jobWords(job))
		}
		rows = append(rows, domain.RecapRow{Key: "jobs", Label: "Jobs", Values: domain.RecapValues(jobs)})
	}
	if len(p.Others) > 0 {
		var stopped []string
		for _, other := range p.Others {
			stopped = append(stopped, other.Worktree)
		}
		rows = append(rows, domain.RecapRow{Key: "stops", Label: "Stops first", Path: PathExclusive, Values: domain.RecapValues(stopped)})
	}
	return rows
}

// unchanged reports whether the plan starts nothing and stops nothing: every job is up already.
func (p Plan) unchanged() bool {
	for _, t := range p.Worktrees {
		if slices.ContainsFunc(t.Jobs, func(job JobPlan) bool { return job.Action != ActionUp }) {
			return false
		}
	}
	return len(p.Others) == 0
}

// jobWords is how the recap names one job: its name, and what it does unless it starts.
func jobWords(job JobPlan) string {
	switch job.Action {
	case ActionUp:
		return job.Name + " (up)"
	case ActionJoin:
		return job.Name + " (joined, in " + job.SharedIn + ")"
	}
	return job.Name
}

// ----------------------------------------------------------------------------
// The outcome

// JobStatus is what became of one job of the start.
type JobStatus string

// The statuses of a job of the start.
const (
	JobStarted    JobStatus = "started"     // a service launched, or a stack its launcher brought up
	JobUp         JobStatus = "up"          // a service already up, left as it is
	JobJoined     JobStatus = "joined"      // a shared service the worktree holds
	JobDone       JobStatus = "done"        // a task that ran to its end
	JobFailed     JobStatus = "failed"      // a task that exited other than 0, or a job that could not start: the start of its worktree stops there
	JobCrashed    JobStatus = "crashed"     // a service that started then ended before the start did
	JobNotStarted JobStatus = "not_started" // a job the start of its worktree never reached
)

// JobStatuses returns every JobStatus.
func JobStatuses() []JobStatus {
	return []JobStatus{JobStarted, JobUp, JobJoined, JobDone, JobFailed, JobCrashed, JobNotStarted}
}

// The codes of the follow-ups of a start: its jobs' output, the stop of what it left up, and the start again where it did not pass.
const (
	CodeLogs  = Name + ".logs"
	CodeDown  = Name + ".down"
	CodeRetry = Name + ".retry"
)

// The commands the follow-ups of a start name, not migrated yet.
const (
	commandRunLogs = "run logs"
	commandRunDown = "run down"
)

// applied is the plan once applied, as Run reports it: its recap, then where each job of each worktree answers.
func (p Plan) applied(result Result) Plan {
	p.Recap = p.recap()
	for _, item := range result.Items {
		if item.Detail == nil {
			continue
		}
		var reached []string
		for _, job := range item.Detail.Jobs {
			if job.URL != "" && (job.Status == JobStarted || job.Status == JobUp) {
				reached = append(reached, job.Name+"  "+job.URL)
			}
		}
		if len(reached) > 0 {
			p.Recap = append(p.Recap, domain.RecapRow{Key: "reach", Label: item.Subject, Values: domain.RecapValues(reached)})
		}
	}
	return p
}

// next are the follow-ups of a start: the start again where it did not pass, then each worktree's output and the stop of what it left up.
func (p Plan) next(items []Item) []domain.FollowUp {
	var followUps []domain.FollowUp
	if retry := p.retry(items); retry != nil {
		followUps = append(followUps, *retry)
	}
	for _, item := range items {
		if item.Detail == nil || !item.Detail.leftUp() {
			continue
		}
		on := []domain.Answered{{Path: domain.PathWorktree, Values: []string{item.Subject}}}
		followUps = append(followUps,
			domain.FollowUp{Command: commandRunLogs, Answers: on, Code: CodeLogs, Message: "follow " + item.Subject + "'s output"},
			domain.FollowUp{Command: commandRunDown, Answers: on, Code: CodeDown, Message: "stop " + item.Subject + "'s jobs"},
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
