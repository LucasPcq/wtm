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
	Missing   bool `json:"missing"` // its directory is gone
	// Condition is the one fact of it that most changes what to do first, as this read knows it.
	Condition  worktree.Condition `json:"condition"`
	Cycle      bool               `json:"cycle"`       // its parents lead back to it: drawn as a root
	ParentGone bool               `json:"parent_gone"` // its parent is no branch any more, local or on origin: sync has nothing to rebase it onto
	Origin     *worktree.Origin   `json:"origin"`      // null when origin has no branch of that name
	PR         *PR                `json:"pr"`          // null without one, or when GitHub was not asked
	Services   []string           `json:"services"`    // the jobs up in it, null when that cannot be told or it has no worktree
	// Actions are every action of its worktree, each the follow-up that carries it out or why it is not offered; none for a virtual root or an unreadable worktree.
	Actions []repository.Offer `json:"actions"`
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
	detail := Detail{Branch: branch, Children: append([]string{}, f.Stack.Children(branch)...), Actions: []repository.Offer{}}
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

	detail.Dirty, detail.Missing = state.Dirty, state.Missing
	detail.Ahead, detail.Behind = state.Ahead, state.Behind
	detail.NeedsSync = w.NeedsSync(state.Behind)
	detail.Cycle = f.Stack.InCycle(branch)
	detail.ParentGone = detail.Parent != nil && !f.Candidate(*detail.Parent)
	detail.Origin = f.OriginOf(branch)
	pr, ok := worktree.PullRequestOf(f.PullRequests, branch)
	if ok {
		detail.PR = &PR{Number: pr.Number, URL: pr.URL, State: pr.State}
	}
	detail.Services = f.Services(w.Path)
	cleans := f.Repository.Cleans(pr, w, f.Behind[branch])
	detail.Condition = worktree.ConditionOf(worktree.ConditionParams{Missing: detail.Missing, Operation: w.Operation(), Merged: cleans, Dirty: detail.Dirty})
	_, recorded := f.Records[branch]
	detail.Actions = Actions(ActionsParams{Detail: detail, Base: branch == f.Config.Worktrees.BaseBranch, Recorded: recorded || f.RecordErrs[branch] != nil})

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

// ActionsParams are a node of a worktree, whether its branch is the base branch, and whether wtm recorded it (its record read or not).
type ActionsParams struct {
	Detail   Detail
	Base     bool
	Recorded bool
}

// Actions are every action of the worktree of p, each with its follow-up or why it does not offer it.
func Actions(p ActionsParams) []repository.Offer {
	detail := p.Detail
	params := repository.OffersParams{
		Command:  Name,
		Branch:   detail.Branch,
		Path:     *detail.Path,
		Main:     detail.Main,
		Base:     p.Base,
		Recorded: p.Recorded,
		Missing:  detail.Missing,
		OnOrigin: detail.Origin != nil,
	}
	if detail.PR != nil {
		params.PRNumber, params.PRURL = detail.PR.Number, detail.PR.URL
	}

	return repository.Offers(params)
}
