// Package github asks GitHub, through its CLI gh, for the open pull requests of a repository.
package github

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

const (
	cli = "gh"
	// shortcut: the 50 open pull requests gh lists first, as wtm's list asks; query by branch when a repository holds more.
	listLimit = "50"
)

// GitHub is the GitHub repository of the git repository at Dir, asked through Binary, gh on the PATH when empty.
type GitHub struct {
	Dir    string
	Binary string
}

// PullRequests returns the open pull requests of the repository; it fails with
// worktree.ErrGitHubMissing when gh is not installed and
// worktree.ErrGitHubSignedOut when it is not signed in.
func (g GitHub) PullRequests(ctx context.Context) ([]worktree.PullRequest, error) {
	binary, err := exec.LookPath(cmp.Or(g.Binary, cli))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", worktree.ErrGitHubMissing, err)
	}

	if err := shell.Command(ctx, binary, "auth", "status").Run(); err != nil {
		return nil, fmt.Errorf("%w: %w", worktree.ErrGitHubSignedOut, err)
	}

	cmd := shell.Command(ctx, binary, "pr", "list", "--state", "open", "--json", "number,headRefName,url,state", "--limit", listLimit)
	cmd.Dir = g.Dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh pr list: %w: %s", err, strings.TrimSpace(stderr.String()))
	}

	return parse(out)
}

func parse(out []byte) ([]worktree.PullRequest, error) {
	var listed []struct {
		Number      int    `json:"number"`
		HeadRefName string `json:"headRefName"`
		URL         string `json:"url"`
		State       string `json:"state"`
	}
	if err := json.Unmarshal(out, &listed); err != nil {
		return nil, errors.Join(errors.New("gh pr list: unexpected output"), err)
	}

	prs := make([]worktree.PullRequest, 0, len(listed))
	for _, pr := range listed {
		prs = append(prs, worktree.PullRequest{Number: pr.Number, URL: pr.URL, State: strings.ToLower(pr.State), Branch: pr.HeadRefName})
	}

	return prs, nil
}
