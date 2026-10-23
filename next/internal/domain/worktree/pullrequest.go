package worktree

import (
	"errors"
	"slices"
)

// A PullRequest is a pull request of the repository: its number, its page, the branch it would merge and the commit its head last stood at, whether it is open, merged or closed, its title, whether it is a draft, its checks counted and its review decision as GitHub spells it (APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED, empty when none is required).
type PullRequest struct {
	Number   int
	URL      string
	Branch   string
	Head     string
	State    PullRequestState
	Title    string
	Draft    bool
	Checks   Checks
	Decision string
}

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

// PullRequestOf returns the pull request of branch among prs, and reports whether it has one; a worktree on no branch has none.
func PullRequestOf(prs []PullRequest, branch string) (PullRequest, bool) {
	at := slices.IndexFunc(prs, func(pr PullRequest) bool { return branch != "" && pr.Branch == branch })
	if at < 0 {
		return PullRequest{}, false
	}

	return prs[at], true
}

// Cleans reports whether pr, once merged, leaves the worktree w to be cleaned: while w's branch stands at the commit pr merged, or behind it (behind: its tip is an ancestor of pr's head, as MergedPast asks), so a branch name reused or moved on since is not taken for merged, and one behind its pull request's head is; never the main worktree nor one on the base branch, which outlive any pull request.
func (r Repository) Cleans(pr PullRequest, w Worktree, behind bool) bool {
	tip, ok := r.mergedInto(pr, w)
	return ok && (tip == pr.Head || behind)
}

// MergedPast returns the ancestry Cleans needs to know of, and reports whether it does: whether w's branch tip is behind the head of its merged pull request pr, asked only when the tip is not that head.
func (r Repository) MergedPast(pr PullRequest, w Worktree) (Ancestry, bool) {
	tip, ok := r.mergedInto(pr, w)
	return Ancestry{Ancestor: tip, Descendant: pr.Head}, ok && tip != pr.Head
}

// mergedInto is the tip of w's branch, and whether pr is merged and w is a worktree it may leave to be cleaned: neither the main worktree nor one on the base branch, its branch's tip and pr's head both known.
func (r Repository) mergedInto(pr PullRequest, w Worktree) (string, bool) {
	branch := w.Branch()
	tip := r.Tip(branch)
	return tip, pr.State == PullRequestMerged && !w.Main && branch != r.Config.Worktrees.BaseBranch && tip != "" && pr.Head != ""
}

// ErrPullRequestsUnavailable is why the pull requests could not be listed, wrapped with what to do about it.
var ErrPullRequestsUnavailable = errors.New("pull requests unavailable")
