package list

import (
	"slices"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
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
	Active       bool   `json:"active"`        // the caller stands in it, the deepest worktree holding its directory
	Missing      bool   `json:"missing"`       // its directory is gone: git calls it prunable
	CommitsAhead int    `json:"commits_ahead"` // the commits it has that the base branch lacks
	Parent       string `json:"parent"`        // the branch it was created from, empty for a root
	// Ahead are the commits it has that its parent lacks, the base branch's for a root; Behind those of its parent it lacks, null for a root. Both null when not counted: the main worktree, a member of a cycle, a parent that is not a local branch.
	Ahead        *int             `json:"ahead"`
	Behind       *int             `json:"behind"`
	NeedsSync    bool             `json:"needs_sync"`     // its parent has commits it lacks and no rebase, bisect or merge is under way: a sync is due
	Changes      *Changes         `json:"changes"`        // null when its directory is gone
	CreatedAt    *time.Time       `json:"created_at"`     // from its meta record, null without one
	LastCommitAt *time.Time       `json:"last_commit_at"` // when the commit its branch stands at was made, null on no branch
	Origin       *worktree.Origin `json:"origin"`         // null when origin has no branch of that name
	PR           *PR              `json:"pr"`             // null without one, or when GitHub was not asked
	Merged       bool             `json:"merged"`         // its pull request merged it and it has not moved since: cleaning it is due; false when GitHub was not asked
	// Finished is how its pull request has done with it, merged or closed while the branch has not moved since (worktree.Repository.Finished), so prune removes it, its directory gone or not (prune skips its hooks then); null when it has not, or when GitHub was not asked.
	Finished  *Finished          `json:"finished"`
	Condition worktree.Condition `json:"condition"` // the one fact of it that most changes what to do first, as this read knows it
	Services  []string           `json:"services"`  // the jobs up in it, null when that cannot be told
	URLs      []string           `json:"urls"`      // the addresses of the jobs up in it, null when that cannot be told
	Jobs      []Job              `json:"jobs"`      // every job the run daemon holds in it, by name, null when that cannot be told
	Profile   string             `json:"profile"`   // the profile the jobs it runs for itself were started under, empty when they say none
	// Actions are every action of it, each the follow-up that carries it out or why it is not offered.
	Actions []repository.Offer   `json:"actions"`
	Record  worktree.RecordState `json:"-"` // its meta record, as its actions say what follows from it

	base bool // it is on the base branch, which is never removed
}

// A Job is one job the run daemon holds in a worktree: what it is, its state, how the worktree holds it, the profile it was started under, its address, the code it crashed with and when it started.
type Job struct {
	Name      string          `json:"name"`
	Kind      project.JobKind `json:"kind"`
	State     run.State       `json:"state,omitempty"` // none for a claim: the job it holds runs in another worktree
	Hold      run.Hold        `json:"hold"`
	Profile   string          `json:"profile,omitempty"`
	URL       string          `json:"url,omitempty"`
	ExitCode  *int            `json:"exit_code,omitempty"`
	StartedAt *time.Time      `json:"started_at,omitempty"`
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

	parent, _ := f.Stack.Parent(w.Branch())
	pr, hasPR := worktree.PullRequestOf(f.PullRequests, w.Branch())
	detail := Detail{
		Branch:       w.Branch(),
		Path:         w.Path,
		Main:         w.Main,
		Dirty:        state.Dirty,
		Locked:       w.Locked,
		Active:       active,
		Missing:      state.Missing,
		CommitsAhead: state.Ahead,
		Parent:       parent,
		Ahead:        state.ParentAhead,
		Behind:       state.ParentBehind,
		NeedsSync:    w.NeedsSync(state.ParentBehind),
		Changes:      changesOf(state),
		CreatedAt:    f.Records[w.Branch()].Created(),
		LastCommitAt: f.lastCommit(w.Branch()),
		Origin:       f.OriginOf(w.Branch()),
		PR:           prOf(pr, hasPR),
		Merged:       !state.Missing && f.Cleans(pr, w, f.Behind[w.Branch()]),
		Finished:     f.finished(pr, w),
		Services:     f.Services(w.Path),
		URLs:         f.URLs(w.Path),
		Jobs:         f.jobs(w.Path),
		Profile:      run.ProfileIn(f.Asked.Jobs, w.Path),
		base:         w.Branch() != "" && w.Branch() == f.Config.Worktrees.BaseBranch,
		Record:       worktree.RecordOf(f.Records, f.RecordErrs, w.Branch()),
	}
	detail.Condition = conditionOf(detail, w.Operation())
	detail.Actions = Actions(detail)

	return Item{Subject: w.Name(), Status: domain.StatusDone, Detail: &detail, Warnings: f.recordWarnings(w.Branch())}
}

// jobs are the jobs the daemon holds in the worktree at path, by name, nil when it could not tell; an exit code is kept only for a crash, as a stop's signal leaves one that says nothing of the job.
func (f Facts) jobs(path string) []Job {
	if f.JobsErr != nil {
		return nil
	}

	jobs := []Job{}
	for _, job := range f.Asked.Jobs {
		if job.WorkDir != path {
			continue
		}
		held := Job{Name: job.Name, Kind: job.Kind, State: job.State, Hold: job.Hold, Profile: job.Profile, URL: job.URL}
		if job.State == run.Crashed {
			held.ExitCode = job.ExitCode
		}
		if !job.StartedAt.IsZero() {
			held.StartedAt = &job.StartedAt
		}
		jobs = append(jobs, held)
	}
	slices.SortFunc(jobs, func(a, b Job) int { return strings.Compare(a.Name, b.Name) })

	return jobs
}

// A Finished is how a worktree's pull request has done with it: merged, or closed unmerged.
type Finished string

// The ways a pull request has done with its worktree.
const (
	FinishedMerged Finished = "merged"
	FinishedClosed Finished = "closed"
)

// Finisheds returns every Finished.
func Finisheds() []Finished {
	return []Finished{FinishedMerged, FinishedClosed}
}

// finished is how pr has done with w, nil when it has not.
func (f Facts) finished(pr worktree.PullRequest, w worktree.Worktree) *Finished {
	if !f.Finished(pr, w, f.Behind[w.Branch()]) {
		return nil
	}

	finished := Finished(pr.State)
	return &finished
}

func conditionOf(detail Detail, operation worktree.Operation) worktree.Condition {
	return worktree.ConditionOf(worktree.ConditionParams{Missing: detail.Missing, Operation: operation, Merged: detail.Merged, Dirty: detail.Dirty})
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
	if f.DatesErr != nil && !f.Interrupted {
		warnings = append(warnings, domain.Warning{Code: CodeDatesUnreadable, Message: "couldn't read when each branch last moved: " + f.DatesErr.Error()})
	}
	if f.FetchErr != nil && !f.Interrupted {
		warnings = append(warnings, domain.Warning{Code: CodeFetchFailed, Message: "couldn't fetch origin: where each branch stands against it is as last fetched: " + f.FetchErr.Error()})
	}

	return append(warnings, f.Asked.Warnings(Name)...)
}

// ----------------------------------------------------------------------------
// The actions

// Actions are every action of the worktree of detail, each with its follow-up or why it does not offer it.
func Actions(detail Detail) []repository.Offer {
	params := repository.OffersParams{
		Command:  Name,
		Branch:   detail.Branch,
		Path:     detail.Path,
		Main:     detail.Main,
		Base:     detail.base,
		Record:   detail.Record,
		Missing:  detail.Missing,
		OnOrigin: detail.Origin != nil,
	}
	if detail.PR != nil {
		params.PRNumber, params.PRURL = detail.PR.Number, detail.PR.URL
	}

	return repository.Offers(params)
}

// Offered are the follow-ups of the actions the worktree offers, in order.
func (d Detail) Offered() []domain.FollowUp {
	return repository.Offered(d.Actions)
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
		detail.PR, detail.Merged, detail.Finished = from.Items[at].Detail.PR, from.Items[at].Detail.Merged, from.Items[at].Detail.Finished
		detail.Condition = conditionOf(detail, detail.Condition.Operation())
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
