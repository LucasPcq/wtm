package worktree

import "errors"

// A PullRequest is an open pull request: its number, its page and the branch it would merge.
type PullRequest struct {
	Number int
	URL    string
	Branch string
}

// PullRequests are the open pull requests the forge listed, and whether it listed only the most recent of more: Capped.
type PullRequests struct {
	Open   []PullRequest
	Capped bool
}

// ErrPullRequestsUnavailable is why the pull requests could not be listed, wrapped with what to do about it.
var ErrPullRequestsUnavailable = errors.New("pull requests unavailable")
