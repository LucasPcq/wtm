package clean

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

// Facts are what clean decides from, as Observe read them: the project's config, its run.toml, the repository's snapshot, the meta record of each worktree on a branch, where each name asked as a path leads, where the caller stands, what each worktree asked holds that makes it unsafe to remove, and why the run daemon or the forge could not be asked.
type Facts struct {
	worktree.Repository
	Records map[string]worktree.Meta // by branch
	Located map[string]string        // the real path each name asked that is no worktree's name leads to, when something is there
	Checks  map[string]Check         // by the name a worktree is asked by
	Cwd     string                   // where the caller stands, through its symbolic links
	JobsErr error                    // why the run daemon could not tell which jobs are up
	PRsErr  error                    // why the forge could not be asked for open pull requests
}

// A Check is whether a worktree's directory is gone, and what it holds that its removal would lose or cut short: uncommitted changes, commits no other branch holds, an open pull request, jobs up.
type Check struct {
	Missing     bool
	Dirty       bool
	Unpushed    int
	PullRequest string   // the page of its open pull request, empty when it has none
	Jobs        []string // its jobs up, sorted
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

// branches are the branches of worktrees, each asked of the forge: one origin no longer has may be a pull request merged and its branch deleted.
func branches(worktrees []worktree.Worktree) []string {
	var found []string
	for _, w := range worktrees {
		if w.Branch() != "" {
			found = append(found, w.Branch())
		}
	}

	return found
}

// landed reports whether every commit of w is its merged pull request's: the head GitHub keeps is w's branch tip and the commit w stands on, so a branch squash- or rebase-merged, then deleted on origin, loses nothing; a commit made since, or a pull request still open, lands nothing.
func (f Facts) landed(prs []worktree.PullRequest, w worktree.Worktree) bool {
	pr, ok := worktree.PullRequestOf(prs, w.Branch())
	return ok && f.Cleans(pr, w, false) && (w.Head == "" || w.Head == pr.Head)
}

// ----------------------------------------------------------------------------
// The decision of each worktree

// A Blocker is what makes a worktree unsafe to remove: a safety refusal, which only Force lifts.
type Blocker string

// The blockers of a removal, in the order a person acts on them.
const (
	BlockerLocked          Blocker = "locked"            // git worktree lock: someone set it on purpose
	BlockerDirty           Blocker = "dirty"             // uncommitted changes
	BlockerUnpushed        Blocker = "unpushed"          // commits no other branch nor remote holds
	BlockerOpenPullRequest Blocker = "open_pull_request" // a pull request still open
	BlockerJobsRunning     Blocker = "jobs_running"      // jobs of the run daemon up in it
)

// Blockers returns every Blocker.
func Blockers() []Blocker {
	return []Blocker{BlockerLocked, BlockerDirty, BlockerUnpushed, BlockerOpenPullRequest, BlockerJobsRunning}
}

// A Removal is one worktree the plan removes: its name, its branch, deleted with it (none on no branch), its path (none for a branch no worktree holds, deleted alone), whether its directory is gone already, the on_clean hooks it runs first as they will run (none once its directory is gone), the phases of its removal, the namespaces its record names, which stay in their shared services, and what makes it unsafe to remove with what Force would lose.
type Removal struct {
	Worktree    string         `json:"worktree"`
	Branch      string         `json:"branch,omitempty"`
	Path        string         `json:"path"`
	Missing     bool           `json:"missing,omitempty"`
	Hooks       []string       `json:"hooks,omitempty"`
	Phases      []domain.Phase `json:"phases"`
	Namespaces  []string       `json:"namespaces,omitempty"`
	Blockers    []Blocker      `json:"blockers"`
	Unpushed    int            `json:"unpushed,omitempty"`
	PullRequest string         `json:"pull_request,omitempty"`
	Jobs        []string       `json:"jobs,omitempty"`
	entry       int            // its index among the worktrees asked
}

// entries decides each worktree names asks: a Removal, a name that is no worktree (already absent, nothing to remove), or the problem of its entry: blank, given twice, the main worktree or the base branch.
func (f Facts) entries(names []string) (removals []Removal, absent []string, problems []domain.FieldError) {
	for index, name := range names {
		path := entryPath(index)
		switch w, found := f.subject(name); {
		case strings.TrimSpace(name) == "":
			problems = append(problems, domain.Required(path))
		case slices.Contains(names[:index], name):
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeDistinct, Template: "{path}: {value} is given twice", Params: domain.Params{"value": name}})
		case found && w.Main:
			problems = append(problems, domain.FieldError{Path: path, Code: CodeMainWorktree, Template: "{value} is the main worktree: it is never removed", Params: domain.Params{"value": name}})
		case found && w.Branch() == f.Config.Worktrees.BaseBranch:
			problems = append(problems, domain.FieldError{Path: path, Code: CodeBaseBranch, Template: "{value} is the base branch: it is never removed", Params: domain.Params{"value": name}})
		case found && slices.ContainsFunc(removals, func(r Removal) bool { return r.Worktree == w.Name() || w.Path != "" && r.Path == w.Path }):
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeDistinct, Template: "{path}: {value} names a worktree given already", Params: domain.Params{"value": name}})
		case found:
			removals = append(removals, f.removal(w, index))
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

func (f Facts) removal(w worktree.Worktree, entry int) Removal {
	check := f.Checks[w.Name()]
	removal := Removal{Worktree: w.Name(), Branch: w.Branch(), Path: w.Path, Missing: w.Prunable || check.Missing, Blockers: []Blocker{}, entry: entry}
	add := func(blocker Blocker, holds bool) {
		if holds {
			removal.Blockers = append(removal.Blockers, blocker)
		}
	}

	for _, hook := range f.onClean(removal) {
		removal.Hooks = append(removal.Hooks, hook.Cmd)
	}
	removal.Phases = []domain.Phase{domain.PhaseRemove}
	if len(removal.Hooks) > 0 {
		removal.Phases = []domain.Phase{domain.PhaseOnClean, domain.PhaseRemove}
	}
	if removal.Branch != "" {
		removal.Namespaces = f.Records[removal.Branch].Namespaces
	}

	add(BlockerLocked, w.Locked)
	add(BlockerDirty, check.Dirty)
	add(BlockerUnpushed, check.Unpushed > 0)
	add(BlockerOpenPullRequest, check.PullRequest != "")
	add(BlockerJobsRunning, len(check.Jobs) > 0)
	removal.Unpushed, removal.PullRequest, removal.Jobs = check.Unpushed, check.PullRequest, check.Jobs
	return removal
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

// unsafe are the removals a blocker refuses unless forced.
func unsafe(removals []Removal) []Removal {
	return slices.DeleteFunc(slices.Clone(removals), func(r Removal) bool { return len(r.Blockers) == 0 })
}

// A Move is a child worktree a removal leaves behind: its branch, the parent removed under it, and the nearest worktree that stays, else the base branch, which reparenting records as its parent; Base says it is the base branch.
type Move struct {
	Branch string `json:"branch"`
	From   string `json:"from"`
	To     string `json:"to"`
	Base   bool   `json:"base,omitempty"`
}

// moves are the children of the removed worktrees that stay, each with where it would move: up the chain of parents, past every one removed, to the base branch at its end.
func (f Facts) moves(removals []Removal) []Move {
	removed := map[string]bool{}
	for _, r := range removals {
		if r.Branch != "" {
			removed[r.Branch] = true
		}
	}

	var moves []Move
	for _, r := range removals {
		for _, w := range f.Worktrees {
			child := w.Branch()
			if w.Main || child == "" || removed[child] || r.Branch == "" || f.Records[child].SourceBranch != r.Branch {
				continue
			}
			to := f.survivor(r.Branch, removed)
			moves = append(moves, Move{Branch: child, From: r.Branch, To: to, Base: to == f.Config.Worktrees.BaseBranch})
		}
	}

	return moves
}

// survivor is the nearest parent of branch that is not removed and still exists, a branch or a worktree's, else the base branch; a cycle of removed parents ends on the base branch too.
func (f Facts) survivor(branch string, removed map[string]bool) string {
	seen := map[string]bool{}
	for current := branch; current != "" && !seen[current]; current = f.Records[current].SourceBranch {
		if !removed[current] && (f.Local(current) || f.CheckedOutAt(current) != "") {
			return current
		}
		seen[current] = true
	}

	return f.Config.Worktrees.BaseBranch
}

// ----------------------------------------------------------------------------
// Decide

// Decide turns the facts and the resolved request into the plan and its warnings. A worktree a blocker makes unsafe refuses the whole request, before anything is removed, with CodeUnsafe, unless the request forces it.
func Decide(f Facts, req Request) (Plan, []domain.Warning, error) {
	removals, absent, _ := f.entries(req.Worktrees)
	if blocked := unsafe(removals); len(blocked) > 0 && !req.Force {
		return Plan{}, nil, refusal(blocked, req)
	}

	planned := Plan{Worktrees: removals, Absent: absent, Moves: f.moves(removals), ReparentChildren: req.ReparentChildren, Force: req.Force, facts: f}
	if planned.Worktrees == nil {
		planned.Worktrees = []Removal{}
	}
	if at := worktree.Holding(removals, f.Cwd, func(r Removal) string { return r.Path }); at >= 0 {
		planned.standing = removals[at].Worktree
	}

	return planned, f.warnings(planned), nil
}

// CodeUnsafe refuses a request that would remove a worktree a blocker makes unsafe; CodeForce is its follow-up, the same request forced, and CodeStopJobs the one that stops a worktree's jobs.
const (
	CodeUnsafe   = Name + ".unsafe"
	CodeForce    = Name + ".force"
	CodeStopJobs = Name + ".stop_jobs"
)

// refusal refuses the request with CodeUnsafe: a problem per blocker of each unsafe worktree, on its entry, followed by the request forced.
func refusal(blocked []Removal, req Request) *domain.Error {
	var fields []domain.FieldError
	names := make([]string, 0, len(blocked))
	for _, r := range blocked {
		names = append(names, r.Worktree)
		for _, blocker := range r.Blockers {
			fields = append(fields, r.problem(blocker))
		}
	}

	forced := Request{Worktrees: req.Worktrees, ReparentChildren: req.ReparentChildren, Force: true}
	return &domain.Error{
		Kind:     domain.KindRefused,
		Code:     CodeUnsafe,
		Message:  "nothing removed: " + strings.Join(names, ", ") + " can't be removed safely",
		Params:   domain.Params{"worktrees": strings.Join(names, ", ")},
		Fields:   fields,
		FollowUp: &domain.FollowUp{Command: Name, Answers: domain.AnswersOf(Questions{}.Form(), forced), Code: CodeForce, Message: "remove anyway"},
	}
}

// problem is blocker as a problem of the removal's entry, coded clean.<blocker>; jobs up are followed by the command that stops them.
func (r Removal) problem(blocker Blocker) domain.FieldError {
	problem := domain.FieldError{Path: entryPath(r.entry), Code: Name + "." + string(blocker), Params: domain.Params{"value": r.Worktree}}
	switch blocker {
	case BlockerLocked:
		problem.Template = "{value} is locked"
	case BlockerDirty:
		problem.Template = "{value} has uncommitted changes"
	case BlockerUnpushed:
		problem.Template, problem.Params["count"] = "{value} has {count} commits no other branch nor remote holds", strconv.Itoa(r.Unpushed)
		if r.Unpushed == 1 {
			problem.Template = "{value} has 1 commit no other branch nor remote holds"
		}
	case BlockerOpenPullRequest:
		problem.Template, problem.Params["url"] = "{value} has an open pull request: {url}", r.PullRequest
	case BlockerJobsRunning:
		problem.Template, problem.Params["jobs"] = "{value} runs {jobs}", strings.Join(r.Jobs, ", ")
		problem.FollowUp = &domain.FollowUp{Command: commandRunDown, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{r.Worktree}}}, Code: CodeStopJobs, Message: "stop its jobs"}
	}

	return problem
}

// commandRunDown stops a worktree's jobs: the run module's, not migrated yet.
const commandRunDown = "run down"

// CodeJobsLeft warns of the jobs a forced removal leaves running, CodeDataKept of the namespaces a removed worktree keeps in shared services.
const (
	CodeJobsLeft = Name + ".jobs_left"
	CodeDataKept = Name + ".data_kept"
)

// warnings are what the plan cannot see to or leaves behind: the jobs or pull requests that could not be asked, the jobs a forced removal leaves running, and the data each removed worktree keeps in shared services, which clean does not drop yet.
func (f Facts) warnings(planned Plan) []domain.Warning {
	var warnings []domain.Warning
	if f.JobsErr != nil {
		warnings = append(warnings, domain.Warning{Code: Name + domain.JobsUnknown, Message: run.UnknownMessage(f.JobsErr) + " — a job may still run in a worktree removed"})
	}
	if f.PRsErr != nil {
		warnings = append(warnings, domain.Warning{Code: Name + domain.PRsUnavailable, Message: "couldn't ask for open pull requests: " + f.PRsErr.Error()})
	}

	for _, r := range planned.Worktrees {
		params := domain.Params{"worktree": r.Worktree}
		if len(r.Jobs) > 0 {
			warnings = append(warnings, domain.Warning{Code: CodeJobsLeft, Message: r.Worktree + " is removed with its jobs still running: " + strings.Join(r.Jobs, ", "), Params: params})
		}
		if len(r.Namespaces) > 0 {
			params["namespaces"] = strings.Join(r.Namespaces, ", ")
			warnings = append(warnings, domain.Warning{Code: CodeDataKept, Message: r.Worktree + " keeps its data in shared services: " + params["namespaces"] + " (wtm-next does not drop it yet)", Params: params})
		}
	}

	return warnings
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: the worktrees it removes, the names that are no worktree, the children left behind, whether they are reparented, and whether the unsafe worktrees are forced; then the recap, which the fingerprint leaves out.
type Plan struct {
	Worktrees        []Removal `json:"worktrees"`
	Absent           []string  `json:"absent,omitempty"`
	Moves            []Move    `json:"moves,omitempty"`
	ReparentChildren bool      `json:"reparent_children"`
	Force            bool      `json:"force"`
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

// The notes of the recap's values: a worktree removed although unsafe, a branch no worktree holds, deleted alone, a name that is no worktree, a child left on its removed parent.
const (
	NoteForced     = "forced"
	NoteBranchOnly = "branch_only"
	NoteAbsent     = "absent"
	NoteOrphaned   = "orphaned"
)

// recap is the plan row by row: the worktrees removed, if any, each forced one with what it loses, the names that are no worktree, then the children left behind, reparented or orphaned.
func (p Plan) recap() []domain.RecapRow {
	var rows []domain.RecapRow
	if len(p.Worktrees) > 0 {
		rows = append(rows, domain.RecapRow{Key: "remove", Label: "Remove", Path: domain.PathWorktree, Values: recappedRemovals(p.Worktrees)})
	}
	if len(p.Absent) > 0 {
		values := make([]domain.RecapValue, 0, len(p.Absent))
		for _, name := range p.Absent {
			values = append(values, domain.RecapValue{Value: name, Note: NoteAbsent, Text: name + " (no worktree: nothing to remove)"})
		}
		rows = append(rows, domain.RecapRow{Key: "absent", Label: "Already absent", Path: domain.PathWorktree, Values: values})
	}

	if len(p.Moves) > 0 {
		rows = append(rows, domain.RecapRow{Key: "children", Label: "Children", Path: PathReparentChildren, Values: p.recappedMoves()})
	}

	return rows
}

func recappedRemovals(removals []Removal) []domain.RecapValue {
	values := make([]domain.RecapValue, 0, len(removals))
	for _, r := range removals {
		value := domain.RecapValue{Value: r.Worktree, Text: r.Worktree}
		var notes []string
		if r.Path == "" {
			value.Note, notes = NoteBranchOnly, append(notes, "branch only: no worktree")
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

func (p Plan) recappedMoves() []domain.RecapValue {
	values := make([]domain.RecapValue, 0, len(p.Moves))
	for _, move := range p.Moves {
		value := domain.RecapValue{Value: move.Branch, Params: domain.Params{"from": move.From, "to": move.To}, Text: move.Branch + " → " + move.To}
		if !p.ReparentChildren {
			value.Note, value.Text = NoteOrphaned, move.Branch+" (left on "+move.From+")"
		}
		values = append(values, value)
	}

	return values
}

// ----------------------------------------------------------------------------
// The outcome

// CodeGo is the step out of a worktree removed under the caller's feet, into the main one; CodeOrphaned warns of a child left on its removed parent, and CodeReparent is the step that moves it.
const (
	CodeGo       = Name + ".go"
	CodeOrphaned = Name + ".orphaned"
	CodeReparent = Name + ".reparent"
)

// next is the step into the main worktree when the caller stood in a worktree removed; none in a bare repository, which has no main checkout.
func (p Plan) next(items []Item) []domain.FollowUp {
	checkouts := p.facts.MainFirst()
	if p.standing == "" || !removedIn(items, p.standing) || len(checkouts) == 0 || !checkouts[0].Main {
		return nil
	}

	main := domain.Enter(checkouts[0].Name())
	main.Code, main.Message = CodeGo, "you stood in "+p.standing
	return []domain.FollowUp{main}
}

// orphaned warns of each child left on a parent removed, followed by the command that moves it where reparenting would have.
func orphaned(moves []Move) []domain.Warning {
	warnings := make([]domain.Warning, 0, len(moves))
	for _, move := range moves {
		warnings = append(warnings, domain.Warning{
			Code:     CodeOrphaned,
			Message:  move.Branch + " still points at the removed parent " + move.From,
			Params:   domain.Params{"branch": move.Branch, "from": move.From, "to": move.To},
			FollowUp: &domain.FollowUp{Command: commandReparent, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{move.Branch}}, {Path: "to", Values: []string{move.To}}}, Code: CodeReparent, Message: "move it onto " + move.To},
		})
	}

	return warnings
}

// commandReparent changes the parent a worktree is rebased onto: not migrated yet.
const commandReparent = "reparent"

// failure is the error a removal failed on, coded as clean's.
func failure(code, message string, err error) *domain.Error {
	return domain.AsError(domain.AsErrorParams{Err: err, Code: code, Message: message})
}
