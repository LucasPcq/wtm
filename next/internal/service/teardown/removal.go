// Package teardown removes worktrees for the commands that do, clean and prune: what makes one unsafe to remove and the refusal it makes, the children a removal leaves behind, the data it made in shared services, the recap of all three, then each worktree removed (its jobs stopped and its on_clean hooks run first, its data dropped or owed after) and its hold on the shared services let go of; and the drops owed paid once their service is up. Every code it gives is the asking command's: <command>.<code>.
package teardown

import (
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

// Facts are what a removal decides from: the repository, the meta record of each worktree on a branch, the records a newer wtm wrote, what each worktree checked holds that makes it unsafe to remove, the job environment of each, where the caller stands, the jobs the run daemon lists, and why the run daemon or the forge could not be asked.
type Facts struct {
	worktree.Repository
	Records map[string]worktree.Meta       // by branch
	Newer   map[string]*domain.SchemaNewer // by branch, the records this wtm can neither read nor delete
	Checks  map[string]Check               // by the name a worktree is checked by
	Envs    map[string]map[string]string   // by the name a worktree is checked by, its job environment (Service.Envs)
	Cwd     string                         // where the caller stands, through its symbolic links
	Jobs    []run.Job                      // the jobs the run daemon lists
	JobsErr error                          // why the run daemon could not tell which jobs are up
	Skew    *domain.Warning                // the warning of a run daemon of another build, worded by the command
	PRsErr  error                          // why the forge could not be asked for open pull requests
}

// A Check is whether a worktree's directory is gone, and what it holds that its removal would lose or stop: uncommitted changes, commits no other branch holds, an open pull request, the jobs it runs for itself.
type Check struct {
	Missing     bool
	Dirty       bool
	Unpushed    int
	PullRequest string   // the page of its open pull request, empty when it has none
	Jobs        []string // the jobs up it runs for itself, sorted: its claims on shared services are no job of its own
}

// Settle completes the facts with what the run daemon and the forge said: the jobs it lists, those each worktree runs for itself, its open pull request, and no commit unpushed when its merged pull request holds them all (landed), which a forge that could not be asked cannot tell.
func (f *Facts) Settle(worktrees []worktree.Worktree, jobs []run.Job, prs []worktree.PullRequest) {
	f.Jobs = jobs
	for _, w := range worktrees {
		check := f.Checks[w.Name()]
		check.Jobs = run.OwnUpIn(jobs, w.Path)
		check.PullRequest = openPullRequest(prs, w.Branch())
		if f.PRsErr == nil && f.landed(prs, w) {
			check.Unpushed = 0
		}
		f.Checks[w.Name()] = check
	}
}

// landed reports whether every commit of w is its merged pull request's: the head GitHub keeps is w's branch tip and the commit w stands on, so a branch squash- or rebase-merged, then deleted on origin, loses nothing; a commit made since, or a pull request still open, lands nothing.
func (f Facts) landed(prs []worktree.PullRequest, w worktree.Worktree) bool {
	pr, ok := worktree.PullRequestOf(prs, w.Branch())
	return ok && f.Cleans(pr, w, false) && (w.Head == "" || w.Head == pr.Head)
}

// openPullRequest is the page of the open pull request of branch among prs, empty when it has none.
func openPullRequest(prs []worktree.PullRequest, branch string) string {
	at := slices.IndexFunc(prs, func(pr worktree.PullRequest) bool {
		return branch != "" && pr.Branch == branch && pr.State == worktree.PullRequestOpen
	})
	if at < 0 {
		return ""
	}

	return prs[at].URL
}

// Branches are the branches of worktrees, each asked of the forge: one origin no longer has may be a pull request merged and its branch deleted.
func Branches(worktrees []worktree.Worktree) []string {
	var found []string
	for _, w := range worktrees {
		if w.Branch() != "" {
			found = append(found, w.Branch())
		}
	}

	return found
}

// ----------------------------------------------------------------------------
// The removal of each worktree

// A Removal is one worktree a plan removes: its name, its branch, deleted with it (none on no branch), its path (none for a branch no worktree holds, deleted alone), whether its directory is gone already, the on_clean hooks it runs first as they will run (none once its directory is gone), the phases of its removal, the namespaces it made in shared services and their fate, what makes it unsafe to remove with what forcing would lose, and the jobs it runs for itself, stopped first.
type Removal struct {
	Worktree    string             `json:"worktree"`
	Branch      string             `json:"branch,omitempty"`
	Path        string             `json:"path"`
	Missing     bool               `json:"missing,omitempty"`
	Hooks       []string           `json:"hooks,omitempty"`
	Phases      []domain.Phase     `json:"phases"`
	Data        []Namespace        `json:"data,omitempty"`
	Blockers    []worktree.Blocker `json:"blockers"`
	Unpushed    int                `json:"unpushed,omitempty"`
	PullRequest string             `json:"pull_request,omitempty"`
	Jobs        []string           `json:"jobs,omitempty"`
	entry       int                // its index among the worktrees asked, which its problems are on
	stop        bool               // whether its jobs are stopped first: it runs some, or the run daemon could not tell
}

// Removal is the removal of w, asked at index entry of the request's worktrees.
func (f Facts) Removal(w worktree.Worktree, entry int) Removal {
	check := f.Checks[w.Name()]
	removal := Removal{Worktree: w.Name(), Branch: w.Branch(), Path: w.Path, Missing: w.Prunable || check.Missing, Blockers: []worktree.Blocker{}, entry: entry}
	add := func(blocker worktree.Blocker, holds bool) {
		if holds {
			removal.Blockers = append(removal.Blockers, blocker)
		}
	}

	for _, hook := range f.onClean(removal) {
		removal.Hooks = append(removal.Hooks, hook.Cmd)
	}
	if removal.Branch != "" {
		removal.Data = f.data(w)
	}

	add(worktree.BlockerLocked, w.Locked)
	add(worktree.BlockerDirty, check.Dirty)
	add(worktree.BlockerUnpushed, check.Unpushed > 0)
	add(worktree.BlockerOpenPullRequest, check.PullRequest != "")
	removal.Unpushed, removal.PullRequest, removal.Jobs = check.Unpushed, check.PullRequest, check.Jobs
	removal.stop = removal.Path != "" && (len(check.Jobs) > 0 || f.JobsErr != nil)
	removal.Phases = removal.phases()
	return removal
}

// phases are the phases of the removal, in order: its jobs stopped, its on_clean hooks, the removal, its data dropped, each when it has some.
func (r Removal) phases() []domain.Phase {
	var phases []domain.Phase
	if r.stop {
		phases = append(phases, domain.PhaseStopJobs)
	}
	if len(r.Hooks) > 0 {
		phases = append(phases, domain.PhaseOnClean)
	}
	phases = append(phases, domain.PhaseRemove)
	if slices.ContainsFunc(r.Data, func(n Namespace) bool { return n.Status == DataDrop || n.Status == DataStart }) {
		phases = append(phases, domain.PhaseDropData)
	}
	return phases
}

// onClean are the on_clean hooks of removal, their variables expanded as they will run; none once its directory is gone, where they cannot run.
func (f Facts) onClean(removal Removal) []project.Hook {
	if removal.Missing || removal.Path == "" {
		return nil
	}

	vars := env.HookVars{Worktree: removal.Path, Branch: removal.Branch, Root: f.MainPath, FromBranch: f.Records[removal.Branch].SourceBranch}
	var hooks []project.Hook
	for _, hook := range f.Config.OnClean {
		hooks = append(hooks, vars.Hook(hook))
	}
	return hooks
}

// Unsafe reports whether a blocker refuses the removal unless forced.
func (r Removal) Unsafe() bool {
	return len(r.Blockers) > 0
}

// Blocked are the removals a blocker refuses unless forced.
func Blocked(removals []Removal) []Removal {
	return slices.DeleteFunc(slices.Clone(removals), func(r Removal) bool { return !r.Unsafe() })
}

// Standing is the worktree among removals the caller stands in, empty when none.
func (f Facts) Standing(removals []Removal) string {
	if at := worktree.Holding(removals, f.Cwd, func(r Removal) string { return r.Path }); at >= 0 {
		return removals[at].Worktree
	}

	return ""
}

// ----------------------------------------------------------------------------
// The refusal

// The codes a command's refusal of an unsafe removal carries, after its name: the refusal and its follow-up (the same request forced); each blocker is a problem coded <command>.<blocker>.
const (
	Unsafe = ".unsafe"
	Force  = ".force"
)

// NewerRefusal refuses a request that would remove a worktree among removals whose record a newer wtm wrote, forced or not, with <command>.meta_newer caused by each *domain.SchemaNewer: deleting the record is a write this wtm refuses, and what the teardown must undo is that wtm's to know. Nil when no record is newer.
func (f Facts) NewerRefusal(command string, removals []Removal) *domain.Error {
	refusal := domain.MetaNewerParams{Code: command + domain.MetaNewer, Undone: "nothing removed"}
	for _, r := range removals {
		if newer := f.Newer[r.Branch]; newer != nil {
			refusal.Worktrees, refusal.Records = append(refusal.Worktrees, r.Worktree), append(refusal.Records, newer)
		}
	}

	return domain.MetaNewerRefusal(refusal)
}

// RefusalParams are the command refusing, the unsafe removals it refuses, and its request forced, the refusal's follow-up.
type RefusalParams struct {
	Command string
	Blocked []Removal
	Forced  []domain.Answered
}

// Refusal refuses a request with <command>.unsafe: a problem per blocker of each unsafe removal, on its entry, followed by the request forced.
func Refusal(params RefusalParams) *domain.Error {
	var fields []domain.FieldError
	names := make([]string, 0, len(params.Blocked))
	for _, r := range params.Blocked {
		names = append(names, r.Worktree)
		for _, blocker := range r.Blockers {
			fields = append(fields, r.problem(params.Command, blocker))
		}
	}

	return &domain.Error{
		Kind:     domain.KindRefused,
		Code:     params.Command + Unsafe,
		Message:  "nothing removed: " + strings.Join(names, ", ") + " can't be removed safely",
		Params:   domain.Params{"worktrees": strings.Join(names, ", ")},
		Fields:   fields,
		FollowUp: &domain.FollowUp{Command: params.Command, Answers: params.Forced, Code: params.Command + Force, Message: "remove anyway"},
	}
}

// problem is blocker as a problem of the removal's entry, coded <command>.<blocker>.
func (r Removal) problem(command string, blocker worktree.Blocker) domain.FieldError {
	problem := domain.FieldError{Path: EntryPath(r.entry), Code: command + "." + string(blocker), Params: domain.Params{"value": r.Worktree}}
	switch blocker {
	case worktree.BlockerLocked:
		problem.Template = "{value} is locked"
	case worktree.BlockerDirty:
		problem.Template = "{value} has uncommitted changes"
	case worktree.BlockerUnpushed:
		problem.Template, problem.Params["count"] = "{value} has {count} commits no other branch nor remote holds", strconv.Itoa(r.Unpushed)
		if r.Unpushed == 1 {
			problem.Template = "{value} has 1 commit no other branch nor remote holds"
		}
	case worktree.BlockerOpenPullRequest:
		problem.Template, problem.Params["url"] = "{value} has an open pull request: {url}", r.PullRequest
	}

	return problem
}

// EntryPath is the path of the worktree asked at index, as a problem names it.
func EntryPath(index int) string {
	return domain.PathWorktree + "[" + strconv.Itoa(index) + "]"
}

// ----------------------------------------------------------------------------
// The warnings

// WarningsParams are the command planning and what a forge that could not be asked leaves it unable to decide, which it words.
type WarningsParams struct {
	Command string
	Unasked string
}

// Warnings are what a plan cannot see to: the jobs or the pull requests that could not be asked, a run daemon of another build.
func (f Facts) Warnings(params WarningsParams) []domain.Warning {
	var warnings []domain.Warning
	if f.JobsErr != nil {
		warnings = append(warnings, domain.Warning{Code: params.Command + domain.JobsUnknown, Message: run.UnknownMessage(f.JobsErr) + " — a removal whose jobs cannot be seen stopped fails unless forced"})
	}
	if f.Skew != nil {
		warnings = append(warnings, *f.Skew)
	}
	if f.PRsErr != nil {
		warnings = append(warnings, domain.Warning{Code: params.Command + domain.PRsUnavailable, Message: "couldn't ask for pull requests (" + f.PRsErr.Error() + "): " + params.Unasked, Params: worktree.UnavailableParams(f.PRsErr)})
	}

	return warnings
}

// RemovedIn reports whether the worktree of name was removed among items, its branch deleted or not: its item has a detail.
func RemovedIn[D any](items []domain.Unit[D], name string) bool {
	return slices.ContainsFunc(items, func(item domain.Unit[D]) bool { return item.Subject == name && item.Detail != nil })
}

// Went are the removals whose worktree was removed among items.
func Went[D any](removals []Removal, items []domain.Unit[D]) []Removal {
	return slices.DeleteFunc(slices.Clone(removals), func(r Removal) bool { return !RemovedIn(items, r.Worktree) })
}

// ----------------------------------------------------------------------------
// The recap

// The notes of the recap's values: a worktree removed although unsafe, a branch no worktree holds, deleted alone, a child left on its removed parent.
const (
	NoteForced     = "forced"
	NoteBranchOnly = "branch_only"
	NoteOrphaned   = "orphaned"
)

// Recapped are removals as a recap row's values, each with the jobs it stops and, forced, what it loses.
func Recapped(removals []Removal) []domain.RecapValue {
	values := make([]domain.RecapValue, 0, len(removals))
	for _, r := range removals {
		value := domain.RecapValue{Value: r.Worktree, Text: r.Worktree}
		var notes []string
		if r.Path == "" {
			value.Note, notes = NoteBranchOnly, append(notes, "branch only: no worktree")
		}
		if len(r.Jobs) > 0 {
			notes = append(notes, "stops "+strings.Join(r.Jobs, ", "))
		}
		if len(r.Blockers) > 0 {
			lost := strings.Join(domain.Names(r.Blockers), ", ")
			value.Note, value.Params, notes = NoteForced, domain.Params{"blockers": lost}, append(notes, lost+", forced")
		}
		if len(notes) > 0 {
			value.Text += " (" + strings.Join(notes, "; ") + ")"
		}
		values = append(values, value)
	}

	return values
}

// RecappedData are the namespaces removals made in shared services as a recap row's values, each noted with its fate and, when kept, why.
func RecappedData(removals []Removal) []domain.RecapValue {
	var values []domain.RecapValue
	for _, r := range removals {
		for _, namespace := range r.Data {
			value := domain.RecapValue{Value: namespace.Name, Note: string(namespace.Status), Params: domain.Params{"job": namespace.Job, "worktree": r.Worktree}}
			fate := map[DataStatus]string{
				DataDrop:  "dropped once it is removed",
				DataStart: namespace.Job + " started to drop it, then stopped",
				DataDefer: "kept until " + namespace.Job + " is up: it is down",
				DataKeep:  "kept: --keep-data",
			}[namespace.Status]
			if namespace.Reason == ReasonSharedName {
				fate = "kept: another worktree reaches it under the same name"
			}
			if namespace.Reason != "" {
				value.Params["reason"] = string(namespace.Reason)
			}
			value.Text = namespace.Name + " in " + namespace.Job + " (" + fate + ")"
			values = append(values, value)
		}
	}

	return values
}

// ----------------------------------------------------------------------------
// The step out

// Go is the code of the step out of a worktree removed under the caller's feet, after the command's name.
const Go = ".go"

// Next is the step into the main worktree for the caller who stood in standing, a worktree removed; none in a bare repository, which has no main checkout.
func (f Facts) Next(command, standing string) []domain.FollowUp {
	checkouts := f.MainFirst()
	if standing == "" || len(checkouts) == 0 || !checkouts[0].Main {
		return nil
	}

	main := domain.Enter(checkouts[0].Name())
	main.Code, main.Message = command+Go, "you stood in "+standing
	return []domain.FollowUp{main}
}
