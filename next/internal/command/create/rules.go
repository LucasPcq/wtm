package create

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
	Folders    map[string]worktree.Location
}

func (f Facts) pathsOf(branches []string) []string {
	var paths []string
	for _, name := range branches {
		if !worktree.BlankBranch(name) {
			paths = append(paths, f.pathOf(name))
		}
	}
	return paths
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

func cutRemote(name string) (string, bool) {
	return strings.CutPrefix(name, remotePrefix)
}

func aheadAndBehind(counts worktree.AheadBehind) string {
	return strconv.Itoa(counts.Ahead) + " ahead, " + strconv.Itoa(counts.Behind) + " behind"
}

//go-sumtype:decl decision

type decision interface{ decision() }

type refusal struct {
	Kind     domain.Kind
	Code     string
	Template string
	Params   domain.Params
}

func (BranchPlan) decision() {}
func (refusal) decision()    {}

func (r refusal) field(index int) domain.FieldError {
	return domain.FieldError{Path: branchPath(index), Code: r.Code, Template: r.Template, Params: r.Params}
}

type entry struct {
	name        string
	path        string
	earlier     []string
	ifNotExists bool
}

func (f Facts) decisions(req Request) []decision {
	decisions := make([]decision, 0, len(req.Branches))
	for index, name := range req.Branches {
		decisions = append(decisions, f.decide(entry{name: name, path: f.pathOf(name), earlier: req.Branches[:index], ifNotExists: req.IfNotExists}))
	}
	return decisions
}

func (f Facts) decide(e entry) decision {
	if refused, found := f.invalid(e); found {
		return refused
	}
	if at := f.checkedOutAt(e.name); at != "" {
		return e.open(at)
	}
	location, located := f.Folders[e.path]
	if !located {
		panic(fmt.Sprintf("create: the folder of %s was never located: observe with the branches of the request", e.name))
	}
	if location.Exists {
		return e.occupied(f.occupant(location))
	}
	if clash, found := f.clash(worktree.ClashParams{Branch: e.name, Live: f.live()}); found {
		return e.clashed(clash).as(domain.KindConflict)
	}
	switch {
	case f.local(e.name):
		return BranchPlan{Branch: e.name, Path: e.path, Action: ActionCheckOut}
	case f.remote(e.name):
		return BranchPlan{Branch: e.name, Path: e.path, Action: ActionResume, Upstream: remotePrefix + e.name}
	}
	return BranchPlan{Branch: e.name, Path: e.path, Action: ActionCreate}
}

func (f Facts) invalid(e entry) (refusal, bool) {
	if worktree.BlankBranch(e.name) {
		return refusal{Kind: domain.KindInvalid, Code: codeRequired, Template: "{path} is required"}, true
	}
	if slices.Contains(e.earlier, e.name) {
		return refusal{Kind: domain.KindInvalid, Code: "distinct", Template: "{path}: {value} is given twice", Params: domain.Params{"value": e.name}}, true
	}
	if err := worktree.BranchProblem(e.name); err != nil {
		return refusal{Kind: domain.KindInvalid, Code: "create.branch_name_invalid", Template: "{problem}", Params: domain.Params{"problem": err.Error()}}, true
	}
	if at := slices.IndexFunc(e.earlier, func(earlier string) bool { return f.pathOf(earlier) == e.path }); at >= 0 {
		return e.clashed(worktree.Clash{Branch: e.earlier[at], Name: filepath.Base(e.path)}), true
	}
	if clash, found := f.clash(worktree.ClashParams{Branch: e.name, Live: e.earlier}); found {
		return e.clashed(clash), true
	}
	return refusal{}, false
}

func (f Facts) clash(params worktree.ClashParams) (worktree.Clash, bool) {
	if !f.namesWorktrees() {
		return worktree.Clash{}, false
	}
	return worktree.ClashWith(params)
}

func (e entry) open(at string) decision {
	if e.ifNotExists {
		return BranchPlan{Branch: e.name, Path: at, Action: ActionUnchanged}
	}
	return refusal{
		Kind:     domain.KindConflict,
		Code:     "create.checked_out_elsewhere",
		Template: "branch {branch} is checked out at {path} — run `wtm go {branch}` to jump in",
		Params:   domain.Params{"branch": e.name, "path": at},
	}
}

func (e entry) occupied(occupant string) decision {
	switch {
	case e.ifNotExists && occupant == e.name:
		return BranchPlan{Branch: e.name, Path: e.path, Action: ActionUnchanged}
	case occupant == "" || occupant == e.name:
		return refusal{Kind: domain.KindConflict, Code: "create.path_exists", Template: "worktree path already exists: {path}", Params: domain.Params{"path": e.path}}
	}
	return e.clashed(worktree.Clash{Branch: occupant, Name: filepath.Base(e.path)}).as(domain.KindConflict)
}

func (e entry) clashed(clash worktree.Clash) refusal {
	return refusal{
		Kind:     domain.KindInvalid,
		Code:     "create.name_clash",
		Template: "branch {branch} would share its name with {other} ({name}) — pick another name",
		Params:   domain.Params{"branch": e.name, "other": clash.Branch, "name": clash.Name},
	}
}

func (r refusal) as(kind domain.Kind) refusal {
	r.Kind = kind
	return r
}

func (f Facts) checksOut(req Request) bool {
	return slices.ContainsFunc(f.decisions(req), func(d decision) bool {
		planned, isPlanned := d.(BranchPlan)
		return isPlanned && planned.Action == ActionCheckOut
	})
}

func (f Facts) subjectsAt(req Request, standing worktree.Divergence) []string {
	var subjects []string
	for _, d := range f.decisions(req) {
		planned, isPlanned := d.(BranchPlan)
		subject := ""
		switch {
		case !isPlanned:
		case planned.Action == ActionCheckOut:
			subject = planned.Branch
		case planned.Action == ActionCreate && f.local(req.From):
			subject = req.From
		}
		if subject != "" && f.standing(subject) == standing && !slices.Contains(subjects, subject) {
			subjects = append(subjects, subject)
		}
	}
	return subjects
}

func Decide(f Facts, req Request) (Plan, []domain.Warning, error) {
	branches := make([]BranchPlan, 0, len(req.Branches))
	var invalid []domain.FieldError
	for index, d := range f.decisions(req) {
		switch decided := d.(type) {
		case refusal:
			if decided.Kind != domain.KindInvalid {
				return Plan{}, nil, &domain.Error{Kind: decided.Kind, Code: decided.Code, Message: decided.field(index).Message(branchPath(index)), Params: decided.Params}
			}
			invalid = append(invalid, decided.field(index))
		case BranchPlan:
			branches = append(branches, decided)
		}
	}
	if len(invalid) > 0 {
		return Plan{}, nil, domain.Invalid(invalid)
	}
	planned := Plan{
		Branches:    branches,
		From:        req.From,
		EnvFrom:     req.EnvFrom,
		Isolation:   f.plannedIsolation(req.Isolation),
		FastForward: f.fastForwarded(req),
		Remember:    req.Remember,
		facts:       f,
		isolation:   req.Isolation,
		fastForward: req.FastForward != nil && *req.FastForward,
	}
	return planned, append(f.divergedWarnings(req), f.fallbackWarnings(planned)...), nil
}

func (f Facts) plannedIsolation(given project.Isolation) project.Isolation {
	if !f.Run.IsolationApplies() {
		return ""
	}
	return given
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
		counts := f.Divergence[subject]
		warnings = append(warnings, domain.Warning{
			Code:    "create.source_diverged",
			Message: "branch " + subject + " has diverged from origin (" + aheadAndBehind(counts) + ") and can't be fast-forwarded: the worktree starts from your local branch, missing commits that are on origin — you may have to rebase or resolve conflicts later",
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
		SourceHasWorktree: f.checkedOutAt(planned.From) != "",
	})
	if !creates || !fallsBack {
		return nil
	}
	return []domain.Warning{{Code: "create.env_parent_fallback", Message: `The "parent" env strategy needs the source branch checked out to copy its .env; without a worktree it comes from main.`}}
}

type Action string

const (
	ActionCreate    Action = "create"
	ActionCheckOut  Action = "check_out"
	ActionResume    Action = "resume"
	ActionUnchanged Action = "unchanged"
)

type Plan struct {
	Branches    []BranchPlan        `json:"branches"`
	From        string              `json:"from,omitempty"`
	EnvFrom     project.EnvStrategy `json:"env_from"`
	Isolation   project.Isolation   `json:"isolation,omitempty"`
	FastForward []string            `json:"fast_forward,omitempty"`
	Remember    []string            `json:"remember,omitempty"`
	facts       Facts
	isolation   project.Isolation
	fastForward bool
}

type BranchPlan struct {
	Branch   string `json:"branch"`
	Path     string `json:"path"`
	Action   Action `json:"action"`
	Upstream string `json:"upstream,omitempty"`
}

func (p Plan) Locks() []domain.Lock {
	names := make([]string, 0, len(p.Branches)+len(p.FastForward))
	for _, planned := range p.Branches {
		names = append(names, planned.Branch)
	}
	return locksOf(append(names, p.FastForward...))
}

func (p Plan) Fingerprinted() any {
	return struct {
		Plan      Plan              `json:"plan"`
		Isolation project.Isolation `json:"isolation"`
	}{Plan: p, Isolation: p.isolation}
}

func (r Request) Locks() []domain.Lock {
	return locksOf(slices.DeleteFunc(slices.Clone(r.Branches), func(name string) bool { return name == "" }))
}

func locksOf(worktrees []string) []domain.Lock {
	locks := []domain.Lock{{Scope: domain.ScopeRepo, Mode: domain.LockShared}}
	slices.Sort(worktrees)
	for _, name := range slices.Compact(worktrees) {
		locks = append(locks, domain.Lock{Scope: domain.ScopeWorktree, Name: name, Mode: domain.LockExclusive})
	}
	return locks
}

func (p Plan) checkout(planned BranchPlan) worktree.Checkout {
	switch planned.Action {
	case ActionCheckOut:
		return worktree.ExistingBranch{Name: planned.Branch}
	case ActionResume:
		return worktree.TrackingBranch{Name: planned.Branch}
	case ActionCreate, ActionUnchanged:
	}
	return worktree.NewBranch{Name: planned.Branch, From: p.facts.ref(p.From)}
}

func (f Facts) ref(from string) worktree.Ref {
	if remote, onOrigin := cutRemote(from); onOrigin && !f.local(from) {
		return worktree.RemoteBranch{Name: remote}
	}
	return worktree.LocalBranch{Name: from}
}

func (f Facts) standingOf(planned BranchPlan) Standing {
	switch planned.Action {
	case ActionCheckOut:
		counts := f.Divergence[planned.Branch]
		return Standing{ExistingBranch: true, OriginState: f.standing(planned.Branch), OriginAhead: counts.Ahead, OriginBehind: counts.Behind}
	case ActionResume:
		return Standing{Upstream: planned.Upstream}
	case ActionCreate, ActionUnchanged:
	}
	return Standing{}
}

func (p Plan) answers() map[string]string {
	sourceUpdate := project.SourceUpdateKeep
	if p.fastForward {
		sourceUpdate = project.SourceUpdateFastForward
	}
	values := map[string]string{"env_from": string(p.EnvFrom), "isolation": string(p.Isolation), "fast_forward": sourceUpdate}
	answers := map[string]string{}
	for _, r := range remembered {
		if value := values[r.field]; value != "" && slices.Contains(p.Remember, r.field) {
			answers[r.id] = value
		}
	}
	return answers
}
