package worktree

import "errors"

// A PullRequest is an open pull request on GitHub: its number, its page, its state and the branch it would merge.
type PullRequest struct {
	Number int
	URL    string
	State  string
	Branch string
}

// The reasons GitHub cannot be asked for pull requests: its CLI, gh, is not installed, or not signed in.
var (
	ErrGitHubMissing   = errors.New("the GitHub CLI (gh) is not installed")
	ErrGitHubSignedOut = errors.New("the GitHub CLI (gh) is not signed in")
)
