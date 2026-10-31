package fastforward

import (
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what fast-forward decides from, as Observe read them: the project's config and the repository's snapshot, origin's branches as last fetched, where each name asked as a path leads, what each worktree on a branch holds, and why origin could not be fetched.
type Facts struct {
	worktree.Repository
	Located  map[string]string // the worktree root each name asked that is no worktree's name leads to, when it leads to one
	Checks   map[string]Check  // by worktree name
	FetchErr error             // why origin could not be fetched: the plan stands on its branches as last fetched
}

// A Check is whether a worktree's directory is gone, and how many uncommitted changes it holds.
type Check struct {
	Missing bool
	Changes int
}

// Dirty reports whether the worktree holds uncommitted changes.
func (c Check) Dirty() bool {
	return c.Changes > 0
}

// onBranch are the worktrees fast-forward may move, the main one first: those on a branch, a rebase or a bisect under way included.
func (f Facts) onBranch() []worktree.Worktree {
	return slices.DeleteFunc(f.MainFirst(), func(w worktree.Worktree) bool { return w.Branch() == "" })
}

// asked are the worktrees req asks, each once, those on a branch when it names none or asks for every one.
func (f Facts) asked(req Request) []worktree.Worktree {
	if len(req.Worktrees) == 0 || req.All {
		return f.onBranch()
	}

	var found []worktree.Worktree
	for _, name := range req.Worktrees {
		if w, ok := f.subject(name); ok && w.Branch() != "" && !slices.Contains(found, w) {
			found = append(found, w)
		}
	}
	return found
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

// picked are the worktrees picked by default, as origin was last fetched, the main one first: those behind it, but one with uncommitted changes, which the command keeps when it chose it.
func (f Facts) picked() []string {
	var names []string
	for _, w := range f.onBranch() {
		if check := f.Checks[w.Name()]; f.standing(w, check) == StandingBehind && !check.Dirty() {
			names = append(names, w.Name())
		}
	}

	return names
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

// A Standing is where a worktree stands for a fast-forward, the first that applies: its directory gone, an operation git stopped in the middle of (a rebase, a bisect, a merge…), no branch of its name on origin, or its branch diverged from origin's, ahead of it, up to date, or behind it, the only standing that moves.
type Standing string

// The standings of a worktree.
const (
	StandingMissing  Standing = "missing"
	StandingBusy     Standing = "busy"
	StandingNoOrigin Standing = "no_origin"
	StandingDiverged Standing = "diverged"
	StandingAhead    Standing = "ahead"
	StandingUpToDate Standing = "up_to_date"
	StandingBehind   Standing = "behind"
)

// Standings returns every Standing.
func Standings() []Standing {
	return []Standing{StandingMissing, StandingBusy, StandingNoOrigin, StandingDiverged, StandingAhead, StandingUpToDate, StandingBehind}
}

// A Blocker is what makes a worktree behind origin unsafe to fast-forward: a safety refusal, which only Force lifts.
type Blocker string

// BlockerDirty is uncommitted changes, which the merge may overwrite or carry over.
const BlockerDirty Blocker = "dirty"

// Blockers returns every Blocker.
func Blockers() []Blocker {
	return []Blocker{BlockerDirty}
}

// An Advance is one worktree asked: its name, its path, where it stands, the operation git stopped in the middle of in it when it is busy, its branch against origin's (null when origin has none), the commit it stands at and the one origin's stands at when it is behind, what makes it unsafe, and the phases of its fast-forward, none when the plan leaves it as it is.
type Advance struct {
	Worktree  string             `json:"worktree"`
	Path      string             `json:"path"`
	Standing  Standing           `json:"standing"`
	Operation worktree.Operation `json:"operation,omitempty"`
	Origin    *worktree.Origin   `json:"origin"`
	From      string             `json:"from"`
	To        string             `json:"to,omitempty"`
	Blockers  []Blocker          `json:"blockers"`
	Phases    []domain.Phase     `json:"phases"`
	entry     int                // its index among the worktrees asked
}

// moves reports whether the plan fast-forwards the worktree.
func (a Advance) moves() bool {
	return len(a.Phases) > 0
}

func (f Facts) advance(w worktree.Worktree, entry int) Advance {
	check := f.Checks[w.Name()]
	advance := Advance{Worktree: w.Name(), Path: w.Path, Standing: f.standing(w, check), Operation: w.Operation(), Origin: f.OriginOf(w.Branch()), From: f.Tip(w.Branch()), Blockers: []Blocker{}, Phases: []domain.Phase{}, entry: entry}
	if advance.Standing != StandingBehind {
		return advance
	}

	advance.To = f.remoteTip(w.Branch())
	if check.Dirty() {
		advance.Blockers = append(advance.Blockers, BlockerDirty)
	}
	return advance
}

func (f Facts) standing(w worktree.Worktree, check Check) Standing {
	switch {
	case w.Prunable || check.Missing:
		return StandingMissing
	case w.Operation() != "":
		return StandingBusy
	}

	switch f.Standing(w.Branch()) {
	case worktree.NoOrigin:
		return StandingNoOrigin
	case worktree.Diverged:
		return StandingDiverged
	case worktree.Ahead:
		return StandingAhead
	case worktree.UpToDate:
		return StandingUpToDate
	}
	return StandingBehind
}

// unsafe are the advances a blocker refuses unless forced.
func unsafe(advances []Advance) []Advance {
	return slices.DeleteFunc(slices.Clone(advances), func(a Advance) bool { return len(a.Blockers) == 0 })
}

// ----------------------------------------------------------------------------
// Decide

// Decide turns the facts and the resolved request into the plan and its warnings: each worktree behind origin moves, the others stay as they are. An unsafe worktree, unforced, refuses the whole request with CodeUnsafe when the request named it, and is kept as it is, its blockers and no phase, when the command chose it.
func Decide(f Facts, req Request) (Plan, []domain.Warning, error) {
	advances, _ := f.entries(req)
	if blocked := unsafe(advances); len(blocked) > 0 && !req.Force && req.named() {
		return Plan{}, nil, refusal(blocked, req)
	}

	for i, a := range advances {
		if a.Standing == StandingBehind && (len(a.Blockers) == 0 || req.Force) {
			advances[i].Phases = []domain.Phase{domain.PhaseFastForward}
		}
	}
	if advances == nil {
		advances = []Advance{}
	}

	planned := Plan{Worktrees: advances, Force: req.Force}
	return planned, f.warnings(planned), nil
}

// CodeUnsafe refuses a request that names a worktree a blocker makes unsafe; CodeForce is its follow-up, the same request forced.
const (
	CodeUnsafe = Name + ".unsafe"
	CodeForce  = Name + ".force"
)

// refusal refuses the request with CodeUnsafe: a problem per blocker of each unsafe worktree, on its entry, coded fast-forward.<blocker>, followed by the request forced.
func refusal(blocked []Advance, req Request) *domain.Error {
	var fields []domain.FieldError
	names := make([]string, 0, len(blocked))
	for _, a := range blocked {
		names = append(names, a.Worktree)
		for _, blocker := range a.Blockers {
			fields = append(fields, domain.FieldError{Path: entryPath(a.entry), Code: Name + "." + string(blocker), Template: "{value} has " + blockerWords[blocker], Params: domain.Params{"value": a.Worktree}})
		}
	}

	forced := Request{Worktrees: req.Worktrees, Force: true}
	return &domain.Error{
		Kind:     domain.KindRefused,
		Code:     CodeUnsafe,
		Message:  "nothing fast-forwarded: " + strings.Join(names, ", ") + " can't be fast-forwarded safely",
		Params:   domain.Params{"worktrees": strings.Join(names, ", ")},
		Fields:   fields,
		FollowUp: &domain.FollowUp{Command: Name, Answers: domain.AnswersOf(Questions{}.Form(), forced), Code: CodeForce, Message: "fast-forward anyway"},
	}
}

// blockerWords say what each blocker is, after "has".
var blockerWords = map[Blocker]string{BlockerDirty: "uncommitted changes"}

// CodeDirty is the problem of a named worktree with uncommitted changes, and the failure of one that gained them since the plan; CodeKept warns of a worktree the command chose and kept for its blockers.
const (
	CodeDirty = Name + "." + string(BlockerDirty)
	CodeKept  = Name + ".kept"
)

// The codes of the warnings of a worktree left as it is, and of a fetch that failed: each says why it is left, followed by what moves it on.
const (
	CodeDiverged    = Name + ".diverged"
	CodeNoOrigin    = Name + ".no_origin"
	CodeBusy        = Name + ".busy"
	CodeMissing     = Name + ".missing"
	CodeFetchFailed = Name + ".fetch_failed"
	CodeGo          = Name + ".go"
	CodeClean       = Name + ".clean"
)

// warnings are what the plan cannot see to or leaves as it is: the fetch that failed, each worktree left, by why, with the command that moves it on, then each worktree kept (kept).
func (f Facts) warnings(planned Plan) []domain.Warning {
	var warnings []domain.Warning
	if f.FetchErr != nil {
		warnings = append(warnings, domain.Warning{Code: CodeFetchFailed, Message: "couldn't fetch origin: where each branch stands against it is as last fetched: " + f.FetchErr.Error(), Params: domain.Params{"cause": f.FetchErr.Error()}})
	}

	for _, a := range planned.Worktrees {
		if warning, left := leftWarning(a); left && !a.moves() {
			warnings = append(warnings, warning)
		}
	}

	return append(warnings, kept(planned.Worktrees)...)
}

// kept warns of each worktree the command chose and kept for its blockers, the last followed by the one request that forces every one of them.
func kept(advances []Advance) []domain.Warning {
	var warnings []domain.Warning
	var names []string
	for _, a := range advances {
		if a.moves() || len(a.Blockers) == 0 {
			continue
		}
		reasons := make([]string, 0, len(a.Blockers))
		for _, blocker := range a.Blockers {
			reasons = append(reasons, blockerWords[blocker])
		}
		names = append(names, a.Worktree)
		warnings = append(warnings, domain.Warning{Code: CodeKept, Message: a.Worktree + " is kept as it is: " + strings.Join(reasons, ", "), Params: domain.Params{"worktree": a.Worktree, "blockers": strings.Join(domain.Names(a.Blockers), ", ")}})
	}
	if len(warnings) == 0 {
		return nil
	}

	forced := domain.AnswersOf(Questions{}.Form(), Request{Worktrees: names, Force: true})
	warnings[len(warnings)-1].FollowUp = &domain.FollowUp{Command: Name, Answers: forced, Code: CodeForce, Message: "fast-forward them anyway"}
	return warnings
}

// leftWarning is the warning of a worktree the plan leaves as it is, and whether it has one: up to date or ahead, it has nothing to say, and one kept behind origin is warned of by kept.
func leftWarning(a Advance) (domain.Warning, bool) {
	params := domain.Params{"worktree": a.Worktree}
	on := []domain.Answered{{Path: domain.PathWorktree, Values: []string{a.Worktree}}}
	switch a.Standing {
	case StandingDiverged:
		params["ahead"], params["behind"] = strconv.Itoa(a.Origin.Ahead), strconv.Itoa(a.Origin.Behind)
		// sync rebases onto the parent, never onto origin: only a merge or a rebase by hand reconciles the two
		enter := domain.Enter(a.Worktree)
		enter.Code, enter.Message = CodeGo, "reconcile it with origin by hand"
		return domain.Warning{Code: CodeDiverged, Message: a.Worktree + " has diverged from origin (" + params["ahead"] + " ahead, " + params["behind"] + " behind): left as it is", Params: params, FollowUp: &enter}, true
	case StandingNoOrigin:
		return domain.Warning{Code: CodeNoOrigin, Message: "origin has no branch " + a.Worktree + ": nothing to fast-forward it to", Params: params}, true
	case StandingBusy:
		enter := domain.Enter(a.Worktree)
		enter.Code, enter.Message, params["operation"] = CodeGo, "finish it in the worktree", string(a.Operation)
		return domain.Warning{Code: CodeBusy, Message: a.Worktree + " is in the middle of an operation (" + string(a.Operation) + "): left as it is", Params: params, FollowUp: &enter}, true
	case StandingMissing:
		return domain.Warning{Code: CodeMissing, Message: a.Worktree + "'s directory is gone: left as it is", Params: params, FollowUp: &domain.FollowUp{Command: domain.CommandClean, Answers: on, Code: CodeClean, Message: "remove what is left of it"}}, true
	case StandingBehind, StandingAhead, StandingUpToDate:
	}

	return domain.Warning{}, false
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: every worktree asked, those that move and those left as they are, and whether the unsafe ones are forced; then the recap, which the fingerprint leaves out.
type Plan struct {
	Worktrees []Advance `json:"worktrees"`
	Force     bool      `json:"force"`
	domain.Recapped
}

// Locks are the locks the plan needs: the repository shared, and each worktree it moves exclusive.
func (p Plan) Locks() []domain.Lock {
	var names []string
	for _, a := range p.Worktrees {
		if a.moves() {
			names = append(names, a.Worktree)
		}
	}

	return domain.WorktreeLocks(names)
}

// Fingerprinted is what the plan's fingerprint covers: the plan, the commits each worktree moves from and to among it, its recap left out.
func (p Plan) Fingerprinted() any {
	p.Recapped = domain.Recapped{}
	return p
}

// Locks are the locks taken before planning: the repository shared, and each worktree named exclusive; every worktree's are taken once the plan names those that move.
func (r Request) Locks() []domain.Lock {
	return domain.WorktreeLocks(slices.DeleteFunc(slices.Clone(r.Worktrees), func(name string) bool { return strings.TrimSpace(name) == "" }))
}

// ----------------------------------------------------------------------------
// The recap

// The notes of the recap's values: a worktree fast-forwarded although it has uncommitted changes, one ahead of origin, and why one is left as it is, its standing or its blocker.
const (
	NoteForced = "forced"
	NoteAhead  = "ahead"
)

// empty reports whether applying the plan changes nothing: no worktree the plan holds moves.
func (p Plan) empty() bool {
	return !slices.ContainsFunc(p.Worktrees, Advance.moves)
}

// recap is the plan row by row: the worktrees fast-forwarded with the commits each takes, the forced ones said so, those already up to date or ahead, then those left as they are, each with why.
func (p Plan) recap() []domain.RecapRow {
	var moved, current, left []domain.RecapValue
	for _, a := range p.Worktrees {
		switch {
		case a.moves():
			moved = append(moved, movedValue(a))
		case a.Standing == StandingUpToDate:
			current = append(current, domain.RecapValue{Value: a.Worktree, Text: a.Worktree})
		case a.Standing == StandingAhead:
			current = append(current, domain.RecapValue{Value: a.Worktree, Note: NoteAhead, Text: a.Worktree + " (ahead of origin)"})
		default:
			left = append(left, leftValue(a))
		}
	}

	var rows []domain.RecapRow
	if len(moved) > 0 {
		rows = append(rows, domain.RecapRow{Key: "fast_forward", Label: "Fast-forward", Path: domain.PathWorktree, Values: moved})
	}
	if len(current) > 0 {
		rows = append(rows, domain.RecapRow{Key: "up_to_date", Label: "Up to date", Path: domain.PathWorktree, Values: current})
	}
	if len(left) > 0 {
		rows = append(rows, domain.RecapRow{Key: "left", Label: "Left as is", Path: domain.PathWorktree, Values: left})
	}
	return rows
}

func movedValue(a Advance) domain.RecapValue {
	value := domain.RecapValue{Value: a.Worktree, Params: domain.Params{"commits": strconv.Itoa(a.Origin.Behind)}, Text: a.Worktree + " (" + commits(a.Origin.Behind) + ")"}
	if len(a.Blockers) > 0 {
		value.Note, value.Text = NoteForced, a.Worktree+" ("+commits(a.Origin.Behind)+"; uncommitted changes, forced)"
	}
	return value
}

var leftWords = map[Standing]string{
	StandingBusy:     "an operation is under way",
	StandingMissing:  "its directory is gone",
	StandingNoOrigin: "not on origin",
	StandingDiverged: "diverged from origin",
	StandingBehind:   "uncommitted changes",
}

func leftValue(a Advance) domain.RecapValue {
	note := string(a.Standing)
	if a.Standing == StandingBehind {
		note = string(BlockerDirty)
	}
	return domain.RecapValue{Value: a.Worktree, Note: note, Text: a.Worktree + " (" + leftWords[a.Standing] + ")"}
}

func commits(n int) string {
	if n == 1 {
		return "1 commit"
	}
	return strconv.Itoa(n) + " commits"
}

// ----------------------------------------------------------------------------
// The outcome

// CodeFailed is a worktree git refused to move; one that gained uncommitted changes or an operation since the plan fails with CodeDirty or CodeBusy.
const CodeFailed = Name + ".failed"

// The messages of a worktree that failed to move, one fixed line per code, the cause git gave in its params.
const (
	messageFailed = "git refused to move the branch"
	messageDirty  = "the worktree has uncommitted changes"
	messageBusy   = "the worktree is in the middle of an operation"
)
