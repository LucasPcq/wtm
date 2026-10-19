package create

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what create decides from, as Observe read them: the project's config, its run.toml, the repository's snapshot and what lies at the path each branch asked would take.
type Facts struct {
	worktree.Repository
	Folders map[string]worktree.Location // keyed by the path a new worktree would take
}

func (f Facts) pathsOf(branches []string) []string {
	var paths []string
	for _, name := range branches {
		if !worktree.BlankBranch(name) {
			paths = append(paths, f.PathOf(name))
		}
	}

	return paths
}

// ----------------------------------------------------------------------------
// Each branch's decision

//go-sumtype:decl decision

// A decision is what becomes of one branch asked: its BranchPlan, or the refusal that stops it.
type decision interface{ decision() }

// A refusal is why one branch is refused, as a field error of that branch.
type refusal domain.FieldError

func (BranchPlan) decision() {}
func (refusal) decision()    {}

func (r refusal) field(index int) domain.FieldError {
	problem := domain.FieldError(r)
	problem.Path = branchPath(index)
	return problem
}

// An entry is one branch asked, with what deciding it needs of the rest of the request.
type entry struct {
	name        string
	path        string
	earlier     []string // the branches asked before it, against which it must be distinct
	ifNotExists bool
}

func (f Facts) decisions(req Request) []decision {
	decisions := make([]decision, 0, len(req.Branches))
	for index, name := range req.Branches {
		decisions = append(decisions, f.decide(entry{name: name, path: f.PathOf(name), earlier: req.Branches[:index], ifNotExists: req.IfNotExists}))
	}

	return decisions
}

// decide places one branch; it panics when Observe was not given that branch, a bug of the caller.
func (f Facts) decide(e entry) decision {
	if refused, found := f.invalid(e); found {
		return refused
	}

	at := f.CheckedOutAt(e.name)
	location, located := f.Folders[e.path]
	if !located && at == "" {
		panic(fmt.Sprintf("create: the folder of %s was never located: observe with the branches of the request", e.name))
	}

	return e.placed(worktree.Place(worktree.PlacementParams{
		Branch:       e.name,
		Path:         e.path,
		CheckedOutAt: at,
		Location:     location,
		Occupant:     f.Occupant(location),
		Local:        f.Local(e.name),
		Remote:       f.Remote(e.name),
		Live:         f.NamedLive(),
		IfNotExists:  e.ifNotExists,
	}))
}

// invalid is the refusal of a branch that the request itself rules out, whatever the repository holds: blank, given twice, not a valid branch name, or sharing its path or derived name with a branch asked before it.
func (f Facts) invalid(e entry) (refusal, bool) {
	if worktree.BlankBranch(e.name) {
		return refusal(domain.Required("")), true
	}

	if slices.Contains(e.earlier, e.name) {
		return refusal{Code: domain.CodeDistinct, Template: "{path}: {value} is given twice", Params: domain.Params{"value": e.name}}, true
	}

	if err := worktree.BranchProblem(e.name); err != nil {
		return refusal{Code: "create.branch_name_invalid", Template: "{problem}", Params: domain.Params{"problem": err.Error()}}, true
	}

	if at := slices.IndexFunc(e.earlier, func(earlier string) bool { return f.PathOf(earlier) == e.path }); at >= 0 {
		return e.clashed(worktree.Clash{Branch: e.earlier[at], Name: filepath.Base(e.path)}), true
	}

	if clash, found := f.clash(e.name, e.earlier); found {
		return e.clashed(clash), true
	}

	return refusal{}, false
}

func (f Facts) clash(branch string, live []string) (worktree.Clash, bool) {
	if !f.NamesWorktrees() {
		return worktree.Clash{}, false
	}

	return worktree.ClashWith(branch, live)
}

// placed turns where the domain places the branch into create's decision and refusals.
func (e entry) placed(placement worktree.Placement) decision {
	switch p := placement.(type) {
	case worktree.Unchanged:
		return BranchPlan{Branch: e.name, Path: p.Path, Action: ActionUnchanged}
	case worktree.CheckedOutElsewhere:
		return refusal{
			Code:     "create.checked_out_elsewhere",
			Template: "branch {branch} is checked out at {path} — run `wtm go {branch}` to jump in",
			Params:   domain.Params{"branch": e.name, "path": p.Path},
		}
	case worktree.PathTaken:
		return refusal{Code: "create.path_exists", Template: "worktree path already exists: {path}", Params: domain.Params{"path": e.path}}
	case worktree.Clash:
		return e.clashed(p)
	case worktree.Placed:
		return BranchPlan{Branch: e.name, Path: e.path, Action: Action(p.Start), Upstream: p.Upstream}
	}

	return nil
}

func (e entry) clashed(clash worktree.Clash) refusal {
	return refusal{
		Code:     "create.name_clash",
		Template: "branch {branch} would share its name with {other} ({name}) — pick another name",
		Params:   domain.Params{"branch": e.name, "other": clash.Branch, "name": clash.Name},
	}
}

// checksOut reports whether the request checks out a branch that already exists locally, whose parent then cannot be inferred.
func (f Facts) checksOut(req Request) bool {
	return slices.ContainsFunc(f.decisions(req), func(d decision) bool {
		planned, isPlanned := d.(BranchPlan)
		return isPlanned && planned.Action == ActionCheckOut
	})
}

// subjectsAt is each local branch a planned worktree starts from whose standing against origin is standing, each once.
func (f Facts) subjectsAt(req Request, standing worktree.Divergence) []string {
	var subjects []string
	for _, d := range f.decisions(req) {
		planned, isPlanned := d.(BranchPlan)
		if !isPlanned {
			continue
		}

		subject := f.StartsFrom(planned.Branch, planned.Action.start(), req.From)
		if subject != "" && f.Standing(subject) == standing && !slices.Contains(subjects, subject) {
			subjects = append(subjects, subject)
		}
	}

	return subjects
}

// ----------------------------------------------------------------------------
// Decide

// Decide turns the facts and the request Resolve settled on them into the plan and its warnings. Resolve refuses every branch the facts rule out, so Decide refuses none: it panics on a request not resolved on these facts, a bug of the caller.
func Decide(f Facts, req Request) (Plan, []domain.Warning) {
	branches := make([]BranchPlan, 0, len(req.Branches))
	for _, d := range f.decisions(req) {
		planned, isPlanned := d.(BranchPlan)
		if !isPlanned {
			panic(fmt.Sprintf("create: decide %v: the request was not resolved on these facts", d))
		}
		branches = append(branches, planned)
	}

	answers := f.newWorktree(req).Answers()
	planned := Plan{
		Branches:    branches,
		From:        req.From,
		EnvFrom:     answers.EnvFrom(),
		Isolation:   f.Run.PlannedIsolation(answers.Isolation()),
		FastForward: f.fastForwarded(req),
		Remember:    req.Remember,
		facts:       f,
		settled:     answers,
	}
	return planned, append(f.divergedWarnings(req), f.fallbackWarnings(planned)...)
}

func (f Facts) fastForwarded(req Request) []string {
	if req.FastForward == nil || !*req.FastForward {
		return nil
	}

	return f.subjectsAt(req, worktree.Behind)
}

func (f Facts) divergedWarnings(req Request) []domain.Warning {
	var warnings []domain.Warning
	for _, subject := range f.subjectsAt(req, worktree.Diverged) {
		counts := f.AheadBehind(subject)
		warnings = append(warnings, domain.Warning{
			Code:    "create.source_diverged",
			Message: "branch " + subject + " has diverged from origin (" + counts.String() + ") and can't be fast-forwarded: the worktree starts from your local branch, missing commits that are on origin — you may have to rebase or resolve conflicts later",
			Params:  domain.Params{"branch": subject, "ahead": strconv.Itoa(counts.Ahead), "behind": strconv.Itoa(counts.Behind)},
		})
	}

	return warnings
}

func (f Facts) fallbackWarnings(planned Plan) []domain.Warning {
	creates := slices.ContainsFunc(planned.Branches, func(b BranchPlan) bool { return b.Action != ActionUnchanged })
	fallsBack := env.ParentFallsBackToMain(env.ParentFallbackParams{
		Strategy:          planned.EnvFrom,
		HasFiles:          len(f.Config.Env.Files) > 0,
		SourceHasWorktree: f.CheckedOutAt(planned.From) != "",
	})
	if !creates || !fallsBack {
		return nil
	}

	return []domain.Warning{{Code: "create.env_parent_fallback", Message: `The "parent" env strategy needs the source branch checked out to copy its .env; without a worktree it comes from main.`}}
}

// ----------------------------------------------------------------------------
// The plan

// An Action is how a branch gets its worktree.
type Action string

// The actions of a branch's plan.
const (
	ActionCreate    Action = "create"    // a new branch, from the plan's source
	ActionCheckOut  Action = "check_out" // a branch that exists locally, as it is
	ActionResume    Action = "resume"    // a branch only origin has, tracking it
	ActionUnchanged Action = "unchanged" // its worktree is already open (--if-not-exists)
)

// start is how the worktree of the action begins; "" for an action that makes none.
func (a Action) start() worktree.Start {
	switch a {
	case ActionCreate:
		return worktree.StartCreate
	case ActionCheckOut:
		return worktree.StartCheckOut
	case ActionResume:
		return worktree.StartResume
	case ActionUnchanged:
		return ""
	}

	panic("create: an action with no start: " + string(a))
}

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: each branch's worktree, the source, the env strategy, the isolation, the sources to fast-forward first and the answers to remember; then the answers recalled and the recap, which the fingerprint leaves out since they change nothing the apply does.
type Plan struct {
	Branches    []BranchPlan        `json:"branches"`
	From        string              `json:"from,omitempty"`
	EnvFrom     project.EnvStrategy `json:"env_from"`
	Isolation   project.Isolation   `json:"isolation,omitempty"`
	FastForward []string            `json:"fast_forward,omitempty"`
	Remember    []string            `json:"remember,omitempty"`
	Creates     int                 `json:"creates,omitempty"`  // the worktrees the apply opens, one already open aside; out of the fingerprint, which the actions cover
	Recalled    []string            `json:"recalled,omitempty"` // the fields answered from memory, as the request was resolved
	Recap       []domain.RecapRow   `json:"recap,omitempty"`    // the plan as every surface shows it, its answers marked as the run left them
	facts       Facts
	settled     provision.Answers // as resolved, handed to provisioning even where the plan shows none, and remembered
}

// A BranchPlan is what becomes of one branch: the path of its worktree, how it gets there, and the remote branch it resumes.
type BranchPlan struct {
	Branch   string `json:"branch"`
	Path     string `json:"path"`
	Action   Action `json:"action"`
	Upstream string `json:"upstream,omitempty"`
}

// Locks are the locks the plan needs: the repository shared, and each worktree it makes or fast-forwards exclusive.
func (p Plan) Locks() []domain.Lock {
	names := make([]string, 0, len(p.Branches)+len(p.FastForward))
	for _, planned := range p.Branches {
		names = append(names, planned.Branch)
	}

	return domain.WorktreeLocks(append(names, p.FastForward...))
}

// Fingerprinted is what the plan's fingerprint covers: the plan with what the apply reads of the project to provision each worktree.
func (p Plan) Fingerprinted() any {
	p.Recalled, p.Recap, p.Creates = nil, nil, 0
	return provision.Fingerprinted(p, p.facts.Repository, p.settled)
}

// opens counts the worktrees the plan opens, a worktree already open aside.
func (p Plan) opens() int {
	count := 0
	for _, planned := range p.Branches {
		if planned.Action != ActionUnchanged {
			count++
		}
	}
	return count
}

// Locks are the locks taken before planning: the repository shared, and each branch asked exclusive.
func (r Request) Locks() []domain.Lock {
	return domain.WorktreeLocks(r.Branches)
}

// placement is the branch's action as the domain places it, for its checkout and its standing.
func (b BranchPlan) placement() worktree.Placed {
	return worktree.Placed{Start: b.Action.start(), Upstream: b.Upstream}
}

// ----------------------------------------------------------------------------
// The recap

// NoteAlreadyOpen notes a branch of the recap whose worktree is open already; the others are worktree's notes.
const NoteAlreadyOpen = "already_open"

// appliedRecap is the recap of the plan once applied, the fields remembered by the apply marked saved; Run sets it as the applied plan's Recap.
func (p Plan) appliedRecap(remembered []string) []domain.RecapRow {
	return p.recap(domain.Marks{Recalled: p.Recalled, Remembered: remembered, Mark: domain.MarkSaved})
}

func (p Plan) previewRecap() []domain.RecapRow {
	return p.recap(domain.Marks{Recalled: p.Recalled, Remembered: p.Remember, Mark: domain.MarkRemember})
}

// recap is the plan row by row: the branches with how each gets its worktree, the source, From when a branch is created from it, else Parent, the branch recorded for wtm sync, then the answers of a new worktree.
func (p Plan) recap(marks domain.Marks) []domain.RecapRow {
	source := domain.RecapRow{Key: "parent", Label: "Parent", Path: pathFrom, Values: domain.RecapValues([]string{p.From})}
	if slices.ContainsFunc(p.Branches, func(b BranchPlan) bool { return b.Action == ActionCreate }) {
		source.Key, source.Label = "from", "From"
	}

	rows := []domain.RecapRow{{Key: pathBranches, Label: "Branches", Path: pathBranches, Values: recappedBranches(p.Branches)}, source}
	return append(rows, provision.RecapRows(provision.RecapParams{EnvFrom: p.EnvFrom, FollowsConfig: p.settled.FollowsConfig(), Isolation: p.Isolation, FastForward: p.FastForward, Marks: marks})...)
}

func recappedBranches(branches []BranchPlan) []domain.RecapValue {
	values := make([]domain.RecapValue, 0, len(branches))
	for _, planned := range branches {
		value := domain.RecapValue{Value: planned.Branch, Text: planned.Branch}
		switch planned.Action {
		case ActionCheckOut:
			value.Note, value.Text = worktree.NoteExisting, planned.Branch+" (existing)"
		case ActionResume:
			value.Note, value.Params, value.Text = worktree.NoteResumes, domain.Params{"upstream": planned.Upstream}, worktree.Resumes(planned.Branch, planned.Upstream)
		case ActionUnchanged:
			value.Note, value.Text = NoteAlreadyOpen, planned.Branch+" (already open)"
		case ActionCreate:
		}
		values = append(values, value)
	}

	return values
}
