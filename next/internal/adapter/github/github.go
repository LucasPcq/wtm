// Package github asks GitHub, through its CLI gh, for the open pull requests of a repository.
package github

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

const (
	cli = "gh"
	// shortcut: the 50 open pull requests gh lists first, as wtm's list asks, saying when there are more; query by branch when that matters.
	listLimit = 50
)

// GitHub is the GitHub repository of the git repository at Dir, asked through Binary, gh on the PATH when empty.
type GitHub struct {
	Dir    string
	Binary string
}

// ViewCommand is gh's command line that opens pull request number in the browser.
func ViewCommand(number int) []string {
	return []string{cli, "pr", "view", strconv.Itoa(number), "--web"}
}

// PullRequests returns the open pull requests of the repository, the most
// recent first and capped, a pull request from a fork left out since its branch
// is not this repository's. Every failure wraps worktree.ErrPullRequestsUnavailable
// with what to do about it.
func (g GitHub) PullRequests(ctx context.Context) (worktree.PullRequests, error) {
	// What failed matters less than what to do about it: the cause is left out.
	binary, err := exec.LookPath(cmp.Or(g.Binary, cli))
	if err != nil {
		return worktree.PullRequests{}, fmt.Errorf("%w: install gh, GitHub's CLI", worktree.ErrPullRequestsUnavailable)
	}

	if err := shell.Command(ctx, binary, "auth", "status").Run(); err != nil {
		return worktree.PullRequests{}, fmt.Errorf("%w: run `gh auth login`", worktree.ErrPullRequestsUnavailable)
	}

	// One more than the cap tells a listing that holds more.
	cmd := shell.Command(ctx, binary, "pr", "list", "--state", "open", "--json", "number,headRefName,url,isCrossRepository", "--limit", strconv.Itoa(listLimit+1))
	cmd.Dir = g.Dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return worktree.PullRequests{}, fmt.Errorf("%w: gh pr list: %w: %s", worktree.ErrPullRequestsUnavailable, err, strings.TrimSpace(stderr.String()))
	}

	return parse(out)
}

func parse(out []byte) (worktree.PullRequests, error) {
	var listed []struct {
		Number            int    `json:"number"`
		HeadRefName       string `json:"headRefName"`
		URL               string `json:"url"`
		IsCrossRepository bool   `json:"isCrossRepository"`
	}
	if err := json.Unmarshal(out, &listed); err != nil {
		return worktree.PullRequests{}, fmt.Errorf("%w: gh pr list: unexpected output: %w", worktree.ErrPullRequestsUnavailable, err)
	}

	prs := worktree.PullRequests{Capped: len(listed) > listLimit}
	for _, pr := range listed[:min(len(listed), listLimit)] {
		if !pr.IsCrossRepository {
			prs.Open = append(prs.Open, worktree.PullRequest{Number: pr.Number, URL: pr.URL, Branch: pr.HeadRefName})
		}
	}

	return prs, nil
}
