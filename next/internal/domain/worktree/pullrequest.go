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

// PullRequests are the open pull requests the forge listed, and whether it listed only the most recent of more: Capped.
type PullRequests struct {
	Open   []PullRequest
	Capped bool
}

// Of returns the open pull request of branch, and reports whether it has one; a worktree on no branch has none.
func (p PullRequests) Of(branch string) (PullRequest, bool) {
	at := slices.IndexFunc(p.Open, func(pr PullRequest) bool { return branch != "" && pr.Branch == branch })
	if at < 0 {
		return PullRequest{}, false
	}

	return p.Open[at], true
}

// ErrPullRequestsUnavailable is why the pull requests could not be listed, wrapped with what to do about it.
var ErrPullRequestsUnavailable = errors.New("pull requests unavailable")
