package create

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/env"
	"github.com/LucasPcq/wtm/next/internal/wtm/project"
	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"
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

func (d Deps) Observe(ctx context.Context, branches []string) (Facts, error) {
	config, global, err := d.LoadConfig()
	if errors.Is(err, project.ErrConfigNotFound) {
		return Facts{}, &wtm.Error{Kind: wtm.KindPrecondition, Code: CodeConfigNotFound, Message: "no wtm config found — run `wtm init` first", Cause: err}
	}
	if err != nil {
		return Facts{}, err
	}
	worktrees, err := d.Git.Worktrees(ctx)
	if err != nil {
		return Facts{}, err
	}
	refs, err := d.Git.Branches(ctx)
	if err != nil {
		return Facts{}, err
	}
	divergence, err := d.divergence(ctx, refs)
	if err != nil {
		return Facts{}, err
	}
	facts := Facts{MainPath: d.MainPath, Config: config, Global: global, Worktrees: worktrees, Branches: refs, Divergence: divergence}
	folders, err := d.locate(ctx, facts.pathsOf(branches))
	if err != nil {
		return Facts{}, err
	}
	facts.Folders = folders
	facts.Run, facts.RunErr = d.LoadRun()
	return facts, nil
}

func (d Deps) divergence(ctx context.Context, refs worktree.Branches) (map[string]worktree.AheadBehind, error) {
	found := map[string]worktree.AheadBehind{}
	for _, local := range refs.Local {
		at := slices.IndexFunc(refs.Remote, func(remote worktree.Branch) bool { return remote.Name == local.Name })
		if at < 0 || refs.Remote[at].Tip == local.Tip {
			continue
		}
		counts, err := d.Git.AheadBehind(ctx, local.Name)
		if err != nil {
			return nil, err
		}
		found[local.Name] = counts
	}
	return found, nil
}

func (d Deps) locate(ctx context.Context, paths []string) (map[string]worktree.Location, error) {
	folders := make(map[string]worktree.Location, len(paths))
	for _, path := range paths {
		location, err := d.Git.Locate(ctx, path)
		if err != nil {
			return nil, err
		}
		folders[path] = location
	}
	return folders, nil
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
	Kind     wtm.Kind
	Code     string
	Template string
	Params   wtm.Params
}

func (BranchPlan) decision() {}
func (refusal) decision()    {}

func (r refusal) field(index int) wtm.FieldError {
	return wtm.FieldError{Path: branchPath(index), Code: r.Code, Template: r.Template, Params: r.Params}
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
		return e.clashed(clash).as(wtm.KindConflict)
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
		return refusal{Kind: wtm.KindInvalid, Code: codeRequired, Template: "{path} is required"}, true
	}
	if slices.Contains(e.earlier, e.name) {
		return refusal{Kind: wtm.KindInvalid, Code: "distinct", Template: "{path}: {value} is given twice", Params: wtm.Params{"value": e.name}}, true
	}
	if err := worktree.BranchProblem(e.name); err != nil {
		return refusal{Kind: wtm.KindInvalid, Code: "create.branch_name_invalid", Template: "{problem}", Params: wtm.Params{"problem": err.Error()}}, true
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
		Kind:     wtm.KindConflict,
		Code:     "create.checked_out_elsewhere",
		Template: "branch {branch} is checked out at {path} — run `wtm go {branch}` to jump in",
		Params:   wtm.Params{"branch": e.name, "path": at},
	}
}

func (e entry) occupied(occupant string) decision {
	switch {
	case e.ifNotExists && occupant == e.name:
		return BranchPlan{Branch: e.name, Path: e.path, Action: ActionUnchanged}
	case occupant == "" || occupant == e.name:
		return refusal{Kind: wtm.KindConflict, Code: "create.path_exists", Template: "worktree path already exists: {path}", Params: wtm.Params{"path": e.path}}
	}
	return e.clashed(worktree.Clash{Branch: occupant, Name: filepath.Base(e.path)}).as(wtm.KindConflict)
}

func (e entry) clashed(clash worktree.Clash) refusal {
	return refusal{
		Kind:     wtm.KindInvalid,
		Code:     "create.name_clash",
		Template: "branch {branch} would share its name with {other} ({name}) — pick another name",
		Params:   wtm.Params{"branch": e.name, "other": clash.Branch, "name": clash.Name},
	}
}

func (r refusal) as(kind wtm.Kind) refusal {
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

func Decide(f Facts, req Request) (Plan, []wtm.Warning, error) {
	branches := make([]BranchPlan, 0, len(req.Branches))
	var invalid []wtm.FieldError
	for index, d := range f.decisions(req) {
		switch decided := d.(type) {
		case refusal:
			if decided.Kind != wtm.KindInvalid {
				return Plan{}, nil, &wtm.Error{Kind: decided.Kind, Code: decided.Code, Message: decided.field(index).Message(branchPath(index)), Params: decided.Params}
			}
			invalid = append(invalid, decided.field(index))
		case BranchPlan:
			branches = append(branches, decided)
		}
	}
	if len(invalid) > 0 {
		return Plan{}, nil, wtm.Invalid(invalid)
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

func (f Facts) divergedWarnings(req Request) []wtm.Warning {
	var warnings []wtm.Warning
	for _, subject := range f.subjectsAt(req, worktree.Diverged) {
		counts := f.Divergence[subject]
		warnings = append(warnings, wtm.Warning{
			Code:    "create.source_diverged",
			Message: "branch " + subject + " has diverged from origin (" + aheadAndBehind(counts) + ") and can't be fast-forwarded: the worktree starts from your local branch, missing commits that are on origin — you may have to rebase or resolve conflicts later",
			Params:  wtm.Params{"branch": subject, "ahead": strconv.Itoa(counts.Ahead), "behind": strconv.Itoa(counts.Behind)},
		})
	}
	return warnings
}

func (f Facts) fallbackWarnings(planned Plan) []wtm.Warning {
	creates := slices.ContainsFunc(planned.Branches, func(b BranchPlan) bool { return b.Action != ActionUnchanged })
	fallsBack := env.ParentFallsBackToMain(env.ParentFallbackParams{
		Strategy:          planned.EnvFrom,
		HasFiles:          len(f.Config.Env.Files) > 0,
		SourceHasWorktree: f.checkedOutAt(planned.From) != "",
	})
	if !creates || !fallsBack {
		return nil
	}
	return []wtm.Warning{{Code: "create.env_parent_fallback", Message: `The "parent" env strategy needs the source branch checked out to copy its .env; without a worktree it comes from main.`}}
}
