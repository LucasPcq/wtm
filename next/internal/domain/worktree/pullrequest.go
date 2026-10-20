package worktree

import "errors"

// A PullRequest is a pull request of the repository: its number, its page, the branch it would merge, and whether it is open, merged or closed.
type PullRequest struct {
	Number int
	URL    string
	Branch string
	State  PullRequestState
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

// ErrPullRequestsUnavailable is why the pull requests could not be listed, wrapped with what to do about it.
var ErrPullRequestsUnavailable = errors.New("pull requests unavailable")
