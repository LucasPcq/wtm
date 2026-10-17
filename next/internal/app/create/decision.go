package create

import (
	"fmt"
	"path/filepath"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"

	"github.com/LucasPcq/wtm/next/internal/wtm"
)

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
