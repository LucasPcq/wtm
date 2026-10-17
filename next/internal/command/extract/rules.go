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

const remotePrefix = "origin/"

type Facts struct {
	MainPath   string
	Config     project.Config
	Global     project.Global
	Run        project.Run
	RunErr     error
	Worktrees  []worktree.Worktree
	Branches   worktree.Branches
	Divergence map[string]worktree.AheadBehind
	Changes    map[string][]worktree.Change
	Records    map[string]worktree.Meta
	Folder     worktree.Location
}

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
	return slices.DeleteFunc(names, func(name string) bool { return name == "" || f.checkedOutAt(name) == "" })
}

func (f Facts) newPath(to string) string {
	if worktree.BlankBranch(to) || f.checkedOutAt(to) != "" {
		return ""
	}
	return f.pathOf(to)
}

func (f Facts) pathOf(name string) string {
	return filepath.Join(f.MainPath, f.Config.Worktrees.BasePath, worktree.PathSegment(name))
}

func (f Facts) local(name string) bool {
	return slices.ContainsFunc(f.Branches.Local, func(b worktree.Branch) bool { return b.Name == name })
}

func (f Facts) remote(name string) bool {
	return slices.ContainsFunc(f.Branches.Remote, func(b worktree.Branch) bool { return b.Name == name })
}

func (f Facts) candidate(name string) bool {
	remote, onOrigin := cutRemote(name)
	return f.local(name) || onOrigin && f.remote(remote)
}

func (f Facts) checkedOutAt(name string) string {
	at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Branch() == name })
	if at < 0 {
		return ""
	}
	return f.Worktrees[at].Path
}

func (f Facts) occupant(location worktree.Location) string {
	at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Path == location.Path })
	if at < 0 {
		return ""
	}
	return f.Worktrees[at].Branch()
}

func (f Facts) standing(name string) worktree.Divergence {
	if !f.remote(name) {
		return worktree.NoOrigin
	}
	return f.Divergence[name].Divergence()
}

func (f Facts) namesWorktrees() bool {
	return f.RunErr != nil || len(f.Run.Jobs) > 0 || len(f.Run.Profiles) > 0
}

func (f Facts) live() []string {
	names := make([]string, 0, len(f.Worktrees))
	for _, w := range f.Worktrees {
		names = append(names, w.Branch())
	}
	return names
}

func (f Facts) nothing(source string) bool {
	return f.checkedOutAt(source) != "" && len(f.Changes[source]) == 0
}

func (f Facts) selected(req Request) []worktree.Change {
	selected, _ := worktree.SelectChanges(f.Changes[req.Source], req.Files)
	return selected
}

func (f Facts) parent(source string) string {
	return f.Records[source].SourceBranch
}

func (f Facts) recordedIsolation(name string) project.Isolation {
	return f.Records[name].Isolation.Effective()
}

func cutRemote(name string) (string, bool) {
	return strings.CutPrefix(name, remotePrefix)
}

func aheadAndBehind(counts worktree.AheadBehind) string {
	return strconv.Itoa(counts.Ahead) + " ahead, " + strconv.Itoa(counts.Behind) + " behind"
}

type Action string

const (
	ActionExisting Action = "existing"
	ActionCreate   Action = "create"
	ActionCheckOut Action = "check_out"
	ActionResume   Action = "resume"
)

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

type decision interface{ decision() }

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

func (f Facts) target(req Request) decision {
	to := req.To
	switch {
	case worktree.BlankBranch(to):
		return refusal{Kind: domain.KindInvalid, Code: codeRequired, Template: "{path} is required"}
	case to == req.Source:
		return refusal{Kind: domain.KindInvalid, Code: "extract.same_worktree", Template: "{path}: {value} is the source — pick another worktree", Params: domain.Params{"value": to}}
	}
	if at := f.checkedOutAt(to); at != "" {
		return Target{Branch: to, Path: at, Action: ActionExisting}
	}
	if err := worktree.BranchProblem(to); err != nil {
		return refusal{Kind: domain.KindInvalid, Code: "extract.branch_name_invalid", Template: "{problem}", Params: domain.Params{"problem": err.Error()}}
	}
	var live []string
	if f.namesWorktrees() {
		live = f.live()
	}
	created := Target{Branch: to, Path: f.pathOf(to)}
	return created.placed(worktree.Place(worktree.PlacementParams{
		Branch:   to,
		Path:     created.Path,
		Location: f.Folder,
		Occupant: f.occupant(f.Folder),
		Local:    f.local(to),
		Remote:   f.remote(to),
		Live:     live,
	}))
}

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

func (f Facts) existing(req Request) bool {
	target, ok := f.planned(req)
	return ok && !target.creates()
}

func (f Facts) subjectAt(req Request, standing worktree.Divergence) string {
	target, ok := f.planned(req)
	subject := ""
	switch {
	case !ok:
	case target.Action == ActionCheckOut:
		subject = target.Branch
	case target.Action == ActionCreate && f.local(req.From):
		subject = req.From
	}
	if subject == "" || f.standing(subject) != standing {
		return ""
	}
	return subject
}

func (f Facts) conflictCheck(req Request) worktree.ConflictCheck {
	check := worktree.ConflictCheck{Source: f.checkedOutAt(req.Source), Changes: f.selected(req)}
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
	return check
}

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

func refusedFor(target string, onConflict OnConflict, conflicts []worktree.Conflict) *domain.Error {
	binary := slices.ContainsFunc(conflicts, func(c worktree.Conflict) bool { return c.Kind == worktree.ConflictBinary })
	if !binary && (len(conflicts) == 0 || onConflict == OnConflictResolve) {
		return nil
	}
	return conflictRefusal(conflictRefusalParams{Conflicts: conflicts, Target: target})
}

type conflictRefusalParams struct {
	Conflicts []worktree.Conflict
	Target    string
}

func conflictRefusal(params conflictRefusalParams) *domain.Error {
	byKind := map[worktree.ConflictKind][]string{}
	var all []string
	for _, conflict := range params.Conflicts {
		byKind[conflict.Kind] = append(byKind[conflict.Kind], conflict.Path)
		all = append(all, conflict.Path)
	}
	refusal := &domain.Error{Kind: domain.KindConflict, Code: CodeConflict, Params: domain.Params{"target": params.Target, "files": strings.Join(all, ", ")}}
	if binary := byKind[worktree.ConflictBinary]; len(binary) > 0 {
		refusal.Params["files"] = strings.Join(binary, ", ")
		refusal.Message = fmt.Sprintf("cannot apply the selected changes: %s %s in %q and %s binary — conflict markers cannot be written, remove %s there or extract to another worktree",
			strings.Join(binary, ", "), plural(len(binary), "already exists", "already exist"), params.Target, plural(len(binary), "is", "are"), plural(len(binary), "it", "them"))
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
		strings.Join(clauses, " and "), params.Target, plural(len(all), "it", "them"))
	return refusal
}

func plural(count int, one, many string) string {
	if count == 1 {
		return one
	}
	return many
}

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
	counts := f.Divergence[subject]
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
		SourceHasWorktree: f.checkedOutAt(planned.From) != "",
	})
	if !fallsBack {
		return nil
	}
	return []domain.Warning{{Code: "extract.env_parent_fallback", Message: `The "parent" env strategy needs the source branch checked out to copy its .env; without a worktree it comes from main.`}}
}

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
	isolation   project.Isolation
	fastForward bool
}

func (p Plan) Nothing() bool {
	return len(p.Files) == 0
}

func (p Plan) Locks() []domain.Lock {
	return locksOf(append([]string{p.Source, p.Target.Branch}, p.FastForward...))
}

func (p Plan) Fingerprinted() any {
	return struct {
		Plan      Plan              `json:"plan"`
		Isolation project.Isolation `json:"isolation"`
	}{Plan: p, Isolation: p.isolation}
}

func (r Request) Locks() []domain.Lock {
	return locksOf([]string{r.Source, r.To})
}

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
	return worktree.NewBranch{Name: p.Target.Branch, From: p.facts.ref(p.From)}
}

func (f Facts) ref(from string) worktree.Ref {
	if remote, onOrigin := cutRemote(from); onOrigin && !f.local(from) {
		return worktree.RemoteBranch{Name: remote}
	}
	return worktree.LocalBranch{Name: from}
}

func (p Plan) move() worktree.Move {
	return worktree.Move{
		Source:      p.facts.checkedOutAt(p.Source),
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
		counts := f.Divergence[target.Branch]
		return Standing{ExistingBranch: true, OriginState: f.standing(target.Branch), OriginAhead: counts.Ahead, OriginBehind: counts.Behind}
	case ActionResume:
		return Standing{Upstream: target.Upstream}
	case ActionCreate, ActionExisting:
	}
	return Standing{}
}

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
