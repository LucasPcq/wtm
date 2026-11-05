package runjobs

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// The other worktrees

// An Other is a worktree outside the request that runs jobs: by its name and root, with the jobs up in it, which an exclusive start stops.
type Other struct {
	Worktree string   `json:"worktree"`
	Path     string   `json:"path"`
	Jobs     []string `json:"jobs"`
}

// Others are the worktrees outside targets with jobs up, in git's order.
func (f Facts) Others(targets []Target) []Other {
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

// Risky reports whether the start of jobs in targets runs a job that changes data a worktree does not own, which only a forced start starts.
func (f Facts) Risky(targets []Target, jobs []project.Job) bool {
	return len(f.risks(targets, jobs)) > 0
}

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
				declared, _ := f.Declared(service)
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
	Foreign  bool // HeldBy is a worktree of another repository, by its path, whose jobs no exclusive start stops
}

// lifts reports whether stopping the other worktrees' jobs first frees the port.
func (c clash) lifts() bool {
	return !c.Started && !c.Foreign
}

// clashes are the ports the targets would bind that a job holds in another worktree, of this repository or another, or that two targets would both bind; this repository's shared jobs bind theirs once, and clash with nobody.
func (f Facts) clashes(targets []Target) []clash {
	type holder struct {
		worktree, job string
		foreign       bool
	}
	held := map[int]holder{}
	for _, job := range f.Jobs {
		_, ours := f.atPath(job.WorkDir)
		if !job.Up || job.Hold == run.HoldClaim || (ours && f.Shared(job.Name)) {
			continue
		}
		if slices.ContainsFunc(targets, func(t Target) bool { return t.Path == job.WorkDir }) {
			continue
		}
		for _, port := range job.Ports {
			held[port] = holder{worktree: f.worktreeAt(job.WorkDir), job: job.Name, foreign: !ours}
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
					clashes = append(clashes, clash{Port: port, Worktree: t.Worktree, Job: job.Name, HeldBy: h.worktree, HeldJob: h.job, Foreign: h.foreign})
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

// worktreeAt is the name of the worktree of this repository at path, the path itself when none is there.
func (f Facts) worktreeAt(path string) string {
	if w, ours := f.atPath(path); ours {
		return w.Name()
	}
	return path
}

// atPath is the worktree of this repository at path, and whether there is one.
func (f Facts) atPath(path string) (worktree.Worktree, bool) {
	at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Path == path })
	if at < 0 {
		return worktree.Worktree{}, false
	}
	return f.Worktrees[at], true
}

// ----------------------------------------------------------------------------
// Runners

// A conflict is a job a start would run twice: started by name, and by a Runner the same start runs, which starts it itself.
type conflict struct {
	Job    string
	Runner string
}

// conflicts are the jobs that would run twice once the start is done, in one of targets: a job of the start, or up there, that a runner of the start, or up there, starts too, a pair at least one of which the start starts; a runner and its child both up already, which the start leaves alone, are not this start's.
func (f Facts) conflicts(targets []Target, jobs []project.Job) []conflict {
	starts := map[string]bool{}
	for _, job := range jobs {
		starts[job.Name] = true
	}

	var conflicts []conflict
	for _, t := range targets {
		running := map[string]bool{}
		for name := range starts {
			running[name] = true
		}
		for _, name := range run.UpIn(f.Jobs, t.Path) {
			running[name] = true
		}
		for _, runner := range f.Run.Jobs {
			if !running[runner.Name] {
				continue
			}
			for _, child := range f.children(runner) {
				pair := conflict{Job: child.Name, Runner: runner.Name}
				if running[child.Name] && (starts[runner.Name] || starts[child.Name]) && !slices.Contains(conflicts, pair) {
					conflicts = append(conflicts, pair)
				}
			}
		}
	}
	return conflicts
}

// ----------------------------------------------------------------------------
// The refusals

// The codes of the refusals of a start, after the command's name: a job that changes foreign data (Unsafe, a ForeignData problem per job, followed by Force, the request forced), a port another worktree holds (PortClash, followed by ExclusiveStart, the request stopping the others first), a job a runner of the same start starts too (RunnerConflict), a run daemon of another build holding jobs (domain.DaemonSkew, followed by RestartDaemon).
const (
	Unsafe         = ".unsafe"
	ForeignData    = ".foreign_data"
	Force          = ".force"
	PortClash      = ".port_clash"
	ExclusiveStart = ".exclusive"
	RunnerConflict = ".runner_conflict"
	RestartDaemon  = ".restart_daemon"
)

// RefusalParams are what a start is checked against besides the facts: the command asking, whose codes the refusals give; the follow-up that restarts a run daemon of another build; the worktrees as planned and the jobs the start runs in each, in order; whether the other worktrees' jobs stop first and whether a job that changes foreign data starts all the same; the field a runner conflict is a problem of; the answers of the request stopping the others first, offered when that frees every port held (nil: offered by no refusal); and those of the request forced.
type RefusalParams struct {
	Command   string
	Restart   *domain.FollowUp
	Targets   []Target
	Jobs      []project.Job
	Exclusive bool
	Force     bool
	JobsPath  string
	Stopping  []domain.Answered
	Forced    []domain.Answered
}

// Refusal is the refusal that stops a start before anything starts, nil when none does: a run daemon of another build holding jobs, a job a runner of the start starts too, a port another worktree holds unless its jobs are stopped first, a job that changes data a worktree does not own unless forced.
func (f Facts) Refusal(params RefusalParams) error {
	if f.Skewed != nil && slices.ContainsFunc(f.Jobs, func(job run.Job) bool { return job.Up }) {
		return domain.SkewRefusal(domain.SkewRefusalParams{Command: params.Command, Undone: "nothing started", Err: f.Skewed, Restart: params.Restart})
	}
	if conflicts := f.conflicts(params.Targets, params.Jobs); len(conflicts) > 0 {
		return conflictRefusal(params, conflicts)
	}
	if clashes := f.clashes(params.Targets); len(clashes) > 0 && !(params.Exclusive && !slices.ContainsFunc(clashes, func(c clash) bool { return !c.lifts() })) {
		return clashRefusal(params, clashes)
	}
	if risks := f.risks(params.Targets, params.Jobs); len(risks) > 0 && !params.Force {
		return foreignRefusal(params, risks)
	}
	return nil
}

func conflictRefusal(params RefusalParams, conflicts []conflict) *domain.Error {
	fields := make([]domain.FieldError, 0, len(conflicts))
	for _, c := range conflicts {
		fields = append(fields, domain.FieldError{Path: params.JobsPath, Code: params.Command + RunnerConflict, Template: "{job} would run twice: {runner} starts it too", Params: domain.Params{"job": c.Job, "runner": c.Runner}})
	}
	return domain.Invalid(fields)
}

// clashRefusal names each port held; it offers the start stopping the others first when that frees every one and the command offers it (a start in one worktree only, as exclusive refuses several).
func clashRefusal(params RefusalParams, clashes []clash) *domain.Error {
	var lines []string
	fields := make([]domain.FieldError, 0, len(clashes))
	lifted := params.Stopping != nil
	for _, c := range clashes {
		port := strconv.Itoa(c.Port)
		template, holder := "{worktree}'s {job} would bind {port}, which {held_by}'s {held_job} holds", c.HeldBy+"'s "+c.HeldJob
		if c.Foreign {
			template, holder = "{worktree}'s {job} would bind {port}, which {held_job}, a job outside this repository's worktrees, holds in {held_by}", c.HeldJob+", a job outside this repository's worktrees, in "+c.HeldBy
		}
		lines = append(lines, port+" ("+holder+")")
		fields = append(fields, domain.FieldError{Path: domain.PathWorktree, Code: params.Command + PortClash, Template: template, Params: domain.Params{"worktree": c.Worktree, "job": c.Job, "port": port, "held_by": c.HeldBy, "held_job": c.HeldJob}})
		lifted = lifted && c.lifts()
	}
	refusal := &domain.Error{Kind: domain.KindRefused, Code: params.Command + PortClash, Message: "nothing started: ports another worktree holds: " + strings.Join(lines, ", "), Fields: fields}
	if lifted {
		refusal.FollowUp = &domain.FollowUp{Command: params.Command, Answers: params.Stopping, Code: params.Command + ExclusiveStart, Message: "stop the other worktrees' jobs first"}
	}
	return refusal
}

// ownerWords say whose data an owner's is, after "changes".
var ownerWords = map[owner]string{ownerSource: "its source's data: it is verbatim", ownerEveryone: "every worktree's data: it has no namespace"}

func foreignRefusal(params RefusalParams, risks []risk) *domain.Error {
	fields := make([]domain.FieldError, 0, len(risks))
	var worktrees []string
	for _, r := range risks {
		job := r.Job
		if r.Via != "" {
			job += " (run by " + r.Via + ")"
		}
		fields = append(fields, domain.FieldError{Path: entryPath(r.entry), Code: params.Command + ForeignData, Template: "{value}: {job} changes {service}, " + ownerWords[r.Owner], Params: domain.Params{"value": r.Worktree, "job": job, "service": r.Service, "owner": string(r.Owner)}})
		if !slices.Contains(worktrees, r.Worktree) {
			worktrees = append(worktrees, r.Worktree)
		}
	}
	return &domain.Error{
		Kind:     domain.KindRefused,
		Code:     params.Command + Unsafe,
		Message:  "nothing started: a job changes data " + strings.Join(worktrees, ", ") + " does not own",
		Params:   domain.Params{"worktrees": strings.Join(worktrees, ", ")},
		Fields:   fields,
		FollowUp: &domain.FollowUp{Command: params.Command, Answers: params.Forced, Code: params.Command + Force, Message: "start them anyway"},
	}
}
