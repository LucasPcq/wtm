package extract

import (
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what extract decides from, as Observe read them: the project's config, its run.toml, the repository's snapshot, the uncommitted changes of the source, the meta records of the source and target, and what lies at the path a new target would take.
type Facts struct {
	MainPath string
	Config   project.Config
	Global   project.Global
	Run      project.Run
	RunErr   error // why run.toml could not be loaded, nil when it was
	worktree.Snapshot
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

	return f.pathOf(to)
}

func (f Facts) pathOf(name string) string {
	return filepath.Join(f.MainPath, f.Config.Worktrees.BasePath, worktree.PathSegment(name))
}

// namesWorktrees reports whether run.toml gives each worktree a name derived from its branch, so two branches deriving the same name clash; a run.toml that failed to load is assumed to.
func (f Facts) namesWorktrees() bool {
	return f.RunErr != nil || len(f.Run.Jobs) > 0 || len(f.Run.Profiles) > 0
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

func aheadAndBehind(counts worktree.AheadBehind) string {
	return strconv.Itoa(counts.Ahead) + " ahead, " + strconv.Itoa(counts.Behind) + " behind"
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

//go-sumtype:decl decision

// A decision is what becomes of the target asked: its Target, or the refusal that stops the request.
type decision interface{ decision() }

// A refusal is why the target is refused; KindInvalid makes it a field error of --to, any other kind refuses the request with its code.
type refusal struct {
	Kind     domain.Kind
	Code     string
	Template string
	Params   domain.Params
}

func (Target) decision()  {}
func (refusal) decision() {}

func (r refusal) field(path string) domain.FieldError {
	return domain.FieldError{Path: path, Code: r.Code, Template: r.Template, Params: r.Params}
}

func (r refusal) error(path string) *domain.Error {
	if r.Kind == domain.KindInvalid {
		return domain.Invalid([]domain.FieldError{r.field(path)})
	}

	return &domain.Error{Kind: r.Kind, Code: r.Code, Message: r.field(path).Message(path), Params: r.Params}
}

// target decides the target: the worktree of to when it has one, else a worktree placed for it as create would place it.
func (f Facts) target(req Request) decision {
	to := req.To
	switch {
	case worktree.BlankBranch(to):
		return refusal{Kind: domain.KindInvalid, Code: codeRequired, Template: "{path} is required"}
	case to == req.Source:
		return refusal{Kind: domain.KindInvalid, Code: "extract.same_worktree", Template: "{path}: {value} is the source — pick another worktree", Params: domain.Params{"value": to}}
	}

	if at := f.CheckedOutAt(to); at != "" {
		return Target{Branch: to, Path: at, Action: ActionExisting}
	}

	if err := worktree.BranchProblem(to); err != nil {
		return refusal{Kind: domain.KindInvalid, Code: "extract.branch_name_invalid", Template: "{problem}", Params: domain.Params{"problem": err.Error()}}
	}

	var live []string
	if f.namesWorktrees() {
		live = f.Live()
	}

	created := Target{Branch: to, Path: f.pathOf(to)}
	return created.placed(worktree.Place(worktree.PlacementParams{
		Branch:   to,
		Path:     created.Path,
		Location: f.Folder,
		Occupant: f.Occupant(f.Folder),
		Local:    f.Local(to),
		Remote:   f.Remote(to),
		Live:     live,
	}))
}

// placed turns where the domain places the target into extract's decision; a worktree the branch already has is the target as it stands.
func (t Target) placed(placement worktree.Placement) decision {
	switch p := placement.(type) {
	case worktree.Unchanged:
		return Target{Branch: t.Branch, Path: p.Path, Action: ActionExisting}
	case worktree.CheckedOutElsewhere:
		return Target{Branch: t.Branch, Path: p.Path, Action: ActionExisting}
	case worktree.PathTaken:
		return refusal{Kind: domain.KindConflict, Code: "extract.path_exists", Template: "worktree path already exists: {path}", Params: domain.Params{"path": t.Path}}
	case worktree.Clash:
		return refusal{
			Kind:     domain.KindConflict,
			Code:     "extract.name_clash",
			Template: "branch {branch} would share its name with {other} ({name}) — pick another name",
			Params:   domain.Params{"branch": t.Branch, "other": p.Branch, "name": p.Name},
		}
	case worktree.Placed:
		return Target{Branch: t.Branch, Path: t.Path, Action: Action(p.Start), Upstream: p.Upstream}
	}

	return nil
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

// subjectAt is the local branch a created target starts from — the existing branch it checks out, or the source of a new branch — when its standing against origin is standing, else "".
func (f Facts) subjectAt(req Request, standing worktree.Divergence) string {
	target, ok := f.planned(req)
	subject := ""
	switch {
	case !ok:
	case target.Action == ActionCheckOut:
		subject = target.Branch
	case target.Action == ActionCreate && f.Local(req.From):
		subject = req.From
	}

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

// Decide turns the facts, the resolved request and the conflicts the changes would meet into the plan and its warnings. A source with nothing to extract is a plan with no files. A target refused is a field error of --to or a refusal with its code; conflicts are refused with CodeConflict unless the request resolves them, and binary ones always are.
func Decide(f Facts, req Request, conflicts []worktree.Conflict) (Plan, []domain.Warning, error) {
	planned := Plan{Source: req.Source, Files: []worktree.Change{}, Conflicts: []worktree.Conflict{}, facts: f}
	if f.nothing(req.Source) {
		return planned, nil, nil
	}

	switch decided := f.target(req).(type) {
	case refusal:
		return Plan{}, nil, decided.error("to")
	case Target:
		planned.Target = decided
	}

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

	planned.From = req.From
	planned.Isolation = f.plannedIsolation(req.Isolation)
	planned.FastForward = f.fastForwarded(req)
	planned.Remember = req.Remember
	planned.isolation = req.Isolation
	planned.fastForward = req.FastForward != nil && *req.FastForward
	return planned, append(f.divergedWarnings(req), f.fallbackWarnings(planned)...), nil
}

// refusedFor is the refusal of the conflicts, nil when there are none or the request resolves them; a binary conflict cannot take conflict markers, so it is refused even then.
func refusedFor(target string, onConflict OnConflict, conflicts []worktree.Conflict) *domain.Error {
	binary := slices.ContainsFunc(conflicts, func(c worktree.Conflict) bool { return c.Kind == worktree.ConflictBinary })
	if !binary && (len(conflicts) == 0 || onConflict == OnConflictResolve) {
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

// plannedIsolation is the isolation the plan shows: none when run.toml declares nothing a worktree isolates.
func (f Facts) plannedIsolation(given project.Isolation) project.Isolation {
	if !f.Run.IsolationApplies() {
		return ""
	}

	return given
}

func (f Facts) fastForwarded(req Request) []string {
	subject := f.subjectAt(req, worktree.Behind)
	if req.FastForward == nil || !*req.FastForward || subject == "" {
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
		Message: "branch " + subject + " has diverged from origin (" + aheadAndBehind(counts) + ") and can't be fast-forwarded: the worktree starts from your local branch, missing commits that are on origin — you may have to rebase or resolve conflicts later",
		Params:  domain.Params{"branch": subject, "ahead": strconv.Itoa(counts.Ahead), "behind": strconv.Itoa(counts.Behind)},
	}}
}

func (f Facts) fallbackWarnings(planned Plan) []domain.Warning {
	fallsBack := env.ParentFallsBackToMain(env.ParentFallbackParams{
		Strategy:          f.Config.Env.Strategy,
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

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: the source and the files to take from it, the target, how a created target is made (its source, isolation, fast-forward), whether the files are copied rather than moved, the conflicts and what to do with them, and the answers to remember.
type Plan struct {
	Source      string              `json:"source"`
	Files       []worktree.Change   `json:"files"`
	Target      Target              `json:"target,omitzero"`
	From        string              `json:"from,omitempty"`
	Isolation   project.Isolation   `json:"isolation,omitempty"`
	FastForward []string            `json:"fast_forward,omitempty"`
	Keep        bool                `json:"keep"`
	Conflicts   []worktree.Conflict `json:"conflicts"`
	OnConflict  OnConflict          `json:"on_conflict,omitempty"`
	Remember    []string            `json:"remember,omitempty"`
	facts       Facts
	isolation   project.Isolation // as resolved, handed to provisioning even where the plan shows none
	fastForward bool              // as answered, even with nothing to fast-forward, for the remembered answer
}

// Nothing reports whether the plan has no file to extract: the source holds no uncommitted change.
func (p Plan) Nothing() bool {
	return len(p.Files) == 0
}

// Locks are the locks the plan needs: the repository shared, and the source, the target and the branch to fast-forward exclusive.
func (p Plan) Locks() []domain.Lock {
	return locksOf(append([]string{p.Source, p.Target.Branch}, p.FastForward...))
}

// Fingerprinted is what the plan's fingerprint covers: the plan, the isolation as resolved, and what the apply reads of the project to provision a created target (its env files and strategy, its on_create hooks, its run.toml and the global settings).
func (p Plan) Fingerprinted() any {
	f := p.facts
	return struct {
		Plan      Plan              `json:"plan"`
		Isolation project.Isolation `json:"isolation"`
		Env       project.Env       `json:"env"`
		OnCreate  []project.Hook    `json:"on_create"`
		Run       project.Run       `json:"run"`
		RunErr    string            `json:"run_err,omitempty"`
		Global    project.Global    `json:"global"`
	}{Plan: p, Isolation: p.isolation, Env: f.Config.Env, OnCreate: f.Config.OnCreate, Run: f.Run, RunErr: errorText(f.RunErr), Global: f.Global}
}

func errorText(err error) string {
	if err == nil {
		return ""
	}

	return err.Error()
}

// Locks are the locks taken before planning: the repository shared, and the source and target given exclusive.
func (r Request) Locks() []domain.Lock {
	return locksOf([]string{r.Source, r.To})
}

// locksOf sorts the worktrees so that two operations always take their locks in the same order.
func locksOf(worktrees []string) []domain.Lock {
	locks := []domain.Lock{{Scope: domain.ScopeRepo, Mode: domain.LockShared}}
	worktrees = slices.DeleteFunc(worktrees, func(name string) bool { return name == "" })
	slices.Sort(worktrees)
	for _, name := range slices.Compact(worktrees) {
		locks = append(locks, domain.Lock{Scope: domain.ScopeWorktree, Name: name, Mode: domain.LockExclusive})
	}

	return locks
}

func (p Plan) checkout() worktree.Checkout {
	switch p.Target.Action {
	case ActionCheckOut:
		return worktree.ExistingBranch{Name: p.Target.Branch}
	case ActionResume:
		return worktree.TrackingBranch{Name: p.Target.Branch}
	case ActionCreate, ActionExisting:
	}

	return worktree.NewBranch{Name: p.Target.Branch, From: p.facts.Ref(p.From)}
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

func (f Facts) standingOf(target Target) Standing {
	switch target.Action {
	case ActionCheckOut:
		counts := f.AheadBehind(target.Branch)
		return Standing{ExistingBranch: true, OriginState: f.Standing(target.Branch), OriginAhead: counts.Ahead, OriginBehind: counts.Behind}
	case ActionResume:
		return Standing{Upstream: target.Upstream}
	case ActionCreate, ActionExisting:
	}

	return Standing{}
}

// answers are the answers to remember, keyed by their id in config.toml.
func (p Plan) answers() map[string]string {
	sourceUpdate := project.SourceUpdateKeep
	if p.fastForward {
		sourceUpdate = project.SourceUpdateFastForward
	}

	values := map[string]string{"isolation": string(p.Isolation), "fast_forward": sourceUpdate}
	answers := map[string]string{}
	for _, r := range remembered {
		if value := values[r.field]; value != "" && slices.Contains(p.Remember, r.field) {
			answers[r.id] = value
		}
	}

	return answers
}
