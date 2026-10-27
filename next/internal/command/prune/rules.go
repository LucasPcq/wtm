package prune

import (
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/project"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/service/teardown"
)

// ----------------------------------------------------------------------------
// Facts

// Facts are what prune decides from, as Observe read them: what a removal decides from (the project's config, its run.toml, the repository's snapshot, the records, what each worktree done with holds that makes it unsafe to remove, where the caller stands, why the run daemon or the forge could not be asked), the pull request of each worktree's branch, the branches behind their finished pull request's head (repository.Behind), the branches whose upstream origin deleted, and why fetching origin failed.
type Facts struct {
	teardown.Facts
	PullRequests []worktree.PullRequest
	Behind       map[string]bool // by branch
	Gone         map[string]bool // by branch
	FetchErr     error
}

// ----------------------------------------------------------------------------
// The decision of each worktree

// A Reason is why a worktree is done with, as a code each surface words its own way.
type Reason string

// The reasons a worktree is done with, in the order prune tells them.
const (
	ReasonMerged Reason = "merged" // its pull request merged it, and it has not moved since
	ReasonClosed Reason = "closed" // its pull request was closed, and it has not moved since
	ReasonGone   Reason = "gone"   // origin deleted the branch it tracks
)

// Reasons returns every Reason.
func Reasons() []Reason {
	return []Reason{ReasonMerged, ReasonClosed, ReasonGone}
}

// A Candidate is a worktree done with: its removal, why it is done with, and the pull request that finished it, none when origin deleted its branch.
type Candidate struct {
	teardown.Removal
	Reason Reason `json:"reason"`
	PR     *PR    `json:"pr,omitempty"`
}

// PR is the pull request that finished a candidate: its number and its page.
type PR struct {
	Number int    `json:"number"`
	URL    string `json:"url"`
}

// Candidates are the worktrees done with for one of the reasons req asks, in git's order: neither the main worktree, nor the base branch's, nor one on no branch; merged or closed while its branch stands at its pull request's head or behind it (worktree.Repository.Finished), else tracking a branch origin deleted.
func (f Facts) Candidates(req Request) []Candidate {
	var found []Candidate
	for _, w := range f.Worktrees {
		reason, pr, done := f.reasonOf(w, req)
		if !done {
			continue
		}
		candidate := Candidate{Removal: f.Removal(w, slices.Index(req.Worktrees, w.Name())), Reason: reason}
		if pr != nil {
			candidate.PR = &PR{Number: pr.Number, URL: pr.URL}
		}
		found = append(found, candidate)
	}

	return found
}

// reasonOf is why w is done with, the first reason req asks that holds, with the pull request that finished it, and whether it is.
func (f Facts) reasonOf(w worktree.Worktree, req Request) (Reason, *worktree.PullRequest, bool) {
	branch := w.Branch()
	if w.Main || branch == "" || branch == f.Config.Worktrees.BaseBranch {
		return "", nil, false
	}

	pr, has := worktree.PullRequestOf(f.PullRequests, branch)
	finished := has && f.Finished(pr, w, f.Behind[branch])
	switch {
	case finished && pr.State == worktree.PullRequestMerged && req.asks(ReasonMerged):
		return ReasonMerged, &pr, true
	case finished && pr.State == worktree.PullRequestClosed && req.asks(ReasonClosed):
		return ReasonClosed, &pr, true
	case f.Gone[branch] && req.asks(ReasonGone):
		return ReasonGone, nil, true
	}

	return "", nil, false
}

// asks reports whether the request asks for reason.
func (r Request) asks(reason Reason) bool {
	return slices.Contains(r.Reasons(), reason)
}

// The codes of an entry that names no candidate: one not done with, the main worktree, the base branch.
const (
	CodeNotFinished  = Name + ".not_finished"
	CodeMainWorktree = Name + ".main_worktree"
	CodeBaseBranch   = Name + ".base_branch"
)

// picked are the candidates the request picks, with the problems of its entries: those it names, each a candidate, else every one safe to remove, every one when forced.
func (f Facts) picked(req Request, candidates []Candidate) ([]Candidate, []domain.FieldError) {
	if len(req.Worktrees) == 0 {
		return slices.DeleteFunc(slices.Clone(candidates), func(c Candidate) bool { return c.Unsafe() && !req.Force }), nil
	}

	var picked []Candidate
	var problems []domain.FieldError
	for index, name := range req.Worktrees {
		path := teardown.EntryPath(index)
		at := slices.IndexFunc(candidates, func(c Candidate) bool { return c.Worktree == name })
		switch {
		case strings.TrimSpace(name) == "":
			problems = append(problems, domain.Required(path))
		case slices.Contains(req.Worktrees[:index], name):
			problems = append(problems, domain.FieldError{Path: path, Code: domain.CodeDistinct, Template: "{path}: {value} is given twice", Params: domain.Params{"value": name}})
		case at >= 0:
			picked = append(picked, candidates[at])
		default:
			problems = append(problems, f.notCandidate(req, path, name))
		}
	}

	return picked, problems
}

// notCandidate is the problem of the entry at path naming name, which is no candidate: the main worktree, the base branch, or a worktree not done with, and why.
func (f Facts) notCandidate(req Request, path, name string) domain.FieldError {
	problem := domain.FieldError{Path: path, Code: CodeNotFinished, Template: "{value} is not done with: {why}", Params: domain.Params{"value": name}}
	at := slices.IndexFunc(f.Worktrees, func(w worktree.Worktree) bool { return w.Name() == name })
	if at < 0 {
		problem.Params["why"] = "no worktree is on that branch"
		return problem
	}

	w := f.Worktrees[at]
	pr, has := worktree.PullRequestOf(f.PullRequests, w.Branch())
	switch {
	case w.Main:
		problem.Code, problem.Template = CodeMainWorktree, "{value} is the main worktree: it is never removed"
	case w.Branch() == f.Config.Worktrees.BaseBranch:
		problem.Code, problem.Template = CodeBaseBranch, "{value} is the base branch: it is never removed"
	case has && pr.State == worktree.PullRequestOpen:
		problem.Params["why"] = "its pull request is still open"
	case has && pr.State != worktree.PullRequestOpen && !f.Finished(pr, w, f.Behind[w.Branch()]):
		problem.Params["why"] = "it has commits past its pull request's head"
	case has && !req.asks(Reason(pr.State)):
		problem.Params["why"] = "its pull request is " + string(pr.State) + ", which this run does not ask for"
	case f.PRsErr != nil:
		problem.Params["why"] = "its pull requests could not be asked"
	default:
		problem.Params["why"] = "it has no merged or closed pull request"
	}
	if why, asked := problem.Params["why"]; asked && req.asks(ReasonGone) && !has {
		problem.Params["why"] = why + ", and origin has not deleted its branch"
	}

	return problem
}

func removals(candidates []Candidate) []teardown.Removal {
	found := make([]teardown.Removal, 0, len(candidates))
	for _, c := range candidates {
		found = append(found, c.Removal)
	}

	return found
}

func names(candidates []Candidate) []string {
	found := make([]string, 0, len(candidates))
	for _, c := range candidates {
		found = append(found, c.Worktree)
	}

	return found
}

// blocked are the candidates a blocker refuses unless forced (teardown.Removal.Unsafe).
func blocked(candidates []Candidate) []Candidate {
	return slices.DeleteFunc(slices.Clone(candidates), func(c Candidate) bool { return !c.Unsafe() })
}

// ruledOut reports whether the plan leaves the candidate as it is, unsafe and chosen by the command: it has no phase.
func (c Candidate) ruledOut() bool {
	return len(c.Phases) == 0
}

// ----------------------------------------------------------------------------
// Decide

// Decide turns the facts and the resolved request into the plan and its warnings. A worktree the request names that a blocker makes unsafe refuses the whole request, before anything is removed, with CodeUnsafe, unless the request forces it. One the command chose itself, the request naming none, is left as it is unless forced: a plan entry with its blockers and no phase, an item ruled out on the apply, and a CodeKept warning.
func Decide(f Facts, req Request) (Plan, []domain.Warning, error) {
	candidates := f.Candidates(req)
	picked, _ := f.picked(req, candidates)
	if unsafe := blocked(picked); len(unsafe) > 0 && !req.Force {
		forced := req
		forced.Force = true
		return Plan{}, nil, teardown.Refusal(teardown.RefusalParams{Command: Name, Blocked: removals(unsafe), Forced: domain.AnswersOf(Questions{}.Form(), forced)})
	}

	worktrees := picked
	if len(req.Worktrees) == 0 {
		worktrees = slices.Clone(candidates)
		for i, c := range worktrees {
			if c.Unsafe() && !req.Force {
				worktrees[i].Phases, worktrees[i].Hooks = []domain.Phase{}, nil
			}
		}
	}
	planned := Plan{Worktrees: orEmpty(worktrees), Reasons: req.Reasons(), ReparentChildren: req.ReparentChildren, Force: req.Force, facts: f}
	planned.Moves = f.Moves(removals(planned.removed()))
	planned.standing = f.Standing(removals(planned.removed()))

	return planned, f.warnings(planned, req), nil
}

func orEmpty(candidates []Candidate) []Candidate {
	if candidates == nil {
		return []Candidate{}
	}
	return candidates
}

// CodeUnsafe refuses a request naming a worktree a blocker makes unsafe; CodeForce is its follow-up, the same request forced, and that of the worktrees left as they are; CodeStopJobs is the one that stops a worktree's jobs.
const (
	CodeUnsafe   = Name + teardown.Unsafe
	CodeForce    = Name + teardown.Force
	CodeStopJobs = Name + teardown.StopJobs
)

// The codes of the plan's warnings: a worktree done with but unsafe to remove, which the command chose and leaves as it is; origin that could not be fetched; jobs a forced removal leaves running; the namespaces a removed worktree keeps in shared services.
const (
	CodeKept        = Name + ".kept"
	CodeFetchFailed = Name + ".fetch_failed"
	CodeJobsLeft    = Name + teardown.JobsLeft
	CodeDataKept    = Name + teardown.DataKept
)

// warnings are what the plan cannot see to or leaves behind: origin that could not be fetched, so gone branches are read as last fetched; what a removal cannot see to or leaves behind (teardown.Facts.Warnings), a forge that could not be asked matching no pull request; each worktree kept for being unsafe, the last followed by the request that prunes them all anyway.
func (f Facts) warnings(planned Plan, req Request) []domain.Warning {
	var warnings []domain.Warning
	if f.FetchErr != nil {
		warnings = append(warnings, domain.Warning{Code: CodeFetchFailed, Message: "couldn't fetch origin (" + f.FetchErr.Error() + "): the branches it deleted are read as last fetched", Params: domain.Params{"cause": f.FetchErr.Error()}})
	}

	unasked := "open ones aren't checked, merged ones don't count as pushed"
	if req.asks(ReasonMerged) || req.asks(ReasonClosed) {
		unasked = "merged and closed ones match nothing, " + unasked
	}
	warnings = append(warnings, f.Warnings(teardown.WarningsParams{Command: Name, Removals: removals(planned.removed()), Unasked: unasked})...)

	kept := planned.kept()
	for i, c := range kept {
		lost := strings.Join(domain.Names(c.Blockers), ", ")
		warning := domain.Warning{Code: CodeKept, Message: c.Worktree + " is kept: unsafe to remove (" + lost + ")", Params: domain.Params{"worktree": c.Worktree, "blockers": lost}}
		if i == len(kept)-1 {
			forced := req
			forced.Worktrees, forced.Force = names(kept), true
			warning.FollowUp = &domain.FollowUp{Command: Name, Answers: domain.AnswersOf(Questions{}.Form(), forced), Code: CodeForce, Message: "prune them anyway"}
		}
		warnings = append(warnings, warning)
	}

	return warnings
}

// ----------------------------------------------------------------------------
// The plan

// A Plan is what Apply will do, as the dry run and the JSON envelope show it and the fingerprint covers it: the worktrees done with, each removed in its phases or, unsafe and chosen by the command, left as it is with no phase; the children left behind, the reasons asked, whether the children are reparented and whether the unsafe worktrees are forced; then the recap, which the fingerprint leaves out.
type Plan struct {
	Worktrees        []Candidate     `json:"worktrees"`
	Moves            []teardown.Move `json:"moves,omitempty"`
	Reasons          []Reason        `json:"reasons"`
	ReparentChildren bool            `json:"reparent_children"`
	Force            bool            `json:"force"`
	domain.Recapped
	facts    Facts
	standing string // the worktree removed the caller stands in, empty when none
}

// removed are the worktrees the plan removes, kept those it leaves as they are.
func (p Plan) removed() []Candidate {
	return slices.DeleteFunc(slices.Clone(p.Worktrees), Candidate.ruledOut)
}

func (p Plan) kept() []Candidate {
	return slices.DeleteFunc(slices.Clone(p.Worktrees), func(c Candidate) bool { return !c.ruledOut() })
}

// Locks are the locks the plan needs: the repository shared, and each worktree it removes and each child it reparents exclusive.
func (p Plan) Locks() []domain.Lock {
	locked := names(p.removed())
	if p.ReparentChildren {
		for _, move := range p.Moves {
			locked = append(locked, move.Branch)
		}
	}

	return domain.WorktreeLocks(locked)
}

// Fingerprinted is what the plan's fingerprint covers: the plan, the on_clean hooks of each removal among it, and what their environment reads of each removal's record (its ordinal and isolation, its ports and Compose project); its recap left out.
func (p Plan) Fingerprinted() any {
	p.Recapped = domain.Recapped{}
	type hookEnv struct {
		Ordinal   int               `json:"ordinal"`
		Isolation project.Isolation `json:"isolation"`
	}
	envs := map[string]hookEnv{}
	for _, c := range p.removed() {
		record := p.facts.Records[c.Branch]
		envs[c.Worktree] = hookEnv{Ordinal: record.Ordinal, Isolation: record.Isolation.Effective()}
	}

	return struct {
		Plan
		HookEnvs map[string]hookEnv `json:"hook_envs"`
	}{p, envs}
}

// Locks are the locks taken before planning: the repository shared, and each worktree named exclusive; the plan names the others.
func (r Request) Locks() []domain.Lock {
	return domain.WorktreeLocks(slices.DeleteFunc(slices.Clone(r.Worktrees), func(name string) bool { return strings.TrimSpace(name) == "" }))
}

// ----------------------------------------------------------------------------
// The recap

// NoteKept notes a recap's value done with but kept for being unsafe; the others are teardown's.
const NoteKept = "kept"

// recap is the plan row by row: the worktrees removed, each with why it is done with and, forced, what it loses; those kept, with what makes them unsafe; then the children left behind, reparented or orphaned.
func (p Plan) recap() []domain.RecapRow {
	var rows []domain.RecapRow
	if removed := p.removed(); len(removed) > 0 {
		rows = append(rows, domain.RecapRow{Key: "prune", Label: "Prune", Path: domain.PathWorktree, Values: recappedPruned(removed)})
	}
	if kept := p.kept(); len(kept) > 0 {
		values := make([]domain.RecapValue, 0, len(kept))
		for _, c := range kept {
			lost := strings.Join(domain.Names(c.Blockers), ", ")
			values = append(values, domain.RecapValue{Value: c.Worktree, Note: NoteKept, Params: domain.Params{"reason": string(c.Reason), "blockers": lost}, Text: c.Worktree + " (" + c.said() + "; " + lost + ")"})
		}
		rows = append(rows, domain.RecapRow{Key: "kept", Label: "Kept", Path: PathForce, Values: values})
	}
	if len(p.Moves) > 0 {
		rows = append(rows, domain.RecapRow{Key: "children", Label: "Children", Path: PathReparentChildren, Values: teardown.RecappedMoves(p.Moves, p.ReparentChildren)})
	}

	return rows
}

// recappedPruned are the worktrees removed as the recap's values: each with why it is done with, and, forced, what it loses.
func recappedPruned(removed []Candidate) []domain.RecapValue {
	values := teardown.Recapped(removals(removed))
	for i, c := range removed {
		notes := []string{c.said()}
		if lost := values[i].Params["blockers"]; lost != "" {
			notes = append(notes, lost+", forced")
		}
		if values[i].Params == nil {
			values[i].Params = domain.Params{}
		}
		values[i].Params["reason"] = string(c.Reason)
		values[i].Text = c.Worktree + " (" + strings.Join(notes, "; ") + ")"
	}

	return values
}

// said is why the candidate is done with, as the recap's text tells it.
func (c Candidate) said() string {
	if c.PR == nil {
		return "deleted on origin"
	}

	return string(c.Reason) + " #" + strconv.Itoa(c.PR.Number)
}

// ----------------------------------------------------------------------------
// The outcome

// CodeGo is the step out of a worktree removed under the caller's feet, into the main one; CodeOrphaned warns of a child left on its removed parent, and CodeReparent is the step that moves it.
const (
	CodeGo       = Name + teardown.Go
	CodeOrphaned = Name + teardown.Orphaned
	CodeReparent = Name + teardown.Reparent
)

// next is the step into the main worktree when the caller stood in a worktree removed.
func (p Plan) next(items []Item) []domain.FollowUp {
	if !teardown.RemovedIn(items, p.standing) {
		return nil
	}

	return p.facts.Next(Name, p.standing)
}
