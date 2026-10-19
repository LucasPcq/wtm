package extract

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what extract decides from, as Observe read them: the project's config, its run.toml, the repository's snapshot, the uncommitted changes of the source, the meta records of the source and target, and what lies at the path a new target would take.
type Facts struct {
	worktree.Repository
	Changes map[string][]worktree.Change // the uncommitted changes of each worktree read, by branch
	Records map[string]worktree.Meta     // the meta record of the source and of an existing target, by branch
	Folder  worktree.Location            // what lies at the path a new target would take
}

// sources are the worktrees on a branch whose changes can be extracted: the one of source, or all of them when source is empty.
func (f Facts) sources(source string) []worktree.Worktree {
	var found []worktree.Worktree
	for _, w := range f.Worktrees {
		if w.Branch() != "" && (source == "" || w.Branch() == source) {
			found = append(found, w)
		}
	}

	return found
}

func (f Facts) checkedOut(names ...string) []string {
	return slices.DeleteFunc(names, func(name string) bool { return name == "" || f.CheckedOutAt(name) == "" })
}

// newPath is where the worktree of to would be created, or "" when to has a worktree already or names no branch.
func (f Facts) newPath(to string) string {
	if worktree.BlankBranch(to) || f.CheckedOutAt(to) != "" {
		return ""
	}

	return f.PathOf(to)
}

// nothing reports whether the source has a worktree with no uncommitted change, leaving nothing to extract.
func (f Facts) nothing(source string) bool {
	return f.CheckedOutAt(source) != "" && len(f.Changes[source]) == 0
}

func (f Facts) selected(req Request) []worktree.Change {
	selected, _ := worktree.SelectChanges(f.Changes[req.Source], req.Files)
	return selected
}

// parent is the branch the source's meta record names as its parent, "" when none.
func (f Facts) parent(source string) string {
	return f.Records[source].SourceBranch
}

func (f Facts) recordedIsolation(name string) project.Isolation {
	return f.Records[name].Isolation.Effective()
}

// ----------------------------------------------------------------------------
// The target

// An Action is how the target gets its worktree.
type Action string

// The actions of the target.
const (
	ActionExisting Action = "existing"  // its worktree exists: the changes go there
	ActionCreate   Action = "create"    // a new branch, from the plan's source
	ActionCheckOut Action = "check_out" // a branch that exists locally, as it is
	ActionResume   Action = "resume"    // a branch only origin has, tracking it
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
	case ActionExisting:
		return ""
	}

	panic("extract: an action with no start: " + string(a))
}

// A Target is the worktree the changes go to: its branch, its path, how it gets there, and the remote branch it resumes.
type Target struct {
	Branch   string `json:"branch"`
	Path     string `json:"path"`
	Action   Action `json:"action"`
	Upstream string `json:"upstream,omitempty"`
}

func (t Target) creates() bool {
	return t.Action != ActionExisting
}

// placement is the target's action as the domain places it, for its checkout and its standing.
func (t Target) placement() worktree.Placed {
	return worktree.Placed{Start: t.Action.start(), Upstream: t.Upstream}
}

//go-sumtype:decl decision

// A decision is what becomes of the target asked: its Target, or the refusal that stops the request.
type decision interface{ decision() }

// A refusal is why the target is refused, as a field error of --to.
type refusal domain.FieldError

func (Target) decision()  {}
func (refusal) decision() {}

func (r refusal) field(path string) domain.FieldError {
	problem := domain.FieldError(r)
	problem.Path = path
	return problem
}

// target decides the target: the worktree of to when it has one, else a worktree placed for it as create would place it.
func (f Facts) target(req Request) decision {
	to := req.To
	switch {
	case worktree.BlankBranch(to):
		return refusal(domain.Required(""))
	case to == req.Source:
		return refusal{Code: "extract.same_worktree", Template: "{path}: {value} is the source — pick another worktree", Params: domain.Params{"value": to}}
	}

	if at := f.CheckedOutAt(to); at != "" {
		return Target{Branch: to, Path: at, Action: ActionExisting}
	}

	if err := worktree.BranchProblem(to); err != nil {
		return refusal{Code: "extract.branch_name_invalid", Template: "{problem}", Params: domain.Params{"problem": err.Error()}}
	}

	created := Target{Branch: to, Path: f.PathOf(to)}
	return created.placed(worktree.Place(worktree.PlacementParams{
		Branch:   to,
		Path:     created.Path,
		Location: f.Folder,
		Occupant: f.Occupant(f.Folder),
		Local:    f.Local(to),
		Remote:   f.Remote(to),
		Live:     f.NamedLive(),
	}))
}

// placed turns where the domain places the target into extract's decision. The target is placed only when its branch has no worktree, and with no IfNotExists, so the domain never answers Unchanged nor CheckedOutElsewhere.
func (t Target) placed(placement worktree.Placement) decision {
	switch p := placement.(type) {
	case worktree.PathTaken:
		return refusal{Code: "extract.path_exists", Template: "worktree path already exists: {path}", Params: domain.Params{"path": t.Path}}
	case worktree.Clash:
		return refusal{
			Code:     "extract.name_clash",
			Template: "branch {branch} would share its name with {other} ({name}) — pick another name",
			Params:   domain.Params{"branch": t.Branch, "other": p.Branch, "name": p.Name},
		}
	case worktree.Placed:
		return Target{Branch: t.Branch, Path: t.Path, Action: Action(p.Start), Upstream: p.Upstream}
	case worktree.Unchanged, worktree.CheckedOutElsewhere:
	}

	panic(fmt.Sprintf("extract: %s placed as %T: a branch with a worktree is never placed", t.Branch, placement))
}

func (f Facts) planned(req Request) (Target, bool) {
	target, ok := f.target(req).(Target)
	return target, ok
}

// existing reports whether the target is a worktree that exists, which makes the questions about creating one moot.
func (f Facts) existing(req Request) bool {
	target, ok := f.planned(req)
	return ok && !target.creates()
}

// subjectAt is the local branch a created target starts from when its standing against origin is standing, else "".
func (f Facts) subjectAt(req Request, standing worktree.Divergence) string {
	target, ok := f.planned(req)
	if !ok {
		return ""
	}

	subject := f.StartsFrom(target.Branch, target.Action.start(), req.From)
	if subject == "" || f.Standing(subject) != standing {
		return ""
	}

	return subject
}

// conflictCheck asks which selected changes would conflict in the target: in its worktree when it exists, else in the tree of the ref it will start from, as origin has it when the apply fast-forwards that ref first.
func (f Facts) conflictCheck(req Request) worktree.ConflictCheck {
	check := worktree.ConflictCheck{Source: f.CheckedOutAt(req.Source), Changes: f.selected(req)}
	target, _ := f.planned(req)
	switch target.Action {
	case ActionExisting:
		check.Target = target.Path
	case ActionCheckOut:
		check.TargetRef = target.Branch
	case ActionResume:
		check.TargetRef = target.Upstream
	case ActionCreate:
		check.TargetRef = req.From
	}

	if slices.Contains(f.fastForwarded(req), check.TargetRef) {
		check.TargetRef = worktree.OriginPrefix + check.TargetRef
	}

	return check
}

// ----------------------------------------------------------------------------
// Decide

// Decide turns the facts, the request Resolve settled on them and the conflicts the changes would meet into the plan and its warnings. A source with nothing to extract is a plan with no files. Resolve refuses a target the facts rule out, so Decide refuses only conflicts, with CodeConflict, unless the request resolves them, and binary ones always; it panics on a request not resolved on these facts, a bug of the caller.
func Decide(f Facts, req Request, conflicts []worktree.Conflict) (Plan, []domain.Warning, error) {
	planned := Plan{Source: req.Source, Files: []worktree.Change{}, Conflicts: []worktree.Conflict{}, facts: f}
	if f.nothing(req.Source) {
		return planned, nil, nil
	}

	target, isTarget := f.planned(req)
	if !isTarget {
		panic(fmt.Sprintf("extract: decide %s: the request was not resolved on these facts", req.To))
	}
	planned.Target, planned.Creates = target, target.creates()

	if refused := refusedFor(planned.Target.Branch, req.OnConflict, conflicts); refused != nil {
		return Plan{}, nil, refused
	}

	planned.Files = f.selected(req)
	planned.Keep = req.Keep != nil && *req.Keep
	planned.Conflicts = append(planned.Conflicts, conflicts...)
	planned.OnConflict = req.OnConflict
	if !planned.Target.creates() {
		return planned, f.ignored(req, planned.Target), nil
	}

	planned.settled = f.newWorktree(req).Answers()
	planned.From = req.From
	planned.EnvFrom = planned.settled.EnvFrom()
	planned.Isolation = f.Run.PlannedIsolation(planned.settled.Isolation())
	planned.FastForward = f.fastForwarded(req)
	planned.Remember = req.Remember
	return planned, append(f.divergedWarnings(req), f.fallbackWarnings(planned)...), nil
}

// refusedFor is the refusal of the conflicts, nil when there are none or the request resolves them; a binary conflict cannot take conflict markers, so it is refused even then.
func refusedFor(target string, onConflict OnConflict, conflicts []worktree.Conflict) *domain.Error {
	if !worktree.Refuses(conflicts, onConflict == OnConflictResolve) {
		return nil
	}

	return conflictRefusal(conflicts, target)
}

// conflictRefusal refuses the conflicts with CodeConflict, its message naming the binary files alone when there are any, else each file by how it conflicts.
func conflictRefusal(conflicts []worktree.Conflict, target string) *domain.Error {
	byKind := map[worktree.ConflictKind][]string{}
	var all []string
	for _, conflict := range conflicts {
		byKind[conflict.Kind] = append(byKind[conflict.Kind], conflict.Path)
		all = append(all, conflict.Path)
	}

	refusal := &domain.Error{Kind: domain.KindConflict, Code: CodeConflict, Params: domain.Params{"target": target, "files": strings.Join(all, ", ")}}
	if binary := byKind[worktree.ConflictBinary]; len(binary) > 0 {
		refusal.Params["files"] = strings.Join(binary, ", ")
		refusal.Message = fmt.Sprintf("cannot apply the selected changes: %s %s in %q and %s binary — conflict markers cannot be written, remove %s there or extract to another worktree",
			strings.Join(binary, ", "), plural(len(binary), "already exists", "already exist"), target, plural(len(binary), "is", "are"), plural(len(binary), "it", "them"))
		return refusal
	}

	var clauses []string
	if modified := byKind[worktree.ConflictModified]; len(modified) > 0 {
		clauses = append(clauses, strings.Join(modified, ", ")+" "+plural(len(modified), "was also modified", "were also modified"))
	}
	if exists := byKind[worktree.ConflictExists]; len(exists) > 0 {
		clauses = append(clauses, strings.Join(exists, ", ")+" "+plural(len(exists), "already exists", "already exist"))
	}

	refusal.Message = fmt.Sprintf("cannot apply the selected changes: %s in %q — resolve %s there, extract to another worktree, or re-run with --on-conflict resolve",
		strings.Join(clauses, " and "), target, plural(len(all), "it", "them"))
	return refusal
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}

	return many
}

// ignored warns of each flag given that only applies to a target created, when the target exists.
func (f Facts) ignored(req Request, target Target) []domain.Warning {
	var warnings []domain.Warning
	params := domain.Params{"branch": target.Branch}
	if req.From != "" {
		warnings = append(warnings, domain.Warning{Code: "extract.from_ignored", Message: "--from ignored: " + target.Branch + " already exists, so nothing is created", Params: params})
	}

	if req.EnvFrom != "" {
		warnings = append(warnings, domain.Warning{Code: "extract.env_from_ignored", Message: "--env-from ignored: " + target.Branch + " already exists, so nothing is created", Params: params})
	}

	if req.FastForward != nil && *req.FastForward {
		warnings = append(warnings, domain.Warning{Code: "extract.fast_forward_ignored", Message: "--ff ignored: " + target.Branch + " already exists, so nothing is created", Params: params})
	}

	if current := f.recordedIsolation(target.Branch); req.Isolation != "" && req.Isolation.Effective() != current {
		given := string(req.Isolation)
		warnings = append(warnings, domain.Warning{
			Code:    "extract.isolation_ignored",
			Message: "--isolation " + given + " ignored: " + target.Branch + " already exists and stays " + string(current) + " — switch it with `wtm env " + target.Branch + " --isolation " + given + "`",
			Params:  domain.Params{"branch": target.Branch, "isolation": given, "current": string(current)},
		})
	}

	return warnings
}

// fastForwarded is the branch the apply fast-forwards: the one the target starts from, behind origin and asked to, unless the source's own, whose worktree holds the changes and so is never clean.
func (f Facts) fastForwarded(req Request) []string {
	subject := f.subjectAt(req, worktree.Behind)
	if req.FastForward == nil || !*req.FastForward || subject == "" || subject == req.Source {
		return nil
	}

	return []string{subject}
}

func (f Facts) divergedWarnings(req Request) []domain.Warning {
	subject := f.subjectAt(req, worktree.Diverged)
	if subject == "" {
		return nil
	}

	counts := f.AheadBehind(subject)
	return []domain.Warning{{
		Code:    "extract.source_diverged",
		Message: "branch " + subject + " has diverged from origin (" + counts.String() + ") and can't be fast-forwarded: the worktree starts from your local branch, missing commits that are on origin — you may have to rebase or resolve conflicts later",
		Params:  domain.Params{"branch": subject, "ahead": strconv.Itoa(counts.Ahead), "behind": strconv.Itoa(counts.Behind)},
	}}
}

func (f Facts) fallbackWarnings(planned Plan) []domain.Warning {
	fallsBack := env.ParentFallsBackToMain(env.ParentFallbackParams{
		Strategy:          planned.EnvFrom,
		HasFiles:          len(f.Config.Env.Files) > 0,
		SourceHasWorktree: f.CheckedOutAt(planned.From) != "",
	})
	if !fallsBack {
		return nil
	}

	return []domain.Warning{{Code: "extract.env_parent_fallback", Message: `The "parent" env strategy needs the source branch checked out to copy its .env; without a worktree it comes from main.`}}
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: the source and the files to take from it, the target, how a created target is made (its source, env strategy, isolation, fast-forward), whether the files are copied rather than moved, the conflicts and what to do with them, and the answers to remember; then the answers recalled and the recap, which the fingerprint leaves out since they change nothing the apply does.
type Plan struct {
	Source      string              `json:"source"`
	Files       []worktree.Change   `json:"files"`
	Target      Target              `json:"target,omitzero"`
	From        string              `json:"from,omitempty"`
	EnvFrom     project.EnvStrategy `json:"env_from,omitempty"`
	Isolation   project.Isolation   `json:"isolation,omitempty"`
	FastForward []string            `json:"fast_forward,omitempty"`
	Creates     bool                `json:"creates,omitempty"` // the apply creates the target first; left out of the fingerprint, which the target's action covers
	Keep        bool                `json:"keep"`
	Conflicts   []worktree.Conflict `json:"conflicts"`
	OnConflict  OnConflict          `json:"on_conflict,omitempty"`
	Remember    []string            `json:"remember,omitempty"`
	Recalled    []string            `json:"recalled,omitempty"` // the fields answered from memory, as the request was resolved
	Recap       []domain.RecapRow   `json:"recap,omitempty"`    // the plan as every surface shows it, its answers marked as the run left them
	facts       Facts
	settled     provision.Answers // as resolved, handed to provisioning even where the plan shows none, and remembered
}

// Nothing reports whether the plan has no file to extract: the source holds no uncommitted change.
func (p Plan) Nothing() bool {
	return len(p.Files) == 0
}

// Locks are the locks the plan needs: the repository shared, and the source, the target and the branch to fast-forward exclusive.
func (p Plan) Locks() []domain.Lock {
	return domain.WorktreeLocks(append([]string{p.Source, p.Target.Branch}, p.FastForward...))
}

// Fingerprinted is what the plan's fingerprint covers: the plan with what the apply reads of the project to provision a created target.
func (p Plan) Fingerprinted() any {
	p.Recalled, p.Recap, p.Creates = nil, nil, false
	return provision.Fingerprinted(p, p.facts.Repository, p.settled)
}

// Locks are the locks taken before planning: the repository shared, and the source and target given exclusive.
func (r Request) Locks() []domain.Lock {
	return domain.WorktreeLocks([]string{r.Source, r.To})
}

func (p Plan) move() worktree.Move {
	return worktree.Move{
		Source:      p.facts.CheckedOutAt(p.Source),
		Target:      p.Target.Path,
		Changes:     p.Files,
		Keep:        p.Keep,
		Resolve:     p.OnConflict == OnConflictResolve,
		TargetLabel: p.Target.Branch,
		SourceLabel: p.Source,
	}
}

// ----------------------------------------------------------------------------
// The recap

// CodeGo is the step into the target an extract ends on; CodeResolveThenDiscard the same step after an extract with conflicts: resolve them in the target, then discard the same files in the source.
const (
	CodeGo                 = Name + ".go"
	CodeResolveThenDiscard = Name + ".resolve_then_discard"
)

// NoteNew notes a target of the recap to create, from the source of its params; the others are worktree's notes.
const NoteNew = "new"

func (p Plan) previewRecap() []domain.RecapRow {
	return p.recap(domain.Marks{Recalled: p.Recalled, Remembered: p.Remember, Mark: domain.MarkRemember})
}

// appliedRecap is the recap of the plan once applied, the fields remembered by the apply marked saved; Run sets it as the applied plan's Recap.
func (p Plan) appliedRecap(remembered []string) []domain.RecapRow {
	return p.recap(domain.Marks{Recalled: p.Recalled, Remembered: remembered, Mark: domain.MarkSaved})
}

// recap is the plan row by row, none when there is nothing to extract: the source, its files, the target and how it gets its worktree, the parent of a branch checked out, the answers of a created target, the mode, then the conflicts and what to do with them.
func (p Plan) recap(marks domain.Marks) []domain.RecapRow {
	if p.Nothing() {
		return nil
	}

	rows := []domain.RecapRow{
		{Key: pathSource, Label: "Source", Path: pathSource, Values: domain.RecapValues([]string{p.Source})},
		{Key: PathFiles, Label: "Files", Path: PathFiles, Values: recappedFiles(p.Files)},
		{Key: "target", Label: "Target", Path: PathTo, Values: []domain.RecapValue{p.recappedTarget()}},
	}
	if p.Target.Action == ActionCheckOut {
		rows = append(rows, domain.RecapRow{Key: "parent", Label: "Parent", Path: pathFrom, Values: domain.RecapValues([]string{p.From})})
	}
	rows = append(rows, provision.RecapRows(provision.RecapParams{EnvFrom: p.EnvFrom, FollowsConfig: p.settled.FollowsConfig(), Isolation: p.Isolation, FastForward: p.FastForward, Marks: marks})...)

	mode := "move"
	if p.Keep {
		mode = "copy"
	}
	rows = append(rows, domain.RecapRow{Key: "mode", Label: "Mode", Path: pathKeep, Values: domain.RecapValues([]string{mode})})
	if len(p.Conflicts) > 0 {
		rows = append(rows,
			domain.RecapRow{Key: "conflicts", Label: "Conflicts", Values: recappedConflicts(p.Conflicts)},
			domain.RecapRow{Key: pathOnConflict, Label: "On conflict", Path: pathOnConflict, Values: domain.RecapValues([]string{string(p.OnConflict)})},
		)
	}

	return rows
}

func (p Plan) recappedTarget() domain.RecapValue {
	target := p.Target
	value := domain.RecapValue{Value: target.Branch, Text: target.Branch}
	switch target.Action {
	case ActionCreate:
		value.Note, value.Params, value.Text = NoteNew, domain.Params{"from": p.From}, "new worktree "+target.Branch+" from "+p.From
	case ActionCheckOut:
		value.Note, value.Text = worktree.NoteExisting, target.Branch+" (existing local branch — reused)"
	case ActionResume:
		value.Note, value.Params, value.Text = worktree.NoteResumes, domain.Params{"upstream": target.Upstream}, worktree.Resumes(target.Branch, target.Upstream)
	case ActionExisting:
	}

	return value
}

// recappedFiles are the files, each noted with its status, a renamed one with its old path, and spelled with its tag.
func recappedFiles(files []worktree.Change) []domain.RecapValue {
	values := make([]domain.RecapValue, 0, len(files))
	for _, file := range files {
		value := domain.RecapValue{Value: file.Path, Note: string(file.Status), Text: file.Status.Tag() + " " + file.Label()}
		if file.OrigPath != "" {
			value.Params = domain.Params{"orig_path": file.OrigPath}
		}
		values = append(values, value)
	}

	return values
}

// recappedConflicts are the files in conflict, each noted with why.
func recappedConflicts(conflicts []worktree.Conflict) []domain.RecapValue {
	values := make([]domain.RecapValue, 0, len(conflicts))
	for _, conflict := range conflicts {
		values = append(values, domain.RecapValue{Value: conflict.Path, Note: string(conflict.Kind), Text: conflict.Path + " (" + string(conflict.Kind) + ")"})
	}

	return values
}
