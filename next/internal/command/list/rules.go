package list

import (
	"cmp"
	"slices"
	"strconv"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/repository"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what list reads: the repository, why fetching origin failed when it was asked to, the meta record of each live branch (and the error of a record that exists but does not read), the stack they make, when each branch last moved (and why it could not be read), what each worktree holds, what the run daemon and GitHub said, and the branches behind their merged pull request (repository.Behind).
type Facts struct {
	worktree.Repository
	repository.Asked
	FetchErr   error
	DatesErr   error                    // why when each branch last moved could not be read: none is told
	Records    map[string]worktree.Meta // by branch
	RecordErrs map[string]error         // by branch
	Stack      worktree.Stack
	Dates      map[string]time.Time // by branch
	States     map[string]State     // by worktree path
	Behind     map[string]bool      // by branch
}

// A State is what one worktree holds: whether its directory is gone, its uncommitted changes (read only when it has some), the commits its branch has that the base branch lacks, those it has that its parent lacks and the reverse (nil when not counted), and the error that kept it from being read.
type State struct {
	Missing      bool
	Dirty        bool
	Changes      []worktree.Change
	Ahead        int
	ParentAhead  *int
	ParentBehind *int
	Err          error
}

// ----------------------------------------------------------------------------
// The detail

// A Detail is one worktree as list reports it.
type Detail struct {
	Branch       string `json:"branch"`
	Path         string `json:"path"`
	Main         bool   `json:"main"`
	Dirty        bool   `json:"dirty"`
	Locked       bool   `json:"locked"`
	Rebasing     bool   `json:"rebasing"`
	Active       bool   `json:"active"`        // the caller stands in it, the deepest worktree holding its directory
	Missing      bool   `json:"missing"`       // its directory is gone: git calls it prunable
	CommitsAhead int    `json:"commits_ahead"` // the commits it has that the base branch lacks
	Parent       string `json:"parent"`        // the branch it was created from, empty for a root
	// Ahead are the commits it has that its parent lacks, the base branch's for a root; Behind those of its parent it lacks, null for a root. Both null when not counted: the main worktree, a member of a cycle, a parent that is not a local branch.
	Ahead        *int             `json:"ahead"`
	Behind       *int             `json:"behind"`
	NeedsSync    bool             `json:"needs_sync"`     // its parent has commits it lacks: a rebase is due
	Changes      *Changes         `json:"changes"`        // null when its directory is gone
	CreatedAt    *time.Time       `json:"created_at"`     // from its meta record, null without one
	LastCommitAt *time.Time       `json:"last_commit_at"` // when the commit its branch stands at was made, null on no branch
	Origin       *worktree.Origin `json:"origin"`         // null when origin has no branch of that name
	PR           *PR              `json:"pr"`             // null without one, or when GitHub was not asked
	Merged       bool             `json:"merged"`         // its pull request merged it and it has not moved since: cleaning it is due; false when GitHub was not asked
	Services     []string         `json:"services"`       // the jobs up in it, null when that cannot be told
	URLs         []string         `json:"urls"`           // the addresses of the jobs up in it, null when that cannot be told
	// Actions are what may be done next with it, as Offers lists them, each the command that does it and its answers.
	Actions []domain.FollowUp `json:"actions"`
}

// Changes counts a worktree's uncommitted files: those git tracks that changed, and the untracked.
type Changes struct {
	Changed   int `json:"changed"`
	Untracked int `json:"untracked"`
}

// PR is a worktree's newest pull request and, for an open one only, where its review stands.
type PR struct {
	Number int                       `json:"number"`
	URL    string                    `json:"url"`
	State  worktree.PullRequestState `json:"state"`
	*worktree.Progress
}

// The codes of list's warnings and failures.
const (
	CodePRsUnavailable  = Name + domain.PRsUnavailable
	CodeJobsUnknown     = Name + domain.JobsUnknown
	CodeBaseMissing     = Name + ".base_missing"
	CodeFetchFailed     = Name + ".fetch_failed"
	CodeDatesUnreadable = Name + ".dates_unreadable"
	CodeMetaUnreadable  = Name + domain.MetaUnreadable
	CodeUnreadable      = Name + domain.Unreadable
)

// ----------------------------------------------------------------------------
// List

// List is each worktree of facts as an item, the main one first, then the others by creation, the oldest first and those with no record last, the one caller stands in active; and the warnings of what could not be read.
func List(facts Facts, caller domain.Caller) ([]Item, []domain.Warning) {
	worktrees := facts.Checkouts()
	slices.SortStableFunc(worktrees, func(a, b worktree.Worktree) int {
		if a.Main != b.Main {
			return compareTrue(a.Main, b.Main)
		}
		return compareCreated(facts.Records[a.Branch()], facts.Records[b.Branch()])
	})

	active := worktree.Holding(worktrees, caller.Cwd, func(w worktree.Worktree) string { return w.Path })
	items := make([]Item, 0, len(worktrees))
	for i, w := range worktrees {
		items = append(items, facts.item(w, i == active))
	}

	return items, facts.warnings()
}

func (f Facts) item(w worktree.Worktree, active bool) Item {
	state := f.States[w.Path]
	if state.Err != nil {
		item := domain.Unread[Detail](domain.UnreadParams{Command: Name, Subject: w.Name(), Path: w.Path, Err: state.Err})
		item.Warnings = f.recordWarnings(w.Branch())
		return item
	}

	_, rebasing := w.State.(worktree.Rebasing)
	parent, _ := f.Stack.Parent(w.Branch())
	pr, hasPR := worktree.PullRequestOf(f.PullRequests, w.Branch())
	detail := Detail{
		Branch:       w.Branch(),
		Path:         w.Path,
		Main:         w.Main,
		Dirty:        state.Dirty,
		Locked:       w.Locked,
		Rebasing:     rebasing,
		Active:       active,
		Missing:      state.Missing,
		CommitsAhead: state.Ahead,
		Parent:       parent,
		Ahead:        state.ParentAhead,
		Behind:       state.ParentBehind,
		NeedsSync:    state.ParentBehind != nil && *state.ParentBehind > 0,
		Changes:      changesOf(state),
		CreatedAt:    f.Records[w.Branch()].Created(),
		LastCommitAt: f.lastCommit(w.Branch()),
		Origin:       f.OriginOf(w.Branch()),
		PR:           prOf(pr, hasPR),
		Merged:       !state.Missing && f.Cleans(pr, w, f.Behind[w.Branch()]),
		Services:     f.Services(w.Path),
		URLs:         f.URLs(w.Path),
	}
	detail.Actions = Actions(detail)

	return Item{Subject: w.Name(), Status: domain.StatusDone, Detail: &detail, Warnings: f.recordWarnings(w.Branch())}
}

// recordWarnings are the warnings of the record of branch when it exists but does not read, none otherwise.
func (f Facts) recordWarnings(branch string) []domain.Warning {
	err := f.RecordErrs[branch]
	if err == nil {
		return nil
	}

	return []domain.Warning{{Code: CodeMetaUnreadable, Message: "couldn't read the record of " + branch + ": when it was created is not known: " + err.Error(), Params: domain.Params{"branch": branch}}}
}

func prOf(pr worktree.PullRequest, ok bool) *PR {
	if !ok {
		return nil
	}

	return &PR{Number: pr.Number, URL: pr.URL, State: pr.State, Progress: pr.Progress()}
}

func changesOf(state State) *Changes {
	if state.Missing {
		return nil
	}

	changes := &Changes{}
	for _, change := range state.Changes {
		if change.Status == worktree.ChangeUntracked {
			changes.Untracked++
			continue
		}
		changes.Changed++
	}
	return changes
}

func (f Facts) lastCommit(branch string) *time.Time {
	at, ok := f.Dates[branch]
	if !ok {
		return nil
	}

	return &at
}

func (f Facts) warnings() []domain.Warning {
	var warnings []domain.Warning
	if base, missing := f.MissingBase(); missing {
		warnings = append(warnings, domain.Warning{Code: CodeBaseMissing, Message: "the base branch " + base + " does not exist: commits ahead of it are not counted", Params: domain.Params{"branch": base}})
	}
	if f.DatesErr != nil {
		warnings = append(warnings, domain.Warning{Code: CodeDatesUnreadable, Message: "couldn't read when each branch last moved: " + f.DatesErr.Error()})
	}
	if f.FetchErr != nil && !f.Interrupted {
		warnings = append(warnings, domain.Warning{Code: CodeFetchFailed, Message: "couldn't fetch origin: where each branch stands against it is as last fetched: " + f.FetchErr.Error()})
	}

	return append(warnings, f.Asked.Warnings(Name)...)
}

// ----------------------------------------------------------------------------
// The actions

// An Action is what the user may do next with a worktree picked from the list.
type Action string

// The actions on a worktree, in the order the picker offers them.
const (
	ActionGo      Action = "go"
	ActionOpenPR  Action = "open_pr"
	ActionRunUp   Action = "run_up"
	ActionRunDown Action = "run_down"
	ActionLogs    Action = "logs"
	ActionClean   Action = "clean"
)

// Offers are the actions a worktree offers: only clean when its directory is gone, none when it is on no branch too (clean takes a branch), only go when it is on no branch, open its pull request when it has one.
func Offers(detail Detail) []Action {
	switch {
	case detail.Missing && detail.Branch == "":
		return nil
	case detail.Missing:
		return []Action{ActionClean}
	case detail.Branch == "":
		return []Action{ActionGo}
	}

	actions := []Action{ActionGo}
	if detail.PR != nil {
		actions = append(actions, ActionOpenPR)
	}
	actions = append(actions, ActionRunUp, ActionRunDown, ActionLogs)
	if !detail.Main {
		actions = append(actions, ActionClean)
	}

	return actions
}

// ParamNumber is the param of the action that opens a pull request: its number, beside its URL.
const ParamNumber = "number"

// byAction is what each action runs, the wtm command as its command line spells it after the program's name, none to open a pull request, and what it reads as in the picker and as the note of its follow-up.
var byAction = map[Action]struct{ command, label string }{
	ActionGo:      {domain.CommandGo, "Go to the worktree"},
	ActionOpenPR:  {"", "Open the pull request"},
	ActionRunUp:   {"run up", "Start the profile"},
	ActionRunDown: {"run down", "Stop the profile"},
	ActionLogs:    {"run logs", "View the logs"},
	ActionClean:   {domain.CommandClean, "Clean (delete the worktree)"},
}

// Actions are the follow-ups the worktree of detail offers, as Offers lists them.
func Actions(detail Detail) []domain.FollowUp {
	actions := []domain.FollowUp{}
	for _, action := range Offers(detail) {
		actions = append(actions, followUpOf(action, detail))
	}

	return actions
}

// followUpOf is action on the worktree of detail as the follow-up that carries it out, with its code and its label: its wtm command and answers, or the URL of its pull request and its number.
func followUpOf(action Action, detail Detail) domain.FollowUp {
	followUp := domain.FollowUp{Code: Name + "." + string(action), Message: byAction[action].label}
	if action == ActionOpenPR && detail.PR != nil {
		followUp.URL, followUp.Params = detail.PR.URL, domain.Params{ParamNumber: strconv.Itoa(detail.PR.Number)}
		return followUp
	}

	followUp.Command, followUp.Answers = byAction[action].command, []domain.Answered{{Path: domain.PathWorktree, Values: []string{cmp.Or(detail.Branch, detail.Path)}}}
	return followUp
}

// WithPullRequests is shown with the pull request of each worktree as from read it, whether it merged it, what each offers then, and the warnings of asking GitHub: what a read that did not ask for them becomes once they land.
func WithPullRequests(shown, from Outcome) Outcome {
	merged := shown
	merged.Interrupted = shown.Interrupted || from.Interrupted
	merged.Items = slices.Clone(shown.Items)
	for i, item := range merged.Items {
		at := slices.IndexFunc(from.Items, func(other Item) bool { return other.Subject == item.Subject && other.Detail != nil })
		if item.Detail == nil || at < 0 {
			continue
		}
		detail := *item.Detail
		detail.PR, detail.Merged = from.Items[at].Detail.PR, from.Items[at].Detail.Merged
		detail.Actions = Actions(detail)
		merged.Items[i].Detail = &detail
	}

	merged.Warnings = slices.Clone(shown.Warnings)
	for _, warning := range from.Warnings {
		if warning.Code == CodePRsUnavailable {
			merged.Warnings = append(merged.Warnings, warning)
		}
	}

	return merged
}

// ----------------------------------------------------------------------------
// Helpers

// compareTrue orders true before false.
func compareTrue(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return -1
	}
	return 1
}

// compareCreated orders two meta records by their created_at, oldest first, one that does not parse last.
func compareCreated(a, b worktree.Meta) int {
	at, bt := a.Created(), b.Created()
	if at == nil || bt == nil {
		return compareTrue(at != nil, bt != nil)
	}

	return at.Compare(*bt)
}
