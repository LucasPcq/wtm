package create

import (
	"path/filepath"

	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
)

//go-sumtype:decl decision

type decision interface{ decision() }

type refusal struct {
	Code    string
	Message string
	Params  wtm.Params
}

func (BranchPlan) decision() {}
func (refusal) decision()    {}

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
	if branch.Blank(e.name) {
		return refusal{Code: codeRequired, Message: "{path} is required"}
	}
	if err := branch.Problem(e.name); err != nil {
		return refusal{Code: "create.branch_name_invalid", Message: err.Error(), Params: wtm.Params{"problem": err.Error()}}
	}
	if clash, found := f.clash(branch.ClashParams{Branch: e.name, Live: e.earlier}); found {
		return e.clashed(clash)
	}
	if at := f.checkedOutAt(e.name); at != "" {
		return e.open(at)
	}
	if location, taken := f.Taken[e.path]; taken {
		return e.occupied(f.occupant(location))
	}
	if clash, found := f.clash(branch.ClashParams{Branch: e.name, Live: f.live()}); found {
		return e.clashed(clash)
	}
	switch {
	case f.local(e.name):
		return BranchPlan{Branch: e.name, Path: e.path, Action: ActionCheckOut}
	case f.remote(e.name):
		return BranchPlan{Branch: e.name, Path: e.path, Action: ActionResume, Upstream: remotePrefix + e.name}
	}
	return BranchPlan{Branch: e.name, Path: e.path, Action: ActionCreate}
}

func (f Facts) clash(params branch.ClashParams) (branch.Clash, bool) {
	if !f.namesWorktrees() {
		return branch.Clash{}, false
	}
	return branch.ClashWith(params)
}

func (e entry) open(at string) decision {
	if e.ifNotExists {
		return BranchPlan{Branch: e.name, Path: at, Action: ActionUnchanged}
	}
	return refusal{
		Code:    "create.checked_out_elsewhere",
		Message: "branch " + e.name + " is checked out at " + at + " — run `wtm go " + e.name + "` to jump in",
		Params:  wtm.Params{"branch": e.name, "path": at},
	}
}

func (e entry) occupied(occupant string) decision {
	path := e.path
	switch {
	case e.ifNotExists && occupant == e.name:
		return BranchPlan{Branch: e.name, Path: path, Action: ActionUnchanged}
	case occupant == "" || occupant == e.name:
		return refusal{Code: "create.path_exists", Message: "worktree path already exists: " + path, Params: wtm.Params{"path": path}}
	}
	return e.clashed(branch.Clash{Branch: occupant, Name: filepath.Base(path)})
}

func (e entry) clashed(clash branch.Clash) refusal {
	name := e.name
	return refusal{
		Code:    "create.name_clash",
		Message: "branch " + name + " would share its name with " + clash.Branch + " (" + clash.Name + ") — pick another name",
		Params:  wtm.Params{"branch": name, "other": clash.Branch, "name": clash.Name},
	}
}
