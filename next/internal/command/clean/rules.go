package clean

import (
	"slices"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what clean decides from, as Observe read them: what a removal decides from (the project's config, its run.toml, the repository's snapshot, the records, what each worktree asked holds that makes it unsafe to remove, where the caller stands, why the run daemon or the forge could not be asked), and where each name asked as a path leads.
type Facts struct {
	teardown.Facts
	Located map[string]string // the real path each name asked that is no worktree's name leads to, when something is there
}

// named returns the worktree of name, its branch or, on no branch, its path as git spells it, and whether there is one. Unlike worktree.Named, a part of a branch names none: run again once feat/login went, `clean feat/login` would otherwise remove feat/login-ui.
func (f Facts) named(name string) (worktree.Worktree, bool) {
	at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Name() == name })
	if at < 0 {
		return worktree.Worktree{}, false
	}

	return f.Worktrees[at], true
}

// subject is what name asks clean to remove, the first that holds: the worktree it names; a local branch of that name no worktree holds, as a worktree with no path; the worktree whose root the path name stands for leads to. A path inside a worktree names none: `clean docs` from a worktree's root would otherwise remove it.
func (f Facts) subject(name string) (worktree.Worktree, bool) {
	if w, ok := f.named(name); ok {
		return w, true
	}
	if f.Local(name) && f.CheckedOutAt(name) == "" {
		return worktree.Worktree{State: worktree.OnBranch{Branch: name}}, true
	}

	located, ok := f.Located[name]
	at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return ok && w.Path == located })
	if at < 0 {
		return worktree.Worktree{}, false
	}
	return f.Worktrees[at], true
}

// asked are the worktrees, and the branches no worktree holds, that names asks clean to remove, each once: the checks Observe reads.
func (f Facts) asked(names []string) []worktree.Worktree {
	var found []worktree.Worktree
	for _, name := range names {
		w, ok := f.subject(name)
		if ok && !w.Main && !slices.Contains(found, w) {
			found = append(found, w)
		}
	}

	return found
}

// ----------------------------------------------------------------------------
// The decision of each worktree

// entries decides each worktree names asks: a removal, a name that is no worktree (already absent, nothing to remove), or the problem of its entry: blank, given twice, the main worktree or the base branch.
func (f Facts) entries(names []string) (removals []teardown.Removal, absent []string, problems []domain.FieldError) {
	for index, name := range names {
		path := teardown.EntryPath(index)
		switch w, found := f.subject(name); {
		case strings.TrimSpace(name) == "":
			problems = append(problems, domain.Required(path))
		case slices.Contains(names[:index], name):
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeDistinct, Template: "{path}: {value} is given twice", Params: domain.Params{"value": name}})
		case found && w.Main:
			problems = append(problems, domain.FieldError{Path: path, Code: CodeMainWorktree, Template: "{value} is the main worktree: it is never removed", Params: domain.Params{"value": name}})
		case found && w.Branch() == f.Config.Worktrees.BaseBranch:
			problems = append(problems, domain.FieldError{Path: path, Code: CodeBaseBranch, Template: "{value} is the base branch: it is never removed", Params: domain.Params{"value": name}})
		case found && slices.ContainsFunc(removals, func(r teardown.Removal) bool { return r.Worktree == w.Name() || w.Path != "" && r.Path == w.Path }):
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeDistinct, Template: "{path}: {value} names a worktree given already", Params: domain.Params{"value": name}})
		case found:
			removals = append(removals, f.Removal(w, index))
		default:
			absent = append(absent, name)
		}
	}

	return removals, absent, problems
}

// CodeMainWorktree refuses the main worktree and CodeBaseBranch the base branch, which clean never removes.
const (
	CodeMainWorktree = Name + ".main_worktree"
	CodeBaseBranch   = Name + ".base_branch"
)

// ----------------------------------------------------------------------------
// Decide

// Decide turns the facts and the resolved request into the plan and its warnings. A worktree a blocker makes unsafe refuses the whole request, before anything is removed, with CodeUnsafe, unless the request forces it.
func Decide(f Facts, req Request) (Plan, []domain.Warning, error) {
	removals, absent, _ := f.entries(req.Worktrees)
	if refused := f.NewerRefusal(Name, removals); refused != nil {
		return Plan{}, nil, refused
	}
	if blocked := teardown.Blocked(removals); len(blocked) > 0 && !req.Force {
		forced := Request{Worktrees: req.Worktrees, ReparentChildren: req.ReparentChildren, Force: true}
		return Plan{}, nil, teardown.Refusal(teardown.RefusalParams{Command: Name, Blocked: blocked, Forced: domain.AnswersOf(Questions{}.Form(), forced)})
	}

	planned := Plan{Worktrees: removals, Absent: absent, Moves: f.Moves(removals), ReparentChildren: req.ReparentChildren, Force: req.Force, facts: f, standing: f.Standing(removals)}
	if planned.Worktrees == nil {
		planned.Worktrees = []teardown.Removal{}
	}

	return planned, f.warnings(planned), nil
}

// CodeSchemaNewer refuses a request that would remove a worktree whose record a newer wtm wrote, forced or not.
const CodeSchemaNewer = Name + teardown.SchemaNewer

// CodeUnsafe refuses a request that would remove a worktree a blocker makes unsafe; CodeForce is its follow-up, the same request forced, and CodeStopJobs the one that stops a worktree's jobs.
const (
	CodeUnsafe   = Name + teardown.Unsafe
	CodeForce    = Name + teardown.Force
	CodeStopJobs = Name + teardown.StopJobs
)

// CodeJobsLeft warns of the jobs a forced removal leaves running, CodeDataKept of the namespaces a removed worktree keeps in shared services.
const (
	CodeJobsLeft = Name + teardown.JobsLeft
	CodeDataKept = Name + teardown.DataKept
)

// warnings are what the plan cannot see to or leaves behind (teardown.Facts.Warnings); a forge that could not be asked leaves open pull requests unchecked and merged ones not counted as pushed.
func (f Facts) warnings(planned Plan) []domain.Warning {
	return f.Warnings(teardown.WarningsParams{Command: Name, Removals: planned.Worktrees, Unasked: "open ones aren't checked, merged ones don't count as pushed"})
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: the worktrees it removes, the names that are no worktree, the children left behind, whether they are reparented, and whether the unsafe worktrees are forced; then the recap, which the fingerprint leaves out.
type Plan struct {
	Worktrees        []teardown.Removal `json:"worktrees"`
	Absent           []string           `json:"absent,omitempty"`
	Moves            []teardown.Move    `json:"moves,omitempty"`
	ReparentChildren bool               `json:"reparent_children"`
	Force            bool               `json:"force"`
	domain.Recapped
	facts    Facts
	standing string // the worktree removed the caller stands in, empty when none
}

// Locks are the locks the plan needs: the repository shared, and each worktree it removes and each child it reparents exclusive.
func (p Plan) Locks() []domain.Lock {
	names := make([]string, 0, len(p.Worktrees)+len(p.Moves))
	for _, r := range p.Worktrees {
		names = append(names, r.Worktree)
	}
	if p.ReparentChildren {
		for _, move := range p.Moves {
			names = append(names, move.Branch)
		}
	}

	return domain.WorktreeLocks(names)
}

// Fingerprinted is what the plan's fingerprint covers: the plan, the on_clean hooks of each removal among it, and what their environment reads of each removal's record (its ordinal and isolation, its ports and Compose project); its recap left out.
func (p Plan) Fingerprinted() any {
	p.Recapped = domain.Recapped{}
	type hookEnv struct {
		Ordinal   int               `json:"ordinal"`
		Isolation project.Isolation `json:"isolation"`
	}
	envs := map[string]hookEnv{}
	for _, r := range p.Worktrees {
		record := p.facts.Records[r.Branch]
		envs[r.Worktree] = hookEnv{Ordinal: record.Ordinal, Isolation: record.Isolation.Effective()}
	}

	return struct {
		Plan
		HookEnvs map[string]hookEnv `json:"hook_envs"`
	}{p, envs}
}

// Locks are the locks taken before planning: the repository shared, and each worktree asked exclusive.
func (r Request) Locks() []domain.Lock {
	return domain.WorktreeLocks(slices.DeleteFunc(slices.Clone(r.Worktrees), func(name string) bool { return strings.TrimSpace(name) == "" }))
}

// ----------------------------------------------------------------------------
// The recap

// NoteAbsent notes a recap's value that is no worktree, nothing to remove; the others are teardown's.
const NoteAbsent = "absent"

// recap is the plan row by row: the worktrees removed, if any, each forced one with what it loses, the names that are no worktree, then the children left behind, reparented or orphaned.
func (p Plan) recap() []domain.RecapRow {
	var rows []domain.RecapRow
	if len(p.Worktrees) > 0 {
		rows = append(rows, domain.RecapRow{Key: "remove", Label: "Remove", Path: domain.PathWorktree, Values: teardown.Recapped(p.Worktrees)})
	}
	if len(p.Absent) > 0 {
		values := make([]domain.RecapValue, 0, len(p.Absent))
		for _, name := range p.Absent {
			values = append(values, domain.RecapValue{Value: name, Note: NoteAbsent, Text: name + " (no worktree: nothing to remove)"})
		}
		rows = append(rows, domain.RecapRow{Key: "absent", Label: "Already absent", Path: domain.PathWorktree, Values: values})
	}

	if len(p.Moves) > 0 {
		rows = append(rows, domain.RecapRow{Key: "children", Label: "Children", Path: PathReparentChildren, Values: teardown.RecappedMoves(p.Moves, p.ReparentChildren)})
	}

	return rows
}

// ----------------------------------------------------------------------------
// The outcome

// CodeGo is the step out of a worktree removed under the caller's feet, into the main one; CodeOrphaned warns of a child left on its removed parent, and CodeReparent is the step that moves it.
const (
	CodeGo       = Name + teardown.Go
	CodeOrphaned = Name + teardown.Orphaned
	CodeReparent = Name + teardown.Reparent
)

// next is the step into the main worktree when the caller stood in a worktree removed.
func (p Plan) next(items []Item) []domain.FollowUp {
	if !teardown.RemovedIn(items, p.standing) {
		return nil
	}

	return p.facts.Next(Name, p.standing)
}
