package checkout

import (
	"fmt"
	"slices"
	"strconv"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/env"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/provision"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what checkout decides from, as Observe read them: the project's config, its run.toml and the repository's snapshot, the pull request asked, the open pull requests to pick from, what lies at the path its branch would take and why origin's copy of that branch could not be fetched.
type Facts struct {
	worktree.Repository
	PR       *worktree.PullRequest  // the pull request asked, nil when none is or the repository has none of that number
	Open     []worktree.PullRequest // the open pull requests to pick from, read for the questions only
	Folder   worktree.Location      // what lies at the path the pull request's branch would take
	FetchErr error                  // why origin's copy of the branch could not be fetched: the plan stands on it as last fetched
}

// branch is the pull request's branch, "" when none is known or it is a fork's, which names no branch of this repository.
func (f Facts) branch() string {
	if f.PR == nil || f.PR.Fork {
		return ""
	}

	return f.PR.Branch
}

// base is the branch the pull request merges into as a parent: the local branch, else origin's, "" when neither holds it.
func (f Facts) base() string {
	if f.PR == nil {
		return ""
	}

	switch base := f.PR.Base; {
	case f.Local(base):
		return base
	case f.Remote(base):
		return worktree.OriginPrefix + base
	}

	return ""
}

// behind is the pull request's branch when it exists locally, checked out nowhere, and is behind origin's: the branch a fast-forward would update.
func (f Facts) behind() []string {
	branch := f.branch()
	if branch == "" || !f.Local(branch) || f.CheckedOutAt(branch) != "" || f.Standing(branch) != worktree.Behind {
		return nil
	}

	return []string{branch}
}

// ----------------------------------------------------------------------------
// The decision of the pull request

//go-sumtype:decl decision

// A decision is what becomes of the pull request's branch: placed, or the refusal that stops it.
type decision interface{ decision() }

// placed is how the branch gets its worktree.
type placed struct {
	action   Action
	upstream string
}

// A refusal is why the pull request cannot be checked out, as a field error of the pull request.
type refusal domain.FieldError

func (placed) decision()  {}
func (refusal) decision() {}

// The codes of a pull request that cannot be checked out.
const (
	CodeFork                = Name + ".fork"
	CodeCheckedOutElsewhere = Name + ".checked_out_elsewhere"
	CodePathExists          = Name + ".path_exists"
	CodeNameClash           = Name + ".name_clash"
	CodeHeadMissing         = Name + ".head_missing"
)

// decide places the branch of the pull request Observe read; it panics without one, a bug of the caller.
func (f Facts) decide() decision {
	pr := *f.PR
	if pr.Fork {
		return forkOf(pr)
	}

	path := f.PathOf(pr.Branch)
	placement := worktree.Place(worktree.PlacementParams{
		Branch:       pr.Branch,
		Path:         path,
		CheckedOutAt: f.CheckedOutAt(pr.Branch),
		Location:     f.Folder,
		Occupant:     f.Occupant(f.Folder),
		Local:        f.Local(pr.Branch),
		Remote:       f.Remote(pr.Branch),
		Live:         f.NamedLive(),
	})
	switch p := placement.(type) {
	case worktree.CheckedOutElsewhere:
		return checkedOut(pr, p.Path)
	case worktree.Unchanged:
		return checkedOut(pr, p.Path)
	case worktree.PathTaken:
		return refusal{Code: CodePathExists, Template: "worktree path already exists: {path}", Params: domain.Params{"path": path}}
	case worktree.Clash:
		return refusal{Code: CodeNameClash, Template: "branch {branch} would share its name with {other} ({name})", Params: domain.Params{"branch": pr.Branch, "other": p.Branch, "name": p.Name}}
	case worktree.Placed:
		return placedAs(pr, p)
	}

	return nil
}

// forkOf refuses a fork's pull request, naming gh's command that checks it out.
func forkOf(pr worktree.PullRequest) refusal {
	number := strconv.Itoa(pr.Number)
	return refusal{Code: CodeFork, Template: "pull request #{number} is from a fork: wtm checks out only this repository's branches, `{command}` checks it out", Params: domain.Params{"number": number, "command": "gh pr checkout " + number}}
}

func checkedOut(pr worktree.PullRequest, path string) refusal {
	return refusal{
		Code:     CodeCheckedOutElsewhere,
		Template: "branch {branch} of pull request #{number} is checked out at {path}",
		Params:   domain.Params{"branch": pr.Branch, "number": strconv.Itoa(pr.Number), "path": path},
		FollowUp: jumpIn(pr.Branch),
	}
}

// placedAs is the branch checked out as it is when it exists locally, else tracking origin's; one neither holds is refused, since a pull request's branch is never made anew.
func placedAs(pr worktree.PullRequest, p worktree.Placed) decision {
	switch p.Start {
	case worktree.StartCheckOut:
		return placed{action: ActionCheckOut}
	case worktree.StartResume:
		return placed{action: ActionResume, upstream: p.Upstream}
	case worktree.StartCreate:
	}

	return refusal{Code: CodeHeadMissing, Template: "branch {branch} of pull request #{number} is neither local nor on origin", Params: domain.Params{"branch": pr.Branch, "number": strconv.Itoa(pr.Number)}}
}

// CodeJumpIn is the step into the worktree of a pull request's branch checked out elsewhere, which checkout refuses.
const CodeJumpIn = Name + ".jump_in"

func jumpIn(branch string) *domain.FollowUp {
	enter := domain.Enter(branch)
	enter.Code = CodeJumpIn
	return &enter
}

// ----------------------------------------------------------------------------
// Decide

// Decide turns the facts and the request Resolve settled on them into the plan and its warnings. Resolve refuses a pull request the facts rule out, so Decide refuses none: it panics on a request not resolved on these facts, a bug of the caller.
func Decide(f Facts, req Request) (Plan, []domain.Warning) {
	d, isPlaced := f.decide().(placed)
	if !isPlaced {
		panic(fmt.Sprintf("checkout: decide pull request #%d: the request was not resolved on these facts", req.PR))
	}

	answers := f.newWorktree(req).Answers()
	planned := Plan{
		PullRequest: *f.PR,
		Path:        f.PathOf(f.PR.Branch),
		Action:      d.action,
		Upstream:    d.upstream,
		From:        req.From,
		EnvFrom:     answers.EnvFrom(),
		Isolation:   f.PlannedIsolation(answers.Isolation()),
		FastForward: f.fastForwarded(req),
		Remember:    req.Remember,
		facts:       f,
		settled:     answers,
	}
	planned.Phases = provision.Phases(provision.ParamsOf(f.Repository, planned.provisioned(), nil))
	if slices.Contains(planned.Phases, domain.PhaseOnCreate) {
		planned.OnCreate = f.onCreate()
	}

	return planned, f.warnings(planned)
}

func (f Facts) onCreate() []string {
	commands := make([]string, 0, len(f.Config.OnCreate))
	for _, hook := range f.Config.OnCreate {
		commands = append(commands, hook.Cmd)
	}
	return commands
}

func (f Facts) fastForwarded(req Request) []string {
	if req.FastForward == nil || !*req.FastForward {
		return nil
	}

	return f.behind()
}

// The codes of the plan's warnings.
const (
	CodeSourceDiverged    = Name + ".source_diverged"
	CodeFetchFailed       = Name + ".fetch_failed"
	CodePRNotOpen         = Name + ".pr_not_open"
	CodeEnvParentFallback = Name + ".env_parent_fallback"
)

// warnings are what the plan does that its reader may not expect: a pull request no longer open, a branch whose origin could not be fetched or that diverged from it, an env strategy that copies from main instead.
func (f Facts) warnings(planned Plan) []domain.Warning {
	pr, branch := planned.PullRequest, planned.PullRequest.Branch
	var warnings []domain.Warning
	if pr.State != worktree.PullRequestOpen {
		warnings = append(warnings, domain.Warning{Code: CodePRNotOpen, Message: "pull request #" + strconv.Itoa(pr.Number) + " is " + string(pr.State), Params: domain.Params{"number": strconv.Itoa(pr.Number), "state": string(pr.State)}})
	}

	if f.FetchErr != nil {
		warnings = append(warnings, domain.Warning{Code: CodeFetchFailed, Message: "couldn't fetch " + branch + " from origin: where it stands against origin is as last fetched: " + f.FetchErr.Error(), Params: domain.Params{"branch": branch, "cause": f.FetchErr.Error()}})
	}

	if planned.Action == ActionCheckOut && f.Standing(branch) == worktree.Diverged {
		counts := f.AheadBehind(branch)
		warnings = append(warnings, domain.Warning{
			Code:    CodeSourceDiverged,
			Message: "branch " + branch + " has diverged from origin (" + counts.String() + ") and can't be fast-forwarded: the worktree starts from your local branch, missing commits that are on origin — you may have to rebase or resolve conflicts later",
			Params:  domain.Params{"branch": branch, "ahead": strconv.Itoa(counts.Ahead), "behind": strconv.Itoa(counts.Behind)},
		})
	}

	fallsBack := env.ParentFallsBackToMain(env.ParentFallbackParams{Strategy: planned.EnvFrom, HasFiles: len(f.Config.Env.Files) > 0, SourceHasWorktree: f.CheckedOutAt(planned.From) != ""})
	if fallsBack {
		warnings = append(warnings, domain.Warning{Code: CodeEnvParentFallback, Message: `The "parent" env strategy needs the parent branch checked out to copy its .env; without a worktree it comes from main.`})
	}

	return warnings
}

// ----------------------------------------------------------------------------
// The plan

// An Action is how the pull request's branch gets its worktree.
type Action string

// The actions of a plan.
const (
	ActionCheckOut Action = "check_out" // the branch exists locally: checked out as it is
	ActionResume   Action = "resume"    // only origin has the branch: a local one tracking it
)

// Actions are every Action.
func Actions() []Action {
	return []Action{ActionCheckOut, ActionResume}
}

func (a Action) start() worktree.Start {
	if a == ActionCheckOut {
		return worktree.StartCheckOut
	}

	return worktree.StartResume
}

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: the pull request, the path of its branch's worktree, how the branch gets there and the origin branch it tracks, the phases of its provisioning, its parent, the env strategy, the isolation, the branch to fast-forward first, the on_create hooks and the answers to remember; then the answers recalled and the recap, which the fingerprint leaves out since they change nothing the apply does. The pull request's head is in the fingerprint: a push to it between the preview and the apply is a stale plan.
type Plan struct {
	PullRequest worktree.PullRequest `json:"pull_request"`
	Path        string               `json:"path"`
	Action      Action               `json:"action"`
	Upstream    string               `json:"upstream,omitempty"`
	Phases      []domain.Phase       `json:"phases"`
	From        string               `json:"from"`
	EnvFrom     project.EnvStrategy  `json:"env_from"`
	Isolation   project.Isolation    `json:"isolation,omitempty"`
	FastForward []string             `json:"fast_forward,omitempty"`
	OnCreate    []string             `json:"on_create,omitempty"`
	Remember    []string             `json:"remember,omitempty"`
	domain.Recapped
	facts   Facts
	settled provision.Answers // as resolved, handed to provisioning and remembered
}

// Locks are the locks the plan needs: the repository shared, and the pull request's branch exclusive.
func (p Plan) Locks() []domain.Lock {
	return domain.WorktreeLocks([]string{p.PullRequest.Branch})
}

// Fingerprinted is what the plan's fingerprint covers: the plan with what the apply reads of the project to provision the worktree.
func (p Plan) Fingerprinted() any {
	p.Recapped = domain.Recapped{}
	return provision.Fingerprinted(p, p.facts.Repository, p.settled)
}

// Locks are the locks taken before planning: the repository shared, the branch not being known before the pull request is read.
func (Request) Locks() []domain.Lock {
	return domain.WorktreeLocks(nil)
}

// provisioned is the plan as provision makes it.
func (p Plan) provisioned() provision.Planned {
	return provision.Planned{Branch: p.PullRequest.Branch, Path: p.Path, Placement: p.placement(), From: p.From, Answers: p.settled}
}

func (p Plan) placement() worktree.Placed {
	return worktree.Placed{Start: p.Action.start(), Upstream: p.Upstream}
}

// ----------------------------------------------------------------------------
// The recap

func (p Plan) previewRecap() []domain.RecapRow {
	return p.recap(domain.Marks{Recalled: p.Recalled, Remembered: p.Remember, Mark: domain.MarkRemember})
}

// recap is the plan row by row: the pull request, its branch with how it gets its worktree, the parent wtm sync rebases it onto, then the answers of a new worktree.
func (p Plan) recap(marks domain.Marks) []domain.RecapRow {
	pr := p.PullRequest
	number := strconv.Itoa(pr.Number)
	branch := domain.RecapValue{Value: pr.Branch, Note: worktree.NoteExisting, Text: pr.Branch + " (existing)"}
	if p.Action == ActionResume {
		branch = domain.RecapValue{Value: pr.Branch, Note: worktree.NoteResumes, Params: domain.Params{"upstream": p.Upstream}, Text: worktree.Resumes(pr.Branch, p.Upstream)}
	}

	rows := []domain.RecapRow{
		{Key: PathPR, Label: "Pull request", Path: PathPR, Values: []domain.RecapValue{{Value: number, Text: "#" + number + " " + pr.Title}}},
		{Key: "branch", Label: "Branch", Values: []domain.RecapValue{branch}},
		{Key: "parent", Label: "Parent", Path: PathFrom, Values: domain.RecapValues([]string{p.From})},
	}
	return append(rows, provision.RecapRows(provision.RecapParams{EnvFrom: p.EnvFrom, FollowsConfig: p.settled.FollowsConfig(), Isolation: p.Isolation, RunUnreadable: p.facts.RunErr != nil, FastForward: p.FastForward, Marks: marks})...)
}

// ----------------------------------------------------------------------------
// The outcome

// CodeGo is the step into the worktree an apply ends on.
const CodeGo = Name + ".go"

// CodeBehindOrigin warns that the branch, kept behind origin, starts its worktree from the local branch.
const CodeBehindOrigin = Name + ".behind_origin"

// applied is the plan once applied, as Run reports it: its recap with the fields the apply remembered marked saved.
func (p Plan) applied(result Result) Plan {
	p.Recap = p.recap(domain.Marks{Recalled: p.Recalled, Remembered: result.Remembered, Mark: domain.MarkSaved})
	return p
}

// next is the step into the worktree once it stands, done or not.
func (Plan) next(item Item) []domain.FollowUp {
	if item.Detail == nil {
		return nil
	}

	enter := domain.Enter(item.Subject)
	enter.Code = CodeGo
	return []domain.FollowUp{enter}
}

// behindOrigin warns that the branch checked out behind origin starts its worktree from the local branch; a diverged one is the plan's warning already.
func behindOrigin(standing worktree.Standing) domain.Warning {
	behind := strconv.Itoa(standing.OriginBehind)
	return domain.Warning{
		Code:    CodeBehindOrigin,
		Message: behind + " commit(s) behind origin — the worktree starts from your local branch.",
		Params:  domain.Params{"behind": behind},
	}
}
