package worktree

import (
	"errors"
	"slices"
)

// A PullRequest is a pull request of the repository: its number, its page, the branch it would merge and the commit its head last stood at, whether it is open, merged or closed, its title, whether it is a draft, its checks counted, its review decision as GitHub spells it (APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED, empty when none is required), the branch it would merge into, who opened it, and whether its branch is a fork's rather than this repository's.
type PullRequest struct {
	Number   int              `json:"number"`
	URL      string           `json:"url"`
	Branch   string           `json:"branch"`
	Head     string           `json:"head,omitempty"`
	State    PullRequestState `json:"state"`
	Title    string           `json:"title,omitempty"`
	Draft    bool             `json:"draft,omitempty"`
	Checks   Checks           `json:"checks,omitzero"`
	Decision string           `json:"decision,omitempty"`
	Base     string           `json:"base,omitempty"`
	Author   string           `json:"author,omitempty"`
	Fork     bool             `json:"fork,omitempty"`
}

// A PullRequestFilter narrows the open pull requests listed: all of them, the user's own, or those asking for the user's review.
type PullRequestFilter string

// The filters of the open pull requests.
const (
	AllPullRequests PullRequestFilter = ""
	MyPullRequests  PullRequestFilter = "mine"
	ReviewRequested PullRequestFilter = "review_requested"
)

// Checks counts a pull request's checks by outcome.
type Checks struct {
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Pending int `json:"pending"`
}

// A PullRequestState is where a pull request stands: open, merged or closed.
type PullRequestState string

// The states of a pull request.
const (
	PullRequestOpen   PullRequestState = "open"
	PullRequestMerged PullRequestState = "merged"
	PullRequestClosed PullRequestState = "closed"
)

// PullRequestStates returns every PullRequestState.
func PullRequestStates() []PullRequestState {
	return []PullRequestState{PullRequestOpen, PullRequestMerged, PullRequestClosed}
}

// PullRequestOf returns the pull request of branch among prs, and reports whether it has one; a worktree on no branch has none.
func PullRequestOf(prs []PullRequest, branch string) (PullRequest, bool) {
	at := slices.IndexFunc(prs, func(pr PullRequest) bool { return branch != "" && pr.Branch == branch })
	if at < 0 {
		return PullRequest{}, false
	}

	return prs[at], true
}

// Cleans reports whether pr, once merged, leaves the worktree w to be cleaned: Finished, for a merged pull request alone; list, status and tree read it.
func (r Repository) Cleans(pr PullRequest, w Worktree, behind bool) bool {
	return pr.State == PullRequestMerged && r.Finished(pr, w, behind)
}

// Finished reports whether pr, merged or closed, has done with the worktree w: while w's branch stands at the commit pr last stood at, or behind it (behind: its tip is an ancestor of pr's head, as FinishedPast asks), so a branch name reused or moved on since is not taken for finished, and one behind its pull request's head is; never the main worktree nor one on the base branch, which outlive any pull request.
func (r Repository) Finished(pr PullRequest, w Worktree, behind bool) bool {
	tip, ok := r.finishedAt(pr, w)
	return ok && (tip == pr.Head || behind)
}

// FinishedPast returns the ancestry Finished needs to know of, and reports whether it does: whether w's branch tip is behind the head of its finished pull request pr, asked only when the tip is not that head.
func (r Repository) FinishedPast(pr PullRequest, w Worktree) (Ancestry, bool) {
	tip, ok := r.finishedAt(pr, w)
	return Ancestry{Ancestor: tip, Descendant: pr.Head}, ok && tip != pr.Head
}

// finishedAt is the tip of w's branch, and whether pr is merged or closed and w is a worktree it may have done with: neither the main worktree nor one on the base branch, its branch's tip and pr's head both known.
func (r Repository) finishedAt(pr PullRequest, w Worktree) (string, bool) {
	branch := w.Branch()
	tip := r.Tip(branch)
	finished := pr.State == PullRequestMerged || pr.State == PullRequestClosed
	return tip, finished && !w.Main && branch != r.Config.Worktrees.BaseBranch && tip != "" && pr.Head != ""
}

// ErrPullRequestsUnavailable is why the pull requests could not be listed, wrapped with what to do about it.
var ErrPullRequestsUnavailable = errors.New("pull requests unavailable")

// ErrPullRequestNotFound is a pull request number the repository has none of.
var ErrPullRequestNotFound = errors.New("pull request not found")

// Progress is where an open pull request's review stands: its title, whether it is a draft, its checks counted and GitHub's review decision.
type Progress struct {
	Title    string `json:"title"`
	Draft    bool   `json:"draft"`
	Checks   Checks `json:"checks"`
	Decision string `json:"decision"`
}

// Progress returns where pr's review stands, nil unless pr is open.
func (pr PullRequest) Progress() *Progress {
	if pr.State != PullRequestOpen {
		return nil
	}

	return &Progress{Title: pr.Title, Draft: pr.Draft, Checks: pr.Checks, Decision: pr.Decision}
}
