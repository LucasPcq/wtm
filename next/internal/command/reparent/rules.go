package reparent

import (
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what reparent decides from, as Observe read them: the repository (its config and snapshot), the meta record of each worktree on a branch and the error of one that exists but does not read, the stack they make, and the worktree root the caller stands in, as git spells it.
type Facts struct {
	worktree.Repository
	Records    map[string]worktree.Meta
	RecordErrs map[string]error
	Stack      worktree.Stack
	Cwd        string
}

// named returns the worktree whose name is name, its branch or, on no branch, its path, and whether there is one.
func (f Facts) named(name string) (worktree.Worktree, bool) {
	at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Name() == name })
	if at < 0 {
		return worktree.Worktree{}, false
	}

	return f.Worktrees[at], true
}

// movable reports whether reparent can move w: a worktree wtm recorded, on a branch, neither the main one nor the base branch's.
func (f Facts) movable(w worktree.Worktree) bool {
	_, recorded := f.Records[w.Branch()]
	return !w.Main && w.Branch() != "" && w.Branch() != f.Config.Worktrees.BaseBranch && recorded
}

// gone reports whether parent, a branch a worktree records as its parent, is no branch any more.
func (f Facts) gone(parent string) bool {
	return parent != "" && !f.Candidate(parent)
}

// current is the branch of the worktree the caller stands in when reparent can move it, "" otherwise.
func (f Facts) current() string {
	at := worktree.Holding(f.Worktrees, f.Cwd, func(w worktree.Worktree) string { return w.Path })
	if at < 0 || !f.movable(f.Worktrees[at]) {
		return ""
	}

	return f.Worktrees[at].Branch()
}

// ----------------------------------------------------------------------------
// The decision of each worktree

// A Move is one worktree moved onto a new parent: its branch and path, the parent it leaves (empty when it had none) and whether that parent is no branch any more, the one it goes to, and its children, which follow it.
type Move struct {
	Branch     string   `json:"branch"`
	Path       string   `json:"path"`
	From       string   `json:"from"`
	ParentGone bool     `json:"parent_gone"`
	To         string   `json:"to"`
	Children   []string `json:"children,omitempty"`
}

// Unchanged reports whether the worktree already stands on the parent it moves to.
func (m Move) Unchanged() bool {
	return m.From == m.To
}

// The codes of a worktree reparent cannot move: the main worktree, the base branch's, one on no branch, one whose record does not read, one wtm did not create; and of a new parent under a worktree it moves.
const (
	CodeMainWorktree   = Name + ".main_worktree"
	CodeBaseBranch     = Name + ".base_branch"
	CodeDetached       = Name + ".detached"
	CodeMetaUnreadable = Name + domain.MetaUnreadable
	CodeUnrecorded     = Name + ".unrecorded"
	CodeCycle          = Name + ".cycle"
)

// entries decides each worktree names asks to move onto to: its move, or the problem of its entry: blank, given twice, no worktree, or a worktree reparent cannot move.
func (f Facts) entries(names []string, to string) (moves []Move, problems []domain.FieldError) {
	for index, name := range names {
		path := domain.PathWorktree + "[" + strconv.Itoa(index) + "]"
		problem := func(code, template string) domain.FieldError {
			return domain.FieldError{Path: path, Code: code, Template: template, Params: domain.Params{"value": name}}
		}
		w, found := f.named(name)
		switch {
		case strings.TrimSpace(name) == "":
			problems = append(problems, domain.Required(path))
		case slices.Contains(names[:index], name):
			problems = append(problems, problem(domain.CodeDistinct, "{path}: {value} is given twice"))
		case !found:
			problems = append(problems, problem(domain.CodeNotFound, "{value} names no worktree"))
		case w.Main:
			problems = append(problems, problem(CodeMainWorktree, "{value} is the main worktree: it has no parent"))
		case w.Branch() == "":
			problems = append(problems, problem(CodeDetached, "{value} is on no branch: only a branch has a parent"))
		case w.Branch() == f.Config.Worktrees.BaseBranch:
			problems = append(problems, problem(CodeBaseBranch, "{value} is the base branch: sync never rebases it"))
		case f.RecordErrs[w.Branch()] != nil:
			unread := problem(CodeMetaUnreadable, "couldn't read the record of {value}: {cause}")
			unread.Params["cause"] = f.RecordErrs[w.Branch()].Error()
			problems = append(problems, unread)
		case !f.movable(w):
			problems = append(problems, problem(CodeUnrecorded, "{value} has no wtm record: wtm did not create it"))
		default:
			from := f.Records[w.Branch()].SourceBranch
			moves = append(moves, Move{Branch: w.Branch(), Path: w.Path, From: from, ParentGone: f.gone(from), To: to, Children: f.Stack.Children(w.Branch())})
		}
	}

	return moves, problems
}

// problemOf is the problem of to as the new parent of the worktrees moving, nil when it has none or is not given: a branch that does not exist, one of them, or one under them, which would close a cycle.
func (f Facts) problemOf(to string, moving []string) *domain.FieldError {
	problem := domain.FieldError{Path: domain.PathTo, Params: domain.Params{"value": to}}
	switch {
	case to == "":
		return nil
	case !f.Candidate(to):
		problem.Code, problem.Template = domain.CodeNotFound, "{value} names no branch: pass a local branch or origin/<branch> to {path}"
	case slices.Contains(moving, to):
		problem.Code, problem.Template, problem.With = domain.CodeSelfParent, "{value} cannot be its own parent: pass another value to {path}", []string{domain.PathWorktree}
	default:
		under := f.Stack.Within(to, moving)
		if under == "" {
			return nil
		}
		problem.Code, problem.Template = CodeCycle, "{value} sits under {under}: moving {under} onto it would close a cycle"
		problem.Params["under"] = under
	}

	return &problem
}

func branches(moves []Move) []string {
	found := make([]string, 0, len(moves))
	for _, move := range moves {
		found = append(found, move.Branch)
	}

	return found
}

// ----------------------------------------------------------------------------
// Decide

// Decide turns the facts and the resolved request into the plan: the move of each worktree, those already on the new parent included.
func Decide(f Facts, req Request) Plan {
	moves, _ := f.entries(req.Worktrees, req.To)
	return Plan{Moves: moves, To: req.To, chain: append([]string{req.To}, f.Stack.Ancestors(req.To)...), facts: f}
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: the move of each worktree, those already on the new parent included, and the new parent; then the recap, which the fingerprint leaves out.
type Plan struct {
	Moves []Move `json:"moves"`
	To    string `json:"to"`
	domain.Recapped
	chain []string // the new parent and the branches it sits under
	facts Facts
}

// Locks are the locks the plan needs: the repository shared, each worktree it moves exclusive, and the new parent and each branch it sits under shared, so that no other operation moves or removes one of them under the move.
func (p Plan) Locks() []domain.Lock {
	locks := domain.WorktreeLocks(branches(p.Moves))
	for _, name := range p.chain {
		locks = append(locks, domain.Lock{Scope: domain.ScopeWorktree, Name: name, Mode: domain.LockShared})
	}

	return locks
}

// Fingerprinted is what the plan's fingerprint covers: the plan, its recap left out.
func (p Plan) Fingerprinted() any {
	p.Recapped = domain.Recapped{}
	return p
}

// Locks are the locks taken before planning: the repository shared, and each worktree asked exclusive.
func (r Request) Locks() []domain.Lock {
	return domain.WorktreeLocks(slices.DeleteFunc(slices.Clone(r.Worktrees), func(name string) bool { return strings.TrimSpace(name) == "" }))
}

// ----------------------------------------------------------------------------
// The recap

// NoteUnchanged notes a recap's worktree already on the new parent.
const NoteUnchanged = "unchanged"

// empty reports whether applying the plan changes nothing: every worktree already stands on the new parent.
func (p Plan) empty() bool {
	return !slices.ContainsFunc(p.Moves, func(m Move) bool { return !m.Unchanged() })
}

// recap is the plan row by row: each worktree with the parent it leaves and the one it goes to, then the new parent.
func (p Plan) recap() []domain.RecapRow {
	values := make([]domain.RecapValue, 0, len(p.Moves))
	for _, move := range p.Moves {
		value := domain.RecapValue{Value: move.Branch, Params: domain.Params{"from": move.From, "to": move.To}, Text: move.Branch + " (" + parentOf(move) + " → " + move.To + ")"}
		if move.Unchanged() {
			value.Note, value.Text = NoteUnchanged, move.Branch+" (already on "+move.To+")"
		}
		values = append(values, value)
	}

	return []domain.RecapRow{
		{Key: "worktrees", Label: "Worktrees", Path: domain.PathWorktree, Values: values},
		{Key: "to", Label: "New parent", Path: domain.PathTo, Values: domain.RecapValues([]string{p.To})},
	}
}

// parentOf words the parent move leaves in the recap.
func parentOf(move Move) string {
	switch {
	case move.From == "":
		return "no parent"
	case move.ParentGone:
		return move.From + " (gone)"
	}

	return move.From
}

// ----------------------------------------------------------------------------
// The outcome

// CodeSync is the step that rebases the worktrees moved onto their new parent; CodeUndo the one that moves a worktree back onto the parent it left, offered while that parent is a branch the worktree does not sit above.
const (
	CodeSync = Name + ".sync"
	CodeUndo = Name + ".undo"
)

// Undo is the follow-up that moves branch back onto from, the parent it left.
func Undo(branch, from string) domain.FollowUp {
	return domain.FollowUp{Command: Name, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{branch}}, {Path: domain.PathTo, Values: []string{from}}}, Code: CodeUndo, Message: "move " + branch + " back onto " + from}
}

// undoable reports whether branch can move back onto from: a branch still, and not under branch, as the parent of a cycle it was moved out of is.
func (p Plan) undoable(branch, from string) bool {
	return p.facts.Candidate(from) && p.facts.Stack.Within(from, []string{branch}) == ""
}

// commandSync rebases a worktree onto its parent: not migrated yet.
const commandSync = "sync"

// next are the steps after the moves: the sync of every worktree moved, then the undo of each.
func (p Plan) next(items []Item) []domain.FollowUp {
	var moved []string
	var undos []domain.FollowUp
	for _, item := range items {
		if item.Status != domain.StatusDone || item.Reason == domain.ReasonUnchanged {
			continue
		}
		moved = append(moved, item.Subject)
		if from := item.Detail.From; p.undoable(item.Subject, from) {
			undos = append(undos, Undo(item.Subject, from))
		}
	}
	if len(moved) == 0 {
		return nil
	}

	sync := domain.FollowUp{Command: commandSync, Answers: []domain.Answered{{Path: domain.PathWorktree, Values: moved}}, Code: CodeSync, Message: "rebase onto the new parent"}
	return append([]domain.FollowUp{sync}, undos...)
}
