// Package github asks GitHub, through its CLI gh, for the pull requests of a repository: the open ones with their checks and review, or the newest of each branch whatever its state.
package github

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/adapter/shell"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

const (
	cli = "gh"
	// shortcut: the 50 open pull requests gh lists first, as wtm's list asks, saying when there are more; query by branch when that matters.
	listLimit = 50
	// prsPerQuery bounds the branches one GraphQL query asks of, far below the API's node limit.
	prsPerQuery = 100
	// newestPerBranch is how many pull requests of a branch are asked, newest first: a few, since the newest may be a fork's.
	newestPerBranch = 5
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
	binary, err := g.signedIn(ctx)
	if err != nil {
		return worktree.PullRequests{}, err
	}

	// One more than the cap tells a listing that holds more.
	out, err := g.run(ctx, binary, "gh pr list", "pr", "list", "--state", "open", "--json", "number,headRefName,url,isCrossRepository,title,isDraft,reviewDecision,statusCheckRollup", "--limit", strconv.Itoa(listLimit+1))
	if err != nil {
		return worktree.PullRequests{}, err
	}

	return parse(out)
}

// PullRequestsOf returns the newest pull request of each of branches, whatever
// its state, one from a fork left out; a branch without one is absent. It asks
// one GraphQL query per prsPerQuery branches, so a repository with many pull
// requests costs no more. Every failure wraps worktree.ErrPullRequestsUnavailable;
// a query that fails keeps the pull requests the queries before it found.
func (g GitHub) PullRequestsOf(ctx context.Context, branches []string) ([]worktree.PullRequest, error) {
	binary, err := g.signedIn(ctx)
	if err != nil {
		return nil, err
	}

	var found []worktree.PullRequest
	for batch := range slices.Chunk(branches, prsPerQuery) {
		out, err := g.run(ctx, binary, "gh api graphql", newestOf(batch)...)
		if err != nil {
			return found, err
		}
		prs, err := parseNewest(out)
		if err != nil {
			return found, err
		}
		found = append(found, prs...)
	}

	return found, nil
}

// signedIn returns the path of gh once it is found and signed in.
func (g GitHub) signedIn(ctx context.Context) (string, error) {
	// What failed matters less than what to do about it: the cause is left out.
	binary, err := exec.LookPath(cmp.Or(g.Binary, cli))
	if err != nil {
		return "", fmt.Errorf("%w: install gh, GitHub's CLI", worktree.ErrPullRequestsUnavailable)
	}

	if err := shell.Command(ctx, binary, "auth", "status").Run(); err != nil {
		return "", fmt.Errorf("%w: run `gh auth login`", worktree.ErrPullRequestsUnavailable)
	}

	return binary, nil
}

// run runs binary with args in the repository and returns its output; what names the command in its failure.
func (g GitHub) run(ctx context.Context, binary, what string, args ...string) ([]byte, error) {
	cmd := shell.Command(ctx, binary, args...)
	cmd.Dir = g.Dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("%w: %s: %w: %s", worktree.ErrPullRequestsUnavailable, what, err, strings.TrimSpace(stderr.String()))
	}

	return out, nil
}

// newestOf is gh's arguments of the GraphQL query for the newest pull requests of branches, one aliased field per branch, each name a variable never spliced into the query; gh fills {owner} and {repo} from the repository it runs in.
func newestOf(branches []string) []string {
	variables := []string{"$owner: String!", "$name: String!"}
	args := []string{"api", "graphql", "-F", "owner={owner}", "-F", "name={repo}"}
	fields := make([]string, 0, len(branches))
	for i, branch := range branches {
		alias := "b" + strconv.Itoa(i)
		variables = append(variables, "$"+alias+": String!")
		fields = append(fields, alias+": pullRequests(headRefName: $"+alias+", first: "+strconv.Itoa(newestPerBranch)+", orderBy: {field: CREATED_AT, direction: DESC}) { nodes { number url state headRefName isCrossRepository } }")
		args = append(args, "-f", alias+"="+branch)
	}
	query := "query(" + strings.Join(variables, ", ") + ") { repository(owner: $owner, name: $name) { " + strings.Join(fields, " ") + " } }"

	return append(args, "-f", "query="+query)
}

// parseNewest reads the answer of newestOf: per branch, its newest pull request that is not a fork's, its state lowercased.
func parseNewest(out []byte) ([]worktree.PullRequest, error) {
	type node struct {
		Number            int    `json:"number"`
		URL               string `json:"url"`
		State             string `json:"state"`
		HeadRefName       string `json:"headRefName"`
		IsCrossRepository bool   `json:"isCrossRepository"`
	}
	var answer struct {
		Data struct {
			Repository map[string]struct {
				Nodes []node `json:"nodes"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &answer); err != nil {
		return nil, fmt.Errorf("%w: gh api graphql: unexpected output: %w", worktree.ErrPullRequestsUnavailable, err)
	}

	var prs []worktree.PullRequest
	for _, alias := range slices.Sorted(maps.Keys(answer.Data.Repository)) {
		nodes := answer.Data.Repository[alias].Nodes
		at := slices.IndexFunc(nodes, func(pr node) bool { return !pr.IsCrossRepository })
		if at < 0 {
			continue
		}
		pr := nodes[at]
		prs = append(prs, worktree.PullRequest{Number: pr.Number, URL: pr.URL, Branch: pr.HeadRefName, State: worktree.PullRequestState(strings.ToLower(pr.State))})
	}

	return prs, nil
}

func parse(out []byte) (worktree.PullRequests, error) {
	var listed []struct {
		Number            int        `json:"number"`
		HeadRefName       string     `json:"headRefName"`
		URL               string     `json:"url"`
		IsCrossRepository bool       `json:"isCrossRepository"`
		Title             string     `json:"title"`
		IsDraft           bool       `json:"isDraft"`
		ReviewDecision    string     `json:"reviewDecision"`
		StatusCheckRollup []checkRun `json:"statusCheckRollup"`
	}
	if err := json.Unmarshal(out, &listed); err != nil {
		return worktree.PullRequests{}, fmt.Errorf("%w: gh pr list: unexpected output: %w", worktree.ErrPullRequestsUnavailable, err)
	}

	prs := worktree.PullRequests{Capped: len(listed) > listLimit}
	for _, pr := range listed[:min(len(listed), listLimit)] {
		if !pr.IsCrossRepository {
			prs.Open = append(prs.Open, worktree.PullRequest{
				Number: pr.Number, URL: pr.URL, Branch: pr.HeadRefName, State: worktree.PullRequestOpen,
				Title: pr.Title, Draft: pr.IsDraft, Checks: checksOf(pr.StatusCheckRollup), Decision: pr.ReviewDecision,
			})
		}
	}

	return prs, nil
}

// A checkRun is one entry of a pull request's status-check rollup: a check run, which ends with a Conclusion, or a status of the older Status API, which carries only a State.
type checkRun struct {
	Conclusion string `json:"conclusion"`
	State      string `json:"state"`
}

// checksOf counts the rollup's entries by outcome.
func checksOf(rollup []checkRun) worktree.Checks {
	var checks worktree.Checks
	for _, run := range rollup {
		switch outcomeOf(run) {
		case passed:
			checks.Passed++
		case failed:
			checks.Failed++
		case pending:
			checks.Pending++
		}
	}
	return checks
}

type outcome int

const (
	pending outcome = iota
	passed
	failed
)

// outcomeOf is the outcome of a rollup entry: by its conclusion once it ran, by its state for a status, pending while neither is set. A skipped or neutral check passes; one that needs authorisation to run (ACTION_REQUIRED) has not run, so it is pending, as is a status still EXPECTED; an unknown value fails closed, never passing silently.
func outcomeOf(run checkRun) outcome {
	switch {
	case run.Conclusion != "":
		switch run.Conclusion {
		case "SUCCESS", "NEUTRAL", "SKIPPED":
			return passed
		case "ACTION_REQUIRED":
			return pending
		}
		return failed
	case run.State != "":
		switch run.State {
		case "SUCCESS":
			return passed
		case "PENDING", "EXPECTED":
			return pending
		}
		return failed
	}
	return pending
}
