package runview

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
)

// key is a job of the view: its worktree and its name, since two worktrees
// running one profile hold two jobs called web.
type key struct {
	dir  string
	name string
}

// An entry is one job of the list: the worktree it is listed under, the job
// as the daemon lists it (listed false for a job it declares the daemon does
// not know: not started), and whether it is a shared service, listed once in
// the shared group whichever worktree holds it.
type entry struct {
	key
	branch string
	job    run.Job
	listed bool
	shared bool
}

// A row is one line of the list: a heading, a blank line between two groups,
// or a job.
type row struct {
	heading string
	shared  bool
	spacer  bool
	entry   *entry
}

// entries are the jobs the view lists, in the list's order: the shared
// services up, one entry each (the main checkout's own job when the view
// covers it, else the first worktree's claim), then each worktree's jobs, those
// it declares first in their order, then the others the daemon lists, those
// matching filter only.
func entries(params Params, jobs []run.Job, filter string) []entry {
	covered := map[string]string{}
	for _, wt := range params.Worktrees {
		covered[wt.Path] = branchOf(wt)
	}

	var shared []entry
	sharedAt := map[key]int{}
	own := map[key]run.Job{}
	for _, job := range jobs {
		branch, in := covered[job.WorkDir]
		if !in {
			continue
		}
		if !sharedUp(job) {
			own[key{dir: job.WorkDir, name: job.Name}] = job
			continue
		}
		service := key{dir: job.SharedDir, name: job.Name}
		e := entry{key: key{dir: job.WorkDir, name: job.Name}, branch: branch, job: job, listed: true, shared: true}
		at, seen := sharedAt[service]
		switch {
		case !seen:
			sharedAt[service] = len(shared)
			shared = append(shared, e)
		case job.Hold != run.HoldClaim:
			shared[at] = e
		}
	}

	var list []entry
	for _, e := range shared {
		if matches(e.name, filter) {
			list = append(list, e)
		}
	}
	for _, wt := range params.Worktrees {
		for _, e := range worktreeEntries(wt, jobs, own, sharedAt) {
			if matches(e.name, filter) {
				list = append(list, e)
			}
		}
	}
	return list
}

// worktreeEntries are wt's own jobs: those it declares, in order, then the
// others the daemon lists in it; a shared service up is the shared group's.
func worktreeEntries(wt Worktree, jobs []run.Job, own map[key]run.Job, sharedAt map[key]int) []entry {
	var list []entry
	seen := map[string]bool{}
	for _, name := range wt.Jobs {
		if seen[name] || sharesUp(sharedAt, name) {
			continue
		}
		seen[name] = true
		job, listed := own[key{dir: wt.Path, name: name}]
		list = append(list, entry{key: key{dir: wt.Path, name: name}, branch: branchOf(wt), job: job, listed: listed})
	}
	for _, job := range jobs {
		if job.WorkDir != wt.Path || seen[job.Name] || sharedUp(job) {
			continue
		}
		seen[job.Name] = true
		list = append(list, entry{key: key{dir: wt.Path, name: job.Name}, branch: branchOf(wt), job: job, listed: true})
	}
	return list
}

// sharesUp reports whether a shared service called name is up in the shared group.
func sharesUp(sharedAt map[key]int, name string) bool {
	for service := range sharedAt {
		if service.name == name {
			return true
		}
	}
	return false
}

// sharedUp reports whether job is a shared service up, or a worktree's hold on one.
func sharedUp(job run.Job) bool {
	return job.Up && (job.Hold == run.HoldClaim || job.SharedDir != "")
}

func branchOf(wt Worktree) string {
	if wt.Branch != "" {
		return wt.Branch
	}
	return wt.Path
}

func matches(name, filter string) bool {
	return strings.Contains(strings.ToLower(name), strings.ToLower(strings.TrimSpace(filter)))
}

// rows lays the list out: the shared group under its heading, then each
// worktree's group under its branch, a blank line between two groups.
func rows(list []entry) []row {
	var out []row
	current := ""
	for i := range list {
		e := &list[i]
		group := e.dir
		if e.shared {
			group = "\x00shared"
		}
		if group != current {
			if current != "" {
				out = append(out, row{spacer: true})
			}
			current = group
			heading := e.branch
			if e.shared {
				heading = sharedHeading
			}
			out = append(out, row{heading: heading, shared: e.shared})
		}
		out = append(out, row{entry: e})
	}
	return out
}

// ----------------------------------------------------------------------------
// A job's mark and state

// mark is the glyph of a job's state in the list.
type mark int

const (
	markStopped mark = iota
	markStarting
	markUp
	markDone
	markFailed
)

func markOf(e entry) mark {
	job := e.job
	switch {
	case !e.listed:
		return markStopped
	case e.shared || (job.Hold == run.HoldDetached && job.Up) || job.State == run.Running:
		return markUp
	case job.State == run.Starting:
		return markStarting
	case job.State == run.Crashed || job.Reaped || failedExit(job):
		return markFailed
	case job.State == run.Exited:
		return markDone
	}
	return markStopped
}

func failedExit(job run.Job) bool {
	return job.State == run.Exited && job.ExitCode != nil && *job.ExitCode != 0
}

// state words what a job is doing, as the pane's title says it: from the
// daemon's listing, never from what the start said of it.
func (m *model) state(e entry) string {
	job := e.job
	switch {
	case !e.listed:
		return stateNotStarted
	case e.shared && job.Hold == run.HoldClaim:
		return fmt.Sprintf(stateJoinedFmt, m.mainName(job.SharedDir))
	case job.State == run.Starting:
		return string(run.Starting)
	case job.Hold == run.HoldDetached && job.Up:
		return stateDetached
	case job.State == run.Crashed:
		return withExit(string(run.Crashed), job.ExitCode)
	case failedExit(job):
		return withExit(stateFailed, job.ExitCode)
	case job.State == run.Exited:
		return stateDone
	case job.Reaped:
		return stateReaped
	case job.State == "":
		return string(run.Running)
	}
	return string(job.State)
}

func withExit(word string, code *int) string {
	if code == nil {
		return word
	}
	return fmt.Sprintf(exitFmt, word, *code)
}

// mainName names the main checkout at dir: its branch, else its directory.
func (m *model) mainName(dir string) string {
	if m.params.Main != "" {
		return m.params.Main
	}
	return filepath.Base(dir)
}

// uptime is how long a job up has run, empty for one that is not.
func uptime(job run.Job, now time.Time) string {
	if !job.Up || job.StartedAt.IsZero() {
		return ""
	}
	elapsed := max(now.Sub(job.StartedAt), 0)
	switch {
	case elapsed < time.Minute:
		return fmt.Sprintf("%ds", int(elapsed.Seconds()))
	case elapsed < time.Hour:
		return fmt.Sprintf("%dm", int(elapsed.Minutes()))
	case elapsed < 24*time.Hour:
		return fmt.Sprintf("%dh%02dm", int(elapsed.Hours()), int(elapsed.Minutes())%60)
	}
	return fmt.Sprintf("%dd%02dh", int(elapsed.Hours())/24, int(elapsed.Hours())%24)
}

// ----------------------------------------------------------------------------
// What the pane can do with a job

// attachable reports whether the job prints into a stream the view can follow:
// a process the daemon runs, or the shared service a claim holds; a stack its
// launcher left has none, and a job that ended left only its log.
func attachable(e entry) bool {
	return e.listed && e.job.Up && e.job.Hold != run.HoldDetached && (e.job.State == run.Running || e.job.Hold == run.HoldClaim)
}

// onPipe reports whether a job prints through a pipe rather than a terminal,
// whose bare line feeds the emulator must be given carriage returns.
func onPipe(e entry) bool {
	return e.job.Kind == project.Task
}

// refusal is why the keyboard cannot be handed to a job attached, empty when
// it can: a task reads no terminal, and a shared service another worktree's
// hold reaches is read-only.
func (m *model) refusal(e entry) string {
	if e.job.Kind == project.Task {
		return fmt.Sprintf(noTerminalFmt, e.name)
	}
	if e.shared && e.job.Hold == run.HoldClaim {
		return fmt.Sprintf(sharedReadOnlyFmt, e.name, m.mainName(e.job.SharedDir))
	}
	return ""
}
