package worktree

import (
	"errors"
	"slices"
)

// A PullRequest is a pull request of the repository: its number, its page, the branch it would merge, whether it is open, merged or closed, its title, whether it is a draft, its checks counted and its review decision as GitHub spells it (APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED, empty when none is required).
type PullRequest struct {
	Number   int
	URL      string
	Branch   string
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

// Cleans reports whether pr, once merged, leaves the worktree w to be cleaned: never the main worktree nor one on the base branch, which outlive any pull request.
func (pr PullRequest) Cleans(w Worktree, base string) bool {
	return pr.State == PullRequestMerged && !w.Main && w.Branch() != base
}

// ErrPullRequestsUnavailable is why the pull requests could not be listed, wrapped with what to do about it.
var ErrPullRequestsUnavailable = errors.New("pull requests unavailable")
