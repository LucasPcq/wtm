package runjobs

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

// Facts are what a start decides from, as Observe read them: the project's config, its run.toml, the repository's snapshot, the meta record of each worktree on a branch and why one that exists does not read, the worktrees whose directory is gone, where the caller stands, the jobs the run daemon runs (with why it could not tell, or that one of another build answered), and the drops removed worktrees owe the shared services.
type Facts struct {
	worktree.Repository
	Records    map[string]worktree.Meta // by branch
	RecordErrs map[string]error         // by branch
	Gone       map[string]bool          // by path
	Cwd        string                   // where the caller stands, through its symbolic links
	Jobs       []run.Job
	JobsErr    error
	Skewed     *run.Skewed
	Owed       []env.OwedDrop
}

// Standing is the worktree the caller stands in, by its name; none when it stands in none.
func (f Facts) Standing() string {
	worktrees := f.standingWorktrees()
	if at := worktree.Holding(worktrees, f.Cwd, func(w worktree.Worktree) string { return w.Path }); at >= 0 {
		return worktrees[at].Name()
	}
	return ""
}

func (f Facts) standingWorktrees() []worktree.Worktree {
	return slices.DeleteFunc(f.MainFirst(), func(w worktree.Worktree) bool { return f.Gone[w.Path] })
}

// WorktreeOptions are the worktrees a start may run in: each one whose directory stands, the main one first, the caller's tagged current, each with what it runs (domain.RunningIn).
func (f Facts) WorktreeOptions() []domain.Option {
	worktrees := f.standingWorktrees()
	return domain.RunningIn(domain.WorktreeOptions(worktrees, domain.WorktreeQuery{Cwd: f.Cwd}), worktrees, f.Jobs)
}

// Declared is the job of run.toml named name, and whether it declares one.
func (f Facts) Declared(name string) (project.Job, bool) {
	at := slices.IndexFunc(f.Run.Jobs, func(job project.Job) bool { return job.Name == name })
	if at < 0 {
		return project.Job{}, false
	}
	return f.Run.Jobs[at], true
}

// Shared reports whether the job of name runs once for every worktree.
func (f Facts) Shared(name string) bool {
	job, ok := f.Declared(name)
	return ok && job.Scope == project.Shared
}

// OrdinalOf is the ordinal t's record holds, 0 when it holds none.
func (f Facts) OrdinalOf(t Target) int {
	return f.Records[t.Branch].Ordinal
}

// up are the jobs up in the worktree at path, claims included.
func (f Facts) up(path string) []run.Job {
	return slices.DeleteFunc(slices.Clone(f.Jobs), func(job run.Job) bool { return !job.Up || job.WorkDir != path })
}

func (f Facts) named(name string) (worktree.Worktree, bool) {
	at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Name() == name })
	if at < 0 {
		return worktree.Worktree{}, false
	}
	return f.Worktrees[at], true
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

// children are the declared jobs a runner starts itself, its runners' own included, each once.
func (f Facts) children(runner project.Job) []project.Job {
	seen := map[string]bool{runner.Name: true}
	queue := slices.Clone(runner.Runs)
	var children []project.Job
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		child, ok := f.Declared(name)
		if seen[name] || !ok {
			continue
		}
		seen[name] = true
		children = append(children, child)
		queue = append(queue, child.Runs...)
	}
	return children
}

// jobsIn are the declared jobs t starts, in its order.
func (f Facts) jobsIn(t Target) []project.Job {
	jobs := make([]project.Job, 0, len(t.Jobs))
	for _, planned := range t.Jobs {
		if declared, ok := f.Declared(planned.Name); ok {
			jobs = append(jobs, declared)
		}
	}
	return jobs
}

// jobParams is what the job environment of t reads, its record's ordinal and isolation; the main worktree's ordinal is 0.
func (f Facts) jobParams(t Target, ordinal int) env.JobParams {
	record := f.Records[t.Branch]
	return env.JobParams{Branch: t.Branch, ProjectDir: f.MainPath, Ordinal: ordinal, Isolation: record.Isolation.Effective(), Run: f.Run}
}

// mainTarget is the main worktree, as a start in it would be planned.
func (f Facts) mainTarget() Target {
	for _, w := range f.Worktrees {
		if w.Main {
			return Target{Worktree: w.Name(), Branch: w.Branch(), Path: w.Path, Main: true, Numbered: true}
		}
	}
	return Target{Path: f.MainPath, Main: true, Numbered: true}
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

// ----------------------------------------------------------------------------
// The worktrees named

// WorktreeGone is the code, after the command's name, of a worktree named whose directory is gone; one whose record does not read is domain.MetaUnreadable's.
const WorktreeGone = ".worktree_gone"

// Entries decides each worktree names asks a start in, worded under command: a Target, or the problem of its entry (at worktree[index]): blank, given twice, gone, a record that does not read, or naming no worktree.
func (f Facts) Entries(command string, names []string) (targets []Target, problems []domain.FieldError) {
	for index, name := range names {
		path := entryPath(index)
		w, found := f.named(name)
		switch {
		case strings.TrimSpace(name) == "":
			problems = append(problems, domain.Required(path))
		case slices.Contains(names[:index], name):
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeDistinct, Template: "{path}: {value} is given twice", Params: domain.Params{"value": name}})
		case found && f.Gone[w.Path]:
			problems = append(problems, domain.FieldError{Path: path, Code: command + WorktreeGone, Template: "{value}'s directory is gone: nothing to start in", Params: domain.Params{"value": name, "path": w.Path}})
		case found && f.RecordErrs[w.Branch()] != nil:
			problems = append(problems, domain.FieldError{Path: path, Code: command + domain.MetaUnreadable, Template: "{value}'s record does not read: its ports are unknown", Params: domain.Params{"value": name, "cause": f.RecordErrs[w.Branch()].Error()}})
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

func entryPath(index int) string {
	return domain.PathWorktree + "[" + strconv.Itoa(index) + "]"
}

// ----------------------------------------------------------------------------
// The other worktrees' jobs

// The paths of the fields of whether the other worktrees' jobs stop first (exclusive) or keep running (parallel), which the commands that start jobs share.
const (
	PathExclusive = "exclusive"
	PathParallel  = "parallel"
)

// The codes of the problems of the question, after the command's name: exclusive beside several worktrees, or beside parallel.
const (
	ExclusiveSeveral     = ".exclusive_several"
	ExclusiveAndParallel = ".exclusive_and_parallel"
)

// ExclusiveParams are what the question of whether the other worktrees' jobs stop first settles from: the command asking, its request's exclusive (nil: not given) and parallel, and the worktrees it starts in.
type ExclusiveParams struct {
	Command   string
	Exclusive *bool
	Parallel  bool
	Targets   []Target
}

// An Exclusion is the question of whether the other worktrees' jobs are stopped first, and its problems.
type Exclusion struct {
	domain.Confirm
	Problems []domain.FieldError
}

// Exclusive settles whether the other worktrees' jobs are stopped first: exclusive or parallel, else run.toml's concurrency, else asked when another worktree runs jobs, keeping them by default. run.toml's exclusive cannot stop all but one of several worktrees: it is set aside for them, with a warning the command's plan gives.
func (f Facts) Exclusive(params ExclusiveParams) Exclusion {
	switch {
	case params.Parallel && params.Exclusive != nil && *params.Exclusive:
		return Exclusion{Confirm: given(true), Problems: []domain.FieldError{{Path: PathExclusive, Code: params.Command + ExclusiveAndParallel, Template: "{path} stops the others' jobs, which --parallel keeps", With: []string{PathParallel}}}}
	case params.Parallel:
		return Exclusion{Confirm: given(false)}
	case params.Exclusive != nil && *params.Exclusive && len(params.Targets) > 1:
		return Exclusion{Confirm: given(true), Problems: []domain.FieldError{{Path: PathExclusive, Code: params.Command + ExclusiveSeveral, Template: "{path} stops all but one worktree: it cannot start several"}}}
	case params.Exclusive != nil:
		return Exclusion{Confirm: given(*params.Exclusive)}
	}

	question := domain.Confirm{Answer: domain.Answer[bool]{Origin: domain.OriginDefault}}
	switch {
	case len(f.Others(params.Targets)) == 0:
		question.Skip = domain.SkipAlone
	case f.Run.Concurrency != "":
		question.Value, question.Origin, question.Skip = f.Run.Concurrency == project.Exclusive && len(params.Targets) <= 1, domain.OriginConfig, domain.SkipConfigured
	}
	return Exclusion{Confirm: question}
}

func given(yes bool) domain.Confirm {
	return domain.Confirm{Answer: domain.Answer[bool]{Value: yes, Origin: domain.OriginRequest}}
}
