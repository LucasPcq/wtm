package sync

import (
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what sync decides from, as Observe read them: the project's config and the repository's snapshot, the meta record of each branch and the stack they make, where each name asked as a path leads, what each worktree on a branch holds, and why origin could not be fetched.
type Facts struct {
	worktree.Repository
	Records    map[string]worktree.Meta
	RecordErrs map[string]error
	Stack      worktree.Stack
	Located    map[string]string // the worktree root each name asked that is no worktree's name leads to, when it leads to one
	Checks     map[string]Check  // by branch
	FetchErr   error             // why origin could not be fetched: the plan stands on its branches as last fetched
}

// A Check is what a worktree holds: whether its directory is gone, its uncommitted changes, the commits of its parent it lacks, the commits of origin's branch it never held, and where it forked from its parent, the commit its rebase replays after, with its own commits since, which Plan alone reads.
type Check struct {
	Missing bool
	Changes int
	Behind  int
	Never   int
	Fork    worktree.Fork
}

// Dirty reports whether the worktree holds uncommitted changes.
func (c Check) Dirty() bool {
	return c.Changes > 0
}

// candidates are the worktrees sync may rebase, in the order a reader lists them: those on a branch, but the main worktree and the base branch's, which the others are rebased onto.
func (f Facts) candidates() []worktree.Worktree {
	return slices.DeleteFunc(f.MainFirst(), func(w worktree.Worktree) bool {
		return w.Main || w.Branch() == "" || w.Branch() == f.Config.Worktrees.BaseBranch
	})
}

// subject is the worktree name names: by its name, else by the worktree root the path it stands for leads to. A part of a branch names none, nor does a branch no worktree holds.
func (f Facts) subject(name string) (worktree.Worktree, bool) {
	located, isPath := f.Located[name]
	at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Name() == name || isPath && w.Path == located })
	if at < 0 {
		return worktree.Worktree{}, false
	}

	return f.Worktrees[at], true
}

// checkout is the worktree on branch.
func (f Facts) checkout(branch string) worktree.Worktree {
	at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Branch() == branch })
	if at < 0 {
		return worktree.Worktree{}
	}
	return f.Worktrees[at]
}

// picked are the worktrees picked by default: those that need a sync, their parent having commits they lack or being behind origin, which the sync fast-forwards first, and no operation under way; but one with uncommitted changes, which the command keeps when it chose it.
func (f Facts) picked() []string {
	var names []string
	for _, w := range f.candidates() {
		check := f.Checks[w.Branch()]
		parent, ok := f.Stack.Parent(w.Branch())
		behind := check.Behind
		if ok && f.Local(parent) && f.Standing(parent) == worktree.Behind {
			behind++
		}
		if !check.Missing && !check.Dirty() && w.NeedsSync(&behind) {
			names = append(names, w.Branch())
		}
	}

	return names
}

// under are the worktrees under each worktree req names, by the named one, those named themselves left out; none for a named one with none under it.
func (f Facts) under(req Request) map[string][]string {
	names, _, _ := f.named(req)
	under := map[string][]string{}
	for _, name := range names {
		steps, _ := f.Stack.Cascade([]string{name})
		if below := slices.DeleteFunc(steps, func(b string) bool { return slices.Contains(names, b) }); len(below) > 0 {
			under[name] = below
		}
	}

	return under
}

// remoteTip is the commit origin's branch name stands at as last fetched, "" when origin has none.
func (f Facts) remoteTip(name string) string {
	at := slices.IndexFunc(f.Branches.Remote, func(b worktree.Branch) bool { return b.Name == name })
	if at < 0 {
		return ""
	}
	return f.Branches.Remote[at].Tip
}

// ----------------------------------------------------------------------------
// The decision of each worktree

// A Standing is where a worktree stands for a sync, the first that applies: its directory gone, an operation git stopped in the middle of, its meta record unreadable, its parents leading back to it, no parent recorded, its parent no branch any more, its parent left as it is by this run, origin holding commits on its branch it never held, then whether it rebases, its parent having commits it lacks or moving in this run, or already stands on its parent.
type Standing string

// The standings of a worktree.
const (
	StandingMissing    Standing = "missing"
	StandingBusy       Standing = "busy"
	StandingUnreadable Standing = "unreadable"
	StandingInCycle    Standing = "in_cycle"
	StandingNoParent   Standing = "no_parent"
	StandingParentGone Standing = "parent_gone"
	StandingParentLeft Standing = "parent_left"
	StandingDiverged   Standing = "diverged"
	StandingRebases    Standing = "rebases"
	StandingOnParent   Standing = "on_parent"
)

// Standings returns every Standing.
func Standings() []Standing {
	return []Standing{StandingMissing, StandingBusy, StandingUnreadable, StandingInCycle, StandingNoParent, StandingParentGone, StandingParentLeft, StandingDiverged, StandingRebases, StandingOnParent}
}

// A Blocker is what makes a branch unsafe to move: a safety refusal, which only Force lifts.
type Blocker string

// BlockerDirty is uncommitted changes, which a rebase or a fast-forward would carry over; forced, a rebase stashes them around itself.
const BlockerDirty Blocker = "dirty"

// Blockers returns every Blocker.
func Blockers() []Blocker {
	return []Blocker{BlockerDirty}
}

// A Rebase is one worktree of the cascade: its name, its path, its parent and how deep it sits in the stack (1 under a root), whether it is there as a descendant of a worktree named rather than named itself, where it stands, the operation git stopped in the middle of in it when busy, its branch against origin's (null when origin has none), the commit it stands at, where it forked from its parent, its own commits, the commit origin's branch stands at (the lease of its push), what makes it unsafe, and the phases of its sync, none when the plan leaves it as it is.
type Rebase struct {
	Worktree   string             `json:"worktree"`
	Path       string             `json:"path"`
	Parent     string             `json:"parent"`
	Depth      int                `json:"depth"`
	Descendant bool               `json:"descendant"`
	Standing   Standing           `json:"standing"`
	Operation  worktree.Operation `json:"operation,omitempty"`
	Origin     *worktree.Origin   `json:"origin"`
	From       string             `json:"from"`
	Fork       string             `json:"fork,omitempty"`
	Commits    int                `json:"commits"`
	Expect     string             `json:"expect,omitempty"`
	Blockers   []Blocker          `json:"blockers"`
	Phases     []domain.Phase     `json:"phases"`
	entry      int                // its index among the worktrees named, -1 when the command chose it
}

// moves reports whether the plan moves the worktree's branch: rebased or fast-forwarded.
func (r Rebase) moves() bool {
	return slices.Contains(r.Phases, domain.PhaseRebase) || slices.Contains(r.Phases, domain.PhaseFastForward)
}

func (r Rebase) named() bool {
	return r.entry >= 0
}

// mayMove reports whether the worktree may move once origin is fetched: it rebases, or stands on its parent as last fetched.
func (r Rebase) mayMove() bool {
	return r.Standing == StandingRebases || r.Standing == StandingOnParent
}

// left reports whether the plan leaves the worktree as it is for a reason it warns of: not one already on its parent.
func (r Rebase) left() bool {
	return len(r.Phases) == 0 && r.Standing != StandingOnParent
}

// An Onto is a branch the cascade rebases onto without rebasing it, the base branch included: its name, the path of its worktree ("" when none holds it), the worktrees of the cascade on it, the operation git stopped in the middle of in its worktree, its branch against origin's, the commit it stands at and, behind origin, the one origin's stands at, what makes it unsafe, and the phases of its fast-forward, none when it is left as it is.
type Onto struct {
	Branch    string             `json:"branch"`
	Path      string             `json:"path,omitempty"`
	Children  []string           `json:"children"`
	Operation worktree.Operation `json:"operation,omitempty"`
	Origin    *worktree.Origin   `json:"origin"`
	From      string             `json:"from"`
	To        string             `json:"to,omitempty"`
	Blockers  []Blocker          `json:"blockers"`
	Phases    []domain.Phase     `json:"phases"`
}

// behind reports whether origin's branch is ahead of it and it of nothing: a fast-forward brings it up.
func (o Onto) behind() bool {
	return o.Origin != nil && o.Origin.State == worktree.Behind
}

func (o Onto) moves() bool {
	return len(o.Phases) > 0
}

// decided is what req makes of the facts: the plan, the request's problems and the worktrees named in a cycle.
type decided struct {
	plan     Plan
	problems []domain.FieldError
	cycled   []string
}

// decide plans req on the facts: the cascade of the worktrees it asks, each worktree decided in its order, parents before children, after the branches they are rebased onto.
func (f Facts) decide(req Request) decided {
	names, entries, problems := f.named(req)
	roots := names
	switch {
	case req.All:
		roots = branchesOf(f.candidates())
	case !req.named():
		roots = f.picked()
	}

	steps, cycled := f.cascade(roots, req.All || req.WithDescendants())
	d := decided{problems: problems, plan: Plan{Descendants: req.WithDescendants(), Force: req.Force, KeepConflict: req.KeepConflict, FFParents: req.WithParents(), Push: req.Push}}
	for _, name := range cycled {
		if _, named := entries[name]; named {
			d.cycled = append(d.cycled, name)
		}
	}

	rebases := make([]Rebase, 0, len(steps)+len(cycled))
	for _, branch := range append(steps, cycled...) {
		at, named := entries[branch]
		if !named {
			at = -1
		}
		rebases = append(rebases, f.rebase(f.checkout(branch), at, req.named() && !named))
	}

	ontos := f.ontos(rebases)
	moving := map[string]bool{}
	for i := range ontos {
		f.decideOnto(&ontos[i], req)
		moving[ontos[i].Branch] = ontos[i].moves()
	}
	left := map[string]bool{}
	for i := range rebases {
		r := &rebases[i]
		r.Standing = f.standing(*r, left, moving)
		f.decideRebase(r, req)
		moving[r.Worktree], left[r.Worktree] = r.moves(), r.left()
	}

	d.plan.Worktrees, d.plan.Onto, d.plan.LeftOut = rebases, ontos, []string{}
	if !req.All && !req.WithDescendants() {
		for _, under := range f.under(req) {
			d.plan.LeftOut = append(d.plan.LeftOut, under...)
		}
		slices.Sort(d.plan.LeftOut)
		d.plan.LeftOut = slices.Compact(d.plan.LeftOut)
	}
	return d
}

// cascade is roots and, deep, the worktrees under them, parents before children (worktree.Stack.Cascade), and apart the roots whose parents lead back to them, which no cascade reaches.
func (f Facts) cascade(roots []string, deep bool) (steps, cycled []string) {
	reached := slices.DeleteFunc(slices.Clone(roots), func(branch string) bool {
		if f.Stack.InCycle(branch) {
			cycled = append(cycled, branch)
			return true
		}
		return false
	})
	// no root is a cycle's member any more: the cascade reaches none
	steps, _ = f.Stack.Cascade(reached)
	if !deep {
		steps = slices.DeleteFunc(steps, func(branch string) bool { return !slices.Contains(reached, branch) })
	}

	return steps, cycled
}

// rebase is the worktree w as the facts have it, its standing and phases yet to decide; entry is its index among the names, -1 when the command chose it, and descendant whether it came under one named.
func (f Facts) rebase(w worktree.Worktree, entry int, descendant bool) Rebase {
	branch, check := w.Branch(), f.Checks[w.Branch()]
	parent, _ := f.Stack.Parent(branch)
	return Rebase{Worktree: branch, Path: w.Path, Parent: parent, Depth: len(f.Stack.Ancestors(branch)), Descendant: descendant, Operation: w.Operation(), Origin: f.OriginOf(branch), From: f.Tip(branch), Fork: check.Fork.Commit, Commits: check.Fork.Own, Expect: f.remoteTip(branch), Blockers: []Blocker{}, Phases: []domain.Phase{}, entry: entry}
}

// ontos are the local branches the worktrees of rebases are rebased onto without being among them, each once, in the order they are first rebased onto, with the worktrees on each.
func (f Facts) ontos(rebases []Rebase) []Onto {
	var ontos []Onto
	in := map[string]bool{}
	for _, r := range rebases {
		in[r.Worktree] = true
	}
	for _, r := range rebases {
		if r.Parent == "" || in[r.Parent] || !f.Local(r.Parent) {
			continue
		}
		if at := slices.IndexFunc(ontos, func(o Onto) bool { return o.Branch == r.Parent }); at >= 0 {
			ontos[at].Children = append(ontos[at].Children, r.Worktree)
			continue
		}
		holder := f.checkout(r.Parent)
		ontos = append(ontos, Onto{Branch: r.Parent, Path: holder.Path, Children: []string{r.Worktree}, Operation: holder.Operation(), Origin: f.OriginOf(r.Parent), From: f.Tip(r.Parent), Blockers: []Blocker{}, Phases: []domain.Phase{}})
	}

	return ontos
}

// decideOnto plans the fast-forward of o when req asks for it and origin is ahead of it: never while git is in the middle of an operation in its worktree, nor, unforced, while that worktree has uncommitted changes.
func (f Facts) decideOnto(o *Onto, req Request) {
	if !o.behind() || !req.WithParents() || o.Operation != "" {
		return
	}

	o.To = f.remoteTip(o.Branch)
	if o.Path != "" && f.Checks[o.Branch].Dirty() {
		o.Blockers = append(o.Blockers, BlockerDirty)
	}
	if len(o.Blockers) == 0 || req.Force {
		o.Phases = []domain.Phase{domain.PhaseFastForward}
	}
}

// standing is where r stands, the first that applies, its parent left or moving as left and moving say of the branches decided before it.
func (f Facts) standing(r Rebase, left, moving map[string]bool) Standing {
	check := f.Checks[r.Worktree]
	switch {
	case check.Missing:
		return StandingMissing
	case r.Operation != "":
		return StandingBusy
	case f.RecordErrs[r.Worktree] != nil:
		return StandingUnreadable
	case f.Stack.InCycle(r.Worktree):
		return StandingInCycle
	case r.Parent == "":
		return StandingNoParent
	case f.ParentGone(r.Parent):
		return StandingParentGone
	case left[r.Parent]:
		return StandingParentLeft
	case r.Origin != nil && r.Origin.State == worktree.Diverged && check.Never > 0:
		return StandingDiverged
	case check.Behind > 0 || moving[r.Parent]:
		return StandingRebases
	}

	return StandingOnParent
}

// decideRebase plans r's phases from its standing: a worktree that rebases first fast-forwards onto its own origin when origin is ahead of it alone, then rebases, then pushes when req asks it; one on its parent only catches up with origin, or pushes what origin lacks. A branch origin has never had is never pushed: publishing it is the user's call. A worktree the command chose that a blocker makes unsafe, unforced, is kept, with no phase; one named keeps its phases, for Decide to refuse.
func (f Facts) decideRebase(r *Rebase, req Request) {
	var pushable bool
	behindOrigin := r.Origin != nil && r.Origin.State == worktree.Behind
	switch r.Standing {
	case StandingRebases:
		if behindOrigin {
			r.Phases = append(r.Phases, domain.PhaseFastForward)
		}
		r.Phases, pushable = append(r.Phases, domain.PhaseRebase), r.Origin != nil
	case StandingOnParent:
		if behindOrigin {
			r.Phases = append(r.Phases, domain.PhaseFastForward)
		}
		pushable = !behindOrigin && r.Origin != nil && r.Origin.Ahead > 0
	default:
		return
	}

	if r.moves() && f.Checks[r.Worktree].Dirty() {
		r.Blockers = append(r.Blockers, BlockerDirty)
	}
	if len(r.Blockers) > 0 && !req.Force && !r.named() {
		r.Phases = []domain.Phase{}
		return
	}
	if pushable && req.Push {
		r.Phases = append(r.Phases, domain.PhasePush)
	}
}

// mayBeUnsafe are the worktrees named with uncommitted changes that may move once origin is fetched.
func (f Facts) mayBeUnsafe(rebases []Rebase) []Rebase {
	return slices.DeleteFunc(slices.Clone(rebases), func(r Rebase) bool { return !r.named() || !r.mayMove() || !f.Checks[r.Worktree].Dirty() })
}

// unsafe are the worktrees a blocker refuses unless forced, among those planned.
func unsafe(rebases []Rebase) []Rebase {
	return slices.DeleteFunc(slices.Clone(rebases), func(r Rebase) bool { return len(r.Blockers) == 0 || !r.named() })
}

// behind are the branches rebased onto that origin is ahead of.
func (d decided) behind() []Onto {
	return slices.DeleteFunc(slices.Clone(d.plan.Onto), func(o Onto) bool { return !o.behind() })
}

func branchesOf(worktrees []worktree.Worktree) []string {
	names := make([]string, 0, len(worktrees))
	for _, w := range worktrees {
		names = append(names, w.Branch())
	}
	return names
}

// ----------------------------------------------------------------------------
// Decide

// Decide turns the facts and the resolved request into the plan and its warnings: the cascade of the worktrees asked, each rebased onto its parent after the branches they are rebased onto are fast-forwarded. A worktree named that an unforced blocker makes unsafe refuses the whole request with CodeUnsafe, one named whose parents lead back to it with CodeCycle; one the command chose is kept, or left, with a warning.
func Decide(f Facts, req Request) (Plan, []domain.Warning, error) {
	d := f.decide(req)
	if len(d.cycled) > 0 {
		return Plan{}, nil, f.cycleRefusal(d.cycled)
	}
	if blocked := unsafe(d.plan.Worktrees); len(blocked) > 0 && !req.Force {
		return Plan{}, nil, refusal(blocked, req)
	}

	return d.plan, f.warnings(d.plan, req), nil
}

// CodeUnsafe refuses a request that names a worktree a blocker makes unsafe; CodeForce is its follow-up, the same request forced. CodeCycle refuses a request that names a worktree whose parents lead back to it, followed by CodeReparent.
const (
	CodeUnsafe   = Name + ".unsafe"
	CodeForce    = Name + ".force"
	CodeCycle    = Name + ".cycle"
	CodeReparent = Name + ".reparent"
)

// refusal refuses the request with CodeUnsafe: a problem per blocker of each unsafe worktree, on its entry, coded sync.<blocker>, followed by the request forced.
func refusal(blocked []Rebase, req Request) *domain.Error {
	var fields []domain.FieldError
	names := make([]string, 0, len(blocked))
	for _, r := range blocked {
		names = append(names, r.Worktree)
		for _, blocker := range r.Blockers {
			fields = append(fields, domain.FieldError{Path: entryPath(r.entry), Code: Name + "." + string(blocker), Template: "{value} has " + blockerWords[blocker], Params: domain.Params{"value": r.Worktree}})
		}
	}

	forced := req
	forced.Force, forced.Caller = true, domain.Caller{}
	return &domain.Error{
		Kind:     domain.KindRefused,
		Code:     CodeUnsafe,
		Message:  "nothing synced: " + strings.Join(names, ", ") + " can't be rebased safely",
		Params:   domain.Params{"worktrees": strings.Join(names, ", ")},
		Fields:   fields,
		FollowUp: &domain.FollowUp{Command: Name, Answers: domain.AnswersOf(Questions{}.Form(), forced), Code: CodeForce, Message: "sync anyway, its changes stashed around the rebase"},
	}
}

// cycleRefusal refuses the request with CodeCycle, followed by the move of the first worktree named in the cycle onto the base branch, when there is one to move onto.
func (f Facts) cycleRefusal(cycled []string) *domain.Error {
	them := "them"
	if len(cycled) == 1 {
		them = "it"
	}
	refused := &domain.Error{Kind: domain.KindRefused, Code: CodeCycle, Message: "nothing synced: the parents of " + strings.Join(cycled, ", ") + " lead back to " + them, Params: domain.Params{"worktrees": strings.Join(cycled, ", ")}}
	if reparent, ok := f.reparent(cycled[0]); ok {
		refused.FollowUp = &reparent
	}
	return refused
}

// blockerWords say what each blocker is, after "has".
var blockerWords = map[Blocker]string{BlockerDirty: "uncommitted changes"}

// CodeDirty is the problem of a named worktree with uncommitted changes, and the failure of one that gained them since the plan; CodeKept warns of a branch the command chose and kept for its blockers.
const (
	CodeDirty = Name + "." + string(BlockerDirty)
	CodeKept  = Name + ".kept"
)

// The codes of the warnings of a worktree left as it is, or of a branch rebased onto as it stands, and of a fetch that failed: each says why, followed by what moves it on.
const (
	CodeMissing      = Name + ".missing"
	CodeBusy         = Name + ".busy"
	CodeUnreadable   = Name + ".meta_unreadable"
	CodeInCycle      = Name + ".in_cycle"
	CodeNoParent     = Name + ".no_parent"
	CodeParentGone   = Name + ".parent_gone"
	CodeParentLeft   = Name + ".parent_left"
	CodeDiverged     = Name + ".diverged"
	CodeOntoBehind   = Name + ".onto_behind"
	CodeOntoDiverged = Name + ".onto_diverged"
	CodeFetchFailed  = Name + ".fetch_failed"
	CodeGo           = Name + ".go"
	CodeClean        = Name + ".clean"
	CodeParents      = Name + ".parents"
)

// warnings are what the plan cannot see to or leaves as it is: the fetch that failed, each branch rebased onto as it stands, each worktree left, by why, with what moves it on, then each branch kept for its blockers.
func (f Facts) warnings(planned Plan, req Request) []domain.Warning {
	var warnings []domain.Warning
	if f.FetchErr != nil {
		warnings = append(warnings, domain.Warning{Code: CodeFetchFailed, Message: "couldn't fetch origin: where each branch stands against it is as last fetched: " + f.FetchErr.Error(), Params: domain.Params{"cause": f.FetchErr.Error()}})
	}

	for _, o := range planned.Onto {
		if warning, ok := f.ontoWarning(o, req); ok {
			warnings = append(warnings, warning)
		}
	}
	for _, r := range planned.Worktrees {
		if warning, ok := f.leftWarning(r); ok {
			warnings = append(warnings, warning)
		}
	}

	return append(warnings, kept(planned, req)...)
}

// ontoWarning is the warning of a branch rebased onto as it stands though origin is ahead of it or holds commits it never held, followed, when the request did not fast-forward it, by the request that does; none for one kept for its blockers, which kept warns of, nor for one diverged only by its own rewrite.
func (f Facts) ontoWarning(o Onto, req Request) (domain.Warning, bool) {
	params := domain.Params{"branch": o.Branch, "children": strings.Join(o.Children, ", ")}
	switch {
	case o.moves() || o.Origin == nil || len(o.Blockers) > 0:
		return domain.Warning{}, false
	case o.Origin.State == worktree.Diverged && f.Checks[o.Branch].Never > 0:
		params["ahead"], params["behind"] = strconv.Itoa(o.Origin.Ahead), strconv.Itoa(o.Origin.Behind)
		return domain.Warning{Code: CodeOntoDiverged, Message: o.Branch + " has diverged from origin: " + params["children"] + " rebased onto it as it stands", Params: params}, true
	case o.Operation != "" && o.behind():
		params["operation"] = string(o.Operation)
		enter := domain.Enter(o.Branch)
		enter.Code, enter.Message = CodeGo, "finish it in the worktree"
		return domain.Warning{Code: CodeBusy, Message: o.Branch + " is in the middle of an operation (" + string(o.Operation) + "): " + params["children"] + " rebased onto it as it stands", Params: params, FollowUp: &enter}, true
	case o.behind():
		params["behind"] = strconv.Itoa(o.Origin.Behind)
		with := req
		with.FFParents, with.Caller = new(true), domain.Caller{}
		return domain.Warning{Code: CodeOntoBehind, Message: o.Branch + " is " + commits(o.Origin.Behind) + " behind origin: " + params["children"] + " rebased onto it as it stands", Params: params, FollowUp: &domain.FollowUp{Command: Name, Answers: domain.AnswersOf(Questions{}.Form(), with), Code: CodeParents, Message: "fast-forward it first"}}, true
	}

	return domain.Warning{}, false
}

// leftWarning is the warning of a worktree the plan leaves as it is, and whether it has one: one on its parent has nothing to say, and one kept for its blockers is warned of by kept.
func (f Facts) leftWarning(r Rebase) (domain.Warning, bool) {
	params := domain.Params{"worktree": r.Worktree}
	on := []domain.Answered{{Path: domain.PathWorktree, Values: []string{r.Worktree}}}
	if !r.left() || len(r.Blockers) > 0 {
		return domain.Warning{}, false
	}

	switch r.Standing {
	case StandingMissing:
		return domain.Warning{Code: CodeMissing, Message: r.Worktree + "'s directory is gone: left as it is", Params: params, FollowUp: &domain.FollowUp{Command: domain.CommandClean, Answers: on, Code: CodeClean, Message: "remove what is left of it"}}, true
	case StandingBusy:
		params["operation"] = string(r.Operation)
		enter := domain.Enter(r.Worktree)
		enter.Code, enter.Message = CodeGo, "finish it in the worktree"
		return domain.Warning{Code: CodeBusy, Message: r.Worktree + " is in the middle of an operation (" + string(r.Operation) + "): left as it is", Params: params, FollowUp: &enter}, true
	case StandingUnreadable:
		params["cause"] = f.RecordErrs[r.Worktree].Error()
		return domain.Warning{Code: CodeUnreadable, Message: "couldn't read the record of " + r.Worktree + ", which names its parent: left as it is: " + params["cause"], Params: params}, true
	case StandingInCycle, StandingParentGone:
		return f.parentWarning(r, params), true
	case StandingNoParent:
		return domain.Warning{Code: CodeNoParent, Message: r.Worktree + " records no parent to rebase onto: left as it is", Params: params}, true
	case StandingParentLeft:
		params["parent"] = r.Parent
		return domain.Warning{Code: CodeParentLeft, Message: r.Worktree + " stays on " + r.Parent + ", which this sync leaves as it is", Params: params}, true
	case StandingDiverged:
		params["ahead"], params["behind"] = strconv.Itoa(r.Origin.Ahead), strconv.Itoa(r.Origin.Behind)
		enter := domain.Enter(r.Worktree)
		enter.Code, enter.Message = CodeGo, "bring origin's commits in by hand"
		return domain.Warning{Code: CodeDiverged, Message: "origin has commits on " + r.Worktree + " it never held: left as it is", Params: params, FollowUp: &enter}, true
	case StandingRebases, StandingOnParent:
	}

	return domain.Warning{}, false
}

// parentWarning is the warning of a worktree whose parent is no branch any more or leads back to it, followed by its move onto the base branch, when there is one to move onto.
func (f Facts) parentWarning(r Rebase, params domain.Params) domain.Warning {
	warning := domain.Warning{Code: CodeParentGone, Message: r.Worktree + "'s parent " + r.Parent + " is no branch any more: left as it is", Params: params}
	if r.Standing == StandingInCycle {
		warning = domain.Warning{Code: CodeInCycle, Message: "the parents of " + r.Worktree + " lead back to it: left as it is", Params: params}
	}
	if reparent, ok := f.reparent(r.Worktree); ok {
		warning.FollowUp = &reparent
	}
	return warning
}

// reparent is the move of branch onto the base branch, and whether there is one to move onto.
func (f Facts) reparent(branch string) (domain.FollowUp, bool) {
	base := f.ExistingBase()
	if base == "" {
		return domain.FollowUp{}, false
	}
	return domain.FollowUp{Command: domain.CommandReparent, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{branch}}, {Path: domain.PathTo, Values: []string{base}}}, Code: CodeReparent, Message: "move it onto " + base}, true
}

// kept warns of each branch the command chose and kept for its blockers, the last followed by the request that forces them: the worktrees kept named, forced, or the request forced when only a branch rebased onto is kept.
func kept(planned Plan, req Request) []domain.Warning {
	var warnings []domain.Warning
	var names []string
	keep := func(name string, blockers []Blocker) {
		reasons := make([]string, 0, len(blockers))
		for _, blocker := range blockers {
			reasons = append(reasons, blockerWords[blocker])
		}
		warnings = append(warnings, domain.Warning{Code: CodeKept, Message: name + " is kept as it is: " + strings.Join(reasons, ", "), Params: domain.Params{"worktree": name, "blockers": strings.Join(domain.Names(blockers), ", ")}})
	}
	for _, o := range planned.Onto {
		if !o.moves() && len(o.Blockers) > 0 {
			keep(o.Branch, o.Blockers)
		}
	}
	for _, r := range planned.Worktrees {
		if !r.moves() && len(r.Blockers) > 0 {
			keep(r.Worktree, r.Blockers)
			names = append(names, r.Worktree)
		}
	}
	if len(warnings) == 0 {
		return nil
	}

	forced := req
	forced.Force, forced.Caller = true, domain.Caller{}
	if len(names) > 0 {
		forced = Request{Worktrees: names, Force: true, Descendants: planned.descendantsAnswer()}
	}
	warnings[len(warnings)-1].FollowUp = &domain.FollowUp{Command: Name, Answers: domain.AnswersOf(Questions{}.Form(), forced), Code: CodeForce, Message: "sync them anyway, their changes stashed around the rebase"}
	return warnings
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: every worktree of the cascade in its order, those that move and those left as they are, the branches they are rebased onto, the worktrees under those named the run leaves out without its descendants, and the answers that shape the run; then the recap, which the fingerprint leaves out.
type Plan struct {
	Worktrees    []Rebase `json:"worktrees"`
	Onto         []Onto   `json:"onto"`
	LeftOut      []string `json:"left_out"`
	Descendants  bool     `json:"descendants"`
	Force        bool     `json:"force"`
	KeepConflict bool     `json:"keep_conflict"`
	FFParents    bool     `json:"ff_parents"`
	Push         bool     `json:"push"`
	domain.Recapped
}

// Locks are the locks the plan needs: the repository shared, and each branch it moves exclusive.
func (p Plan) Locks() []domain.Lock {
	var names []string
	for _, o := range p.Onto {
		if o.moves() {
			names = append(names, o.Branch)
		}
	}
	for _, r := range p.Worktrees {
		if len(r.Phases) > 0 {
			names = append(names, r.Worktree)
		}
	}

	return domain.WorktreeLocks(names)
}

// Fingerprinted is what the plan's fingerprint covers: the plan, the commits each branch stands at, forked from and is pushed over among it, its recap left out.
func (p Plan) Fingerprinted() any {
	p.Recapped = domain.Recapped{}
	return p
}

// Locks are the locks taken before planning: the repository shared, and each worktree named exclusive; the others are taken once the plan names those that move.
func (r Request) Locks() []domain.Lock {
	return domain.WorktreeLocks(slices.DeleteFunc(slices.Clone(r.Worktrees), func(name string) bool { return strings.TrimSpace(name) == "" }))
}

// empty reports whether applying the plan changes nothing: no phase anywhere.
func (p Plan) empty() bool {
	return !slices.ContainsFunc(p.Onto, Onto.moves) && !slices.ContainsFunc(p.Worktrees, func(r Rebase) bool { return len(r.Phases) > 0 })
}

// ----------------------------------------------------------------------------
// The recap

// The notes of the recap's values: a worktree there as a descendant of one named, one rebased with its uncommitted changes stashed around the rebase, the descendants left out, or none under the named, and why one is left as it is, its standing or its blocker.
const (
	NoteDescendant = "descendant"
	NoteForced     = "forced"
	NoteOnlyNamed  = "only_named"
	NoteNone       = "none"
)

// recap is the plan row by row: the descendants of the named worktrees, when it names some, the branches fast-forwarded first, the worktrees rebased, each onto its parent with its commits, those pushed, what a conflict does, those already on their parent, then those left as they are, each with why.
func (p Plan) recap(named bool) []domain.RecapRow {
	var forwarded, rebased, pushed, current, left []domain.RecapValue
	for _, o := range p.Onto {
		if o.moves() {
			forwarded = append(forwarded, domain.RecapValue{Value: o.Branch, Params: domain.Params{"commits": strconv.Itoa(o.Origin.Behind)}, Text: o.Branch + " (" + commits(o.Origin.Behind) + " from origin)"})
		}
	}
	for _, r := range p.Worktrees {
		switch {
		case slices.Contains(r.Phases, domain.PhaseRebase):
			rebased = append(rebased, rebasedValue(r))
		case slices.Contains(r.Phases, domain.PhaseFastForward):
			forwarded = append(forwarded, domain.RecapValue{Value: r.Worktree, Params: domain.Params{"commits": strconv.Itoa(r.Origin.Behind)}, Text: r.Worktree + " (" + commits(r.Origin.Behind) + " from origin)"})
		case r.Standing == StandingOnParent:
			current = append(current, domain.RecapValue{Value: r.Worktree, Text: r.Worktree})
		default:
			left = append(left, leftValue(r))
		}
		if slices.Contains(r.Phases, domain.PhasePush) {
			pushed = append(pushed, domain.RecapValue{Value: r.Worktree, Text: r.Worktree})
		}
	}

	var rows []domain.RecapRow
	if named {
		rows = append(rows, domain.RecapRow{Key: "descendants", Label: "Descendants", Path: PathDescendants, Values: p.descendants()})
	}
	for _, row := range []domain.RecapRow{
		{Key: "fast_forward", Label: "Fast-forward", Path: PathFFParents, Values: forwarded},
		{Key: "rebase", Label: "Rebase", Path: domain.PathWorktree, Values: rebased},
		{Key: "push", Label: "Push", Path: PathPush, Values: pushed},
	} {
		if len(row.Values) > 0 {
			rows = append(rows, row)
		}
	}
	if len(rebased) > 0 {
		rows = append(rows, p.onConflict())
	}
	if len(current) > 0 {
		rows = append(rows, domain.RecapRow{Key: "on_parent", Label: "On their parent", Path: domain.PathWorktree, Values: current})
	}
	if len(left) > 0 {
		rows = append(rows, domain.RecapRow{Key: "left", Label: "Left as is", Path: domain.PathWorktree, Values: left})
	}
	return rows
}

// descendants are the values of the descendants' row: each worktree there as a descendant, else those left out, or that there are none.
func (p Plan) descendants() []domain.RecapValue {
	var values []domain.RecapValue
	for _, r := range p.Worktrees {
		if r.Descendant {
			values = append(values, domain.RecapValue{Value: r.Worktree, Text: r.Worktree})
		}
	}
	switch {
	case !p.Descendants && len(p.LeftOut) > 0:
		stay := " stay where they are"
		if len(p.LeftOut) == 1 {
			stay = " stays where it is"
		}
		return []domain.RecapValue{{Note: NoteOnlyNamed, Params: domain.Params{"left_out": strings.Join(p.LeftOut, ", ")}, Text: "only the named: " + strings.Join(p.LeftOut, ", ") + stay}}
	case !p.Descendants:
		return []domain.RecapValue{{Note: NoteOnlyNamed, Text: "only the named"}}
	case len(values) == 0:
		return []domain.RecapValue{{Note: NoteNone, Text: "none under the named"}}
	}
	return values
}

func (p Plan) onConflict() domain.RecapRow {
	value := domain.RecapValue{Value: "false", Text: "abort the rebase, the worktree left as it was"}
	if p.KeepConflict {
		value = domain.RecapValue{Value: "true", Text: "leave the rebase in progress, to resolve"}
	}
	return domain.RecapRow{Key: "keep_conflict", Label: "On conflict", Path: PathKeepConflict, Values: []domain.RecapValue{value}}
}

func rebasedValue(r Rebase) domain.RecapValue {
	value := domain.RecapValue{Value: r.Worktree, Params: domain.Params{"parent": r.Parent, "commits": strconv.Itoa(r.Commits)}, Text: r.Worktree + " onto " + r.Parent + " (" + commits(r.Commits) + ")"}
	switch {
	case len(r.Blockers) > 0:
		value.Note, value.Text = NoteForced, r.Worktree+" onto "+r.Parent+" ("+commits(r.Commits)+"; uncommitted changes stashed around it)"
	case r.Descendant:
		value.Note = NoteDescendant
	}
	return value
}

var leftWords = map[Standing]string{
	StandingMissing:    "its directory is gone",
	StandingBusy:       "an operation is under way",
	StandingUnreadable: "its record does not read",
	StandingInCycle:    "its parents lead back to it",
	StandingNoParent:   "no parent recorded",
	StandingParentGone: "its parent is gone",
	StandingParentLeft: "its parent is left as it is",
	StandingDiverged:   "origin has commits it never held",
	StandingRebases:    "uncommitted changes",
	StandingOnParent:   "uncommitted changes",
}

func leftValue(r Rebase) domain.RecapValue {
	note := string(r.Standing)
	if len(r.Blockers) > 0 {
		note = string(BlockerDirty)
	}
	return domain.RecapValue{Value: r.Worktree, Note: note, Text: r.Worktree + " (" + leftWords[r.Standing] + ")"}
}

func commits(n int) string {
	if n == 1 {
		return "1 commit"
	}
	return strconv.Itoa(n) + " commits"
}

// ----------------------------------------------------------------------------
// The outcome

// The codes of a worktree's failure: a rebase stopped on a conflict (CodeConflict), git refusing otherwise (CodeFailed), uncommitted changes or an operation it gained since the plan (CodeDirty, CodeBusy), its push refused (CodePushFailed); CodeStashed warns of changes git kept in its stash.
const (
	CodeConflict   = Name + ".conflict"
	CodeFailed     = Name + ".failed"
	CodePushFailed = Name + ".push_failed"
	CodeStashed    = Name + ".stashed"
)

// The codes of the steps after a run: resolving an aborted conflict by hand (CodeResolve), syncing what is left once a conflict left in progress is resolved (CodeResume), and pushing what was synced and not pushed (CodePush).
const (
	CodeResolve = Name + ".resolve"
	CodeResume  = Name + ".resume"
	CodePush    = Name + ".push"
)

// next are the steps after the run: for each conflict, aborted, the same worktree synced again with its conflict left in progress, or, left in progress, finishing it in the worktree then syncing what is left; then, unpushed, the push of every worktree synced.
func (p Plan) next(items []Item) []domain.FollowUp {
	form := Questions{}.Form()
	var synced []string
	var followUps []domain.FollowUp
	for _, item := range items {
		switch {
		case item.Err != nil && item.Err.Code == CodeConflict && item.Err.Params["kept"] == "true":
			enter := domain.Enter(item.Subject)
			enter.Code, enter.Message = CodeGo, "resolve it, then git rebase --continue"
			followUps = append(followUps, enter, domain.FollowUp{Command: Name, Answers: domain.AnswersOf(form, Request{Worktrees: []string{item.Subject}, Descendants: p.descendantsAnswer()}), Code: CodeResume, Message: "then sync what is left"})
		case item.Err != nil && item.Err.Code == CodeConflict:
			followUps = append(followUps, domain.FollowUp{Command: Name, Answers: domain.AnswersOf(form, Request{Worktrees: []string{item.Subject}, Descendants: p.descendantsAnswer(), KeepConflict: true}), Code: CodeResolve, Message: "resolve the conflict by hand"})
		case item.Status == domain.StatusDone && p.unpushed(item.Subject):
			synced = append(synced, item.Subject)
		}
	}
	if !p.Push && len(synced) > 0 {
		message := "push them, force-with-lease"
		if len(synced) == 1 {
			message = "push it, force-with-lease"
		}
		followUps = append(followUps, domain.FollowUp{Command: Name, Answers: domain.AnswersOf(form, Request{Worktrees: synced, Descendants: new(false), Push: true}), Code: CodePush, Message: message})
	}
	return followUps
}

// descendantsAnswer is the plan's answer on the descendants as a request spells it: none for the default, yes.
func (p Plan) descendantsAnswer() *bool {
	if p.Descendants {
		return nil
	}
	return new(false)
}

// unpushed reports whether origin has the branch of the worktree name and the run leaves it differing from it: rebased, or ahead of origin already.
func (p Plan) unpushed(name string) bool {
	return slices.ContainsFunc(p.Worktrees, func(r Rebase) bool {
		return r.Worktree == name && r.Origin != nil && (slices.Contains(r.Phases, domain.PhaseRebase) || r.Origin.Ahead > 0)
	})
}
