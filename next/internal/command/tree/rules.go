package tree

import (
	"slices"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what tree reads: the repository, the meta record of each live branch (and the error of a record that exists but does not read), the stack they make, what each worktree holds, what the run daemon and GitHub said, and the branches behind their merged pull request (repository.Behind).
type Facts struct {
	worktree.Repository
	repository.Asked
	Records    map[string]worktree.Meta // by branch
	RecordErrs map[string]error         // by branch
	Stack      worktree.Stack
	States     map[string]State // by branch
	Behind     map[string]bool  // by branch
}

// A State is what one worktree holds: whether its directory is gone, whether it has uncommitted changes, the commits it has that its parent lacks and the reverse, nil when they are not counted, and the error that kept it from being read.
type State struct {
	Missing bool
	Dirty   bool
	Ahead   *int
	Behind  *int
	Err     error
}

// ----------------------------------------------------------------------------
// The detail

// A Detail is one node of the tree as tree reports it: a worktree, or a virtual root standing for a parent with no worktree.
type Detail struct {
	Branch   string   `json:"branch"`
	Path     *string  `json:"path"` // null for a virtual root
	Main     bool     `json:"main"`
	Virtual  bool     `json:"virtual"`
	Parent   *string  `json:"parent"` // null for a root
	Children []string `json:"children"`
	// Ahead are the commits the branch has that its parent lacks, the base branch's for a root; Behind those of the parent it lacks, null for a root, which has none. Both null when not counted: the main worktree, a member of a cycle, a parent that is not a local branch.
	Ahead     *int `json:"ahead"`
	Behind    *int `json:"behind"`
	NeedsSync bool `json:"needs_sync"` // its parent has commits it lacks and no rebase, bisect or merge is under way: a sync is due
	Dirty     bool `json:"dirty"`
	Locked    bool `json:"locked"`
	Rebasing  bool `json:"rebasing"`
	Missing   bool `json:"missing"` // its directory is gone
	// Condition is the one fact of it that most changes what to do first, as this read knows it.
	Condition worktree.Condition `json:"condition"`
	Cycle     bool               `json:"cycle"`    // its parents lead back to it: drawn as a root
	Origin    *worktree.Origin   `json:"origin"`   // null when origin has no branch of that name
	PR        *PR                `json:"pr"`       // null without one, or when GitHub was not asked
	Services  []string           `json:"services"` // the jobs up in it, null when that cannot be told or it has no worktree
	// Actions are what the node needs next, as Actions decides it: a command and its answers.
	Actions []domain.FollowUp `json:"actions"`
}

// PR is a branch's newest pull request.
type PR struct {
	Number int                       `json:"number"`
	URL    string                    `json:"url"`
	State  worktree.PullRequestState `json:"state"`
}

// The codes of tree's warnings and failures.
const (
	CodePRsUnavailable = Name + domain.PRsUnavailable
	CodeJobsUnknown    = Name + domain.JobsUnknown
	CodeBaseMissing    = Name + ".base_missing"
	CodeMetaUnreadable = Name + domain.MetaUnreadable
	CodeUnreadable     = Name + domain.Unreadable
)

// ----------------------------------------------------------------------------
// Tree

// Tree is each node of the stack of facts as an item, depth first: the roots as the stack orders them (the main worktree, the others by creation, the virtual ones by name), each followed by its children by creation; and the warnings of what could not be read.
func Tree(facts Facts) ([]Item, []domain.Warning) {
	var items []Item
	var walk func(branch string)
	walk = func(branch string) {
		items = append(items, facts.item(branch))
		for _, child := range facts.Stack.Children(branch) {
			walk(child)
		}
	}
	for _, root := range facts.Stack.Roots() {
		walk(root)
	}

	return items, facts.warnings()
}

// item is the node of branch: where it sits in the tree, then, for a worktree read, what it holds; a worktree that cannot be read keeps its place and fails alone.
func (f Facts) item(branch string) Item {
	detail := Detail{Branch: branch, Children: append([]string{}, f.Stack.Children(branch)...), Actions: []domain.FollowUp{}}
	if parent, ok := f.Stack.Parent(branch); ok {
		detail.Parent = &parent
	}
	if f.Stack.Virtual(branch) {
		detail.Virtual, detail.Condition = true, worktree.ConditionNoWorktree
		return Item{Subject: branch, Status: domain.StatusDone, Detail: &detail}
	}

	w := f.worktreeOf(branch)
	state := f.States[branch]
	detail.Path, detail.Main, detail.Locked = &w.Path, w.Main, w.Locked
	if state.Err != nil {
		detail.Condition = worktree.ConditionUnreadable
		item := domain.Unread[Detail](domain.UnreadParams{Command: Name, Subject: branch, Path: w.Path, Err: state.Err})
		item.Detail, item.Warnings = &detail, f.recordWarnings(branch)
		return item
	}

	_, detail.Rebasing = w.State.(worktree.Rebasing)
	detail.Dirty, detail.Missing = state.Dirty, state.Missing
	detail.Ahead, detail.Behind = state.Ahead, state.Behind
	detail.NeedsSync = w.NeedsSync(state.Behind)
	detail.Cycle = f.Stack.InCycle(branch)
	detail.Origin = f.OriginOf(branch)
	pr, ok := worktree.PullRequestOf(f.PullRequests, branch)
	if ok {
		detail.PR = &PR{Number: pr.Number, URL: pr.URL, State: pr.State}
	}
	detail.Services = f.Services(w.Path)
	cleans := f.Repository.Cleans(pr, w, f.Behind[branch])
	detail.Condition = worktree.ConditionOf(worktree.ConditionParams{Missing: detail.Missing, Operation: w.Operation(), Merged: cleans, Dirty: detail.Dirty})
	detail.Actions = Actions(detail, cleans)

	return Item{Subject: branch, Status: domain.StatusDone, Detail: &detail, Warnings: f.recordWarnings(branch)}
}

// recordWarnings are the warnings of the record of branch when it exists but does not read, none otherwise.
func (f Facts) recordWarnings(branch string) []domain.Warning {
	err := f.RecordErrs[branch]
	if err == nil {
		return nil
	}

	return []domain.Warning{{Code: CodeMetaUnreadable, Message: "couldn't read the record of " + branch + ": its parent is not known, so it stands as a root: " + err.Error(), Params: domain.Params{"branch": branch}}}
}

func (f Facts) warnings() []domain.Warning {
	var warnings []domain.Warning
	if base, missing := f.MissingBase(); missing {
		warnings = append(warnings, domain.Warning{Code: CodeBaseMissing, Message: "the base branch " + base + " does not exist: the roots are not counted against it", Params: domain.Params{"branch": base}})
	}

	return append(warnings, f.Asked.Warnings(Name)...)
}

// nodes are the worktrees that are nodes of the tree: those on a branch, the only place a stack has for them.
func (f Facts) nodes() []worktree.Worktree {
	return slices.DeleteFunc(slices.Clone(f.Worktrees), func(w worktree.Worktree) bool { return w.Branch() == "" })
}

// worktreeOf is the worktree on branch, a node of the tree.
func (f Facts) worktreeOf(branch string) worktree.Worktree {
	at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Branch() == branch })
	return f.Worktrees[at]
}

// ----------------------------------------------------------------------------
// The actions

// finishing names each operation git stops in the middle of as the action that sends a worktree to finish it says it.
var finishing = map[worktree.Operation]string{
	worktree.OperationRebase:     "rebase",
	worktree.OperationBisect:     "bisect",
	worktree.OperationMerge:      "merge",
	worktree.OperationCherryPick: "cherry-pick",
	worktree.OperationRevert:     "revert",
	worktree.OperationApply:      "git am",
}

// commandSync is the command that rebases a worktree onto its parent, answered by the worktree.
const commandSync = "sync"

// Actions is what the worktree of detail needs next, the first that applies: an operation git stopped in the middle of (a rebase, a bisect, a merge…) is finished in it, one its merged pull request cleans (worktree.Repository.Cleans) is cleaned, one behind its parent is synced onto it; none otherwise.
func Actions(detail Detail, cleans bool) []domain.FollowUp {
	var command, message string
	switch {
	case detail.Condition.Operation() != "":
		command, message = domain.CommandGo, "Go to the worktree to finish its "+finishing[detail.Condition.Operation()]
	case cleans:
		command, message = domain.CommandClean, "Clean the worktree: its pull request is merged"
	case detail.NeedsSync:
		command, message = commandSync, "Sync the worktree: its parent has commits it lacks"
	default:
		return []domain.FollowUp{}
	}

	return []domain.FollowUp{{
		Command: command,
		Answers: []domain.Answered{{Path: domain.PathWorktree, Values: []string{detail.Branch}}},
		Code:    Name + "." + command,
		Message: message,
	}}
}
