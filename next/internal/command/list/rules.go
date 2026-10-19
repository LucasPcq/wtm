package list

import (
	"cmp"
	"context"
	"errors"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// ----------------------------------------------------------------------------
// The facts

// Facts are what list reads: the repository, what each worktree holds, the run daemon's jobs and the open pull requests, each with the error that kept it from being read; PullRequests is zero when GitHub was not asked.
type Facts struct {
	worktree.Repository
	States       map[string]State // by worktree path
	Jobs         []run.Job
	JobsErr      error
	PullRequests worktree.PullRequests
	PRsErr       error
}

// A State is what one worktree holds: whether its directory is gone, whether it has uncommitted changes, the commits its branch has that the base branch lacks, its meta record, and the error that kept it from being read.
type State struct {
	Missing bool
	Dirty   bool
	Ahead   int
	Meta    worktree.Meta
	Err     error
}

// ----------------------------------------------------------------------------
// The detail

// A Detail is one worktree as list reports it.
type Detail struct {
	Branch       string     `json:"branch"`
	Path         string     `json:"path"`
	Main         bool       `json:"main"`
	Dirty        bool       `json:"dirty"`
	Locked       bool       `json:"locked"`
	Rebasing     bool       `json:"rebasing"`
	Missing      bool       `json:"missing"` // its directory is gone: git calls it prunable
	CommitsAhead int        `json:"commits_ahead"`
	CreatedAt    *time.Time `json:"created_at"` // from its meta record, null without one
	Origin       *Origin    `json:"origin"`     // null when origin has no branch of that name
	PR           *PR        `json:"pr"`         // null without one, or when GitHub was not asked
	Services     []string   `json:"services"`   // the jobs up in it, null when that cannot be told
	// Actions are what may be done next with it, as Offers lists them, each the command that does it and its answers.
	Actions []domain.FollowUp `json:"actions"`
}

// Origin is where a worktree's branch stands against origin's.
type Origin struct {
	Ahead  int                 `json:"ahead"`
	Behind int                 `json:"behind"`
	State  worktree.Divergence `json:"state"`
}

// PR is a worktree's open pull request.
type PR struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

// The codes of list's warnings and failures.
const (
	CodePRsUnavailable = Name + ".prs_unavailable"
	CodePRsCapped      = Name + ".prs_capped"
	CodeJobsUnknown    = Name + ".jobs_unknown"
	CodeBaseMissing    = Name + ".base_missing"
	CodeUnreadable     = Name + ".unreadable"
)

// ----------------------------------------------------------------------------
// The listing

// List is each worktree of facts as an item, the main one first, then the others by creation, the oldest first and those with no record last, and the warnings of what could not be read.
func List(facts Facts) ([]Item, []domain.Warning) {
	worktrees := facts.listed()
	slices.SortStableFunc(worktrees, func(a, b worktree.Worktree) int {
		if a.Main != b.Main {
			return compareTrue(a.Main, b.Main)
		}
		return compareCreated(facts.States[a.Path].Meta.CreatedAt, facts.States[b.Path].Meta.CreatedAt)
	})

	items := make([]Item, 0, len(worktrees))
	for _, w := range worktrees {
		items = append(items, facts.item(w))
	}

	return items, facts.warnings()
}

func (f Facts) item(w worktree.Worktree) Item {
	state := f.States[w.Path]
	subject := cmp.Or(w.Branch(), w.Path)
	if errors.Is(state.Err, context.Canceled) {
		return Item{Subject: subject, Status: domain.StatusCancelled, Reason: domain.ReasonInterrupted}
	}
	if state.Err != nil {
		return Item{Subject: subject, Status: domain.StatusFailed, Err: &domain.Error{
			Kind: domain.KindInternal, Code: CodeUnreadable, Message: "couldn't read " + w.Path, Params: domain.Params{"path": w.Path}, Cause: state.Err,
		}}
	}

	_, rebasing := w.State.(worktree.Rebasing)
	detail := Detail{
		Branch:       w.Branch(),
		Path:         w.Path,
		Main:         w.Main,
		Dirty:        state.Dirty,
		Locked:       w.Locked,
		Rebasing:     rebasing,
		Missing:      state.Missing,
		CommitsAhead: state.Ahead,
		CreatedAt:    createdAt(state.Meta.CreatedAt),
		Origin:       f.origin(w.Branch()),
		PR:           f.pr(w.Branch()),
		Services:     f.services(w.Path),
	}
	detail.Actions = Actions(detail)

	return Item{Subject: subject, Status: domain.StatusDone, Detail: &detail}
}

func (f Facts) origin(branch string) *Origin {
	standing := f.Standing(branch)
	if branch == "" || standing == worktree.NoOrigin {
		return nil
	}

	counts := f.AheadBehind(branch)
	return &Origin{Ahead: counts.Ahead, Behind: counts.Behind, State: standing}
}

func (f Facts) pr(branch string) *PR {
	at := slices.IndexFunc(f.PullRequests.Open, func(pr worktree.PullRequest) bool { return branch != "" && pr.Branch == branch })
	if at < 0 {
		return nil
	}

	found := f.PullRequests.Open[at]
	return &PR{Number: found.Number, URL: found.URL}
}

// services are the jobs up in the worktree at path, by name, none when the daemon could not tell.
func (f Facts) services(path string) []string {
	if f.JobsErr != nil {
		return nil
	}

	names := []string{}
	for _, job := range f.Jobs {
		if job.Up && job.WorkDir == path {
			names = append(names, job.Name)
		}
	}
	slices.Sort(names)

	return names
}

func (f Facts) warnings() []domain.Warning {
	var warnings []domain.Warning
	if base := f.Config.Worktrees.BaseBranch; base != "" && !f.Local(base) {
		warnings = append(warnings, domain.Warning{Code: CodeBaseMissing, Message: "the base branch " + base + " does not exist: commits ahead of it are not counted", Params: domain.Params{"branch": base}})
	}
	if f.JobsErr != nil {
		warnings = append(warnings, domain.Warning{Code: CodeJobsUnknown, Message: jobsUnknown(f.JobsErr)})
	}
	if f.PRsErr != nil {
		warnings = append(warnings, domain.Warning{Code: CodePRsUnavailable, Message: f.PRsErr.Error()})
	}
	if f.PullRequests.Capped {
		warnings = append(warnings, domain.Warning{Code: CodePRsCapped, Message: "only the most recent open pull requests were listed: an older one is not shown"})
	}

	return warnings
}

func jobsUnknown(err error) string {
	if errors.Is(err, run.ErrUnknown) {
		return "the run daemon is not running: which jobs are up is unknown"
	}

	return "couldn't ask the run daemon which jobs are up: " + err.Error()
}

// counted reports whether the commits a worktree's branch has past the base branch are counted: not for the main worktree, a worktree on no branch, nor when the base branch does not exist.
func (f Facts) counted(w worktree.Worktree) bool {
	base := f.Config.Worktrees.BaseBranch
	return !w.Main && w.Branch() != "" && base != "" && f.Local(base)
}

// listed are the worktrees list reports: all but a bare repository's entry, which has no working tree.
func (f Facts) listed() []worktree.Worktree {
	return slices.DeleteFunc(slices.Clone(f.Worktrees), func(w worktree.Worktree) bool {
		_, bare := w.State.(worktree.Bare)
		return bare
	})
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

// PathWorktree is the path of the answer an action's command takes: the worktree, by its branch or, on no branch, its path.
const PathWorktree = "worktree"

// ParamNumber is the param of the action that opens a pull request: its number, beside its URL.
const ParamNumber = "number"

// byAction is what each action runs, the wtm command as its command line spells it after the program's name, none to open a pull request, and what it reads as in the picker and as the note of its follow-up.
var byAction = map[Action]struct{ command, label string }{
	ActionGo:      {"go", "Go to the worktree"},
	ActionOpenPR:  {"", "Open the pull request"},
	ActionRunUp:   {"run up", "Start the profile"},
	ActionRunDown: {"run down", "Stop the profile"},
	ActionLogs:    {"run logs", "View the logs"},
	ActionClean:   {"clean", "Clean (delete the worktree)"},
}

// Actions are the follow-ups the worktree of detail offers, as Offers lists them.
func Actions(detail Detail) []domain.FollowUp {
	actions := []domain.FollowUp{}
	for _, action := range Offers(detail) {
		actions = append(actions, FollowUpOf(action, detail))
	}

	return actions
}

// FollowUpOf is action on the worktree of detail as the follow-up that carries it out, with its code and its label: its wtm command and answers, or the URL of its pull request and its number.
func FollowUpOf(action Action, detail Detail) domain.FollowUp {
	followUp := domain.FollowUp{Code: Name + "." + string(action), Message: byAction[action].label}
	if action == ActionOpenPR && detail.PR != nil {
		followUp.URL, followUp.Params = detail.PR.URL, domain.Params{ParamNumber: strconv.Itoa(detail.PR.Number)}
		return followUp
	}

	followUp.Command, followUp.Answers = byAction[action].command, []domain.Answered{{Path: PathWorktree, Values: []string{cmp.Or(detail.Branch, detail.Path)}}}
	return followUp
}

// Holding returns the subject of the item whose worktree holds the directory cwd, the deepest one, "" when none does.
func Holding(items []Item, cwd string) string {
	subject, deepest := "", 0
	for _, item := range items {
		if item.Detail == nil || !within(cwd, item.Detail.Path) || len(item.Detail.Path) <= deepest {
			continue
		}
		subject, deepest = item.Subject, len(item.Detail.Path)
	}

	return subject
}

// ----------------------------------------------------------------------------
// Helpers

func within(dir, root string) bool {
	return dir == root || strings.HasPrefix(dir, root+string(filepath.Separator))
}

// createdAt is the time a meta record's created_at says, nil when it has none or it does not parse.
func createdAt(recorded string) *time.Time {
	at, err := time.Parse(time.RFC3339, recorded)
	if err != nil {
		return nil
	}

	return &at
}

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

// compareCreated orders two created_at records oldest first, one that does not parse last.
func compareCreated(a, b string) int {
	at, bt := createdAt(a), createdAt(b)
	if at == nil || bt == nil {
		return compareTrue(at != nil, bt != nil)
	}

	return at.Compare(*bt)
}
