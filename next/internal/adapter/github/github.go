// Package github asks GitHub, through its CLI gh, for the pull request of each branch of a repository, its open one else its newest whatever its state, with its head commit, its checks and its review; for its open pull requests; and for one pull request by its number.
package github

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
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
	// prsPerQuery bounds the branches one GraphQL query asks of, far below the API's node limit.
	prsPerQuery = 100
	// newestPerBranch is how many pull requests of a branch are asked, newest first: a few, since the newest may be a fork's.
	// shortcut: an open pull request older than its branch's 5 newest is missed, the newest of them read instead; ask open ones apart when that matters.
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

// PullRequestsOf returns the pull request of each of branches, its open one
// else its newest whatever its state, with the commit its head stands at, its
// title, whether it is a draft, its review decision and the checks of its last
// commit counted; one from a fork is left out, and a branch without one is
// absent. An empty branch, a worktree on none, is not asked of: with no branch
// left, gh is not run. It asks one GraphQL query per prsPerQuery branches, so a
// repository with many pull requests costs no more. Every failure wraps
// worktree.ErrPullRequestsUnavailable but an interrupted read, which is the
// context's error; every query is asked, one that fails keeping the pull
// requests it found, and its failure joined to the others.
func (g GitHub) PullRequestsOf(ctx context.Context, branches []string) ([]worktree.PullRequest, error) {
	branches = slices.DeleteFunc(slices.Clone(branches), func(branch string) bool { return branch == "" })
	if len(branches) == 0 {
		return nil, nil
	}

	binary, err := g.signedIn(ctx)
	if err != nil {
		return nil, err
	}

	var found []worktree.PullRequest
	var errs []error
	for batch := range slices.Chunk(branches, prsPerQuery) {
		out, err := g.run(ctx, binary, "gh api graphql", newestOf(batch)...)
		if ctx.Err() != nil {
			return found, ctx.Err()
		}
		prs, parseErr := parseNewest(out)
		found = append(found, prs...)
		if err := cmp.Or(err, parseErr); err != nil {
			errs = append(errs, err)
		}
	}

	return found, errors.Join(errs...)
}

// openLimit is how many open pull requests are listed, the newest first, as the legacy picker lists them.
const openLimit = 50

// prJSON are the fields gh pr list and gh pr view read of a pull request.
const prJSON = "number,url,state,headRefName,headRefOid,baseRefName,isCrossRepository,title,isDraft,author"

// OpenPullRequests returns the repository's open pull requests, the newest first and at most openLimit, narrowed by filter, each with its branch, the branch it would merge into, its author, whether it is a draft and whether its branch is a fork's. Every failure wraps worktree.ErrPullRequestsUnavailable but an interrupted read, which is the context's error.
func (g GitHub) OpenPullRequests(ctx context.Context, filter worktree.PullRequestFilter) ([]worktree.PullRequest, error) {
	binary, err := g.signedIn(ctx)
	if err != nil {
		return nil, err
	}

	args := []string{"pr", "list", "--state", "open", "--limit", strconv.Itoa(openLimit), "--json", prJSON}
	switch filter {
	case worktree.MyPullRequests:
		args = append(args, "--author", "@me")
	case worktree.ReviewRequested:
		args = append(args, "--search", "review-requested:@me")
	case worktree.AllPullRequests:
	}
	out, err := g.run(ctx, binary, "gh pr list", args...)
	if err != nil {
		return nil, err
	}

	var listed []listedPR
	if err := json.Unmarshal(out, &listed); err != nil {
		return nil, fmt.Errorf("%w: gh pr list: unexpected output: %w", worktree.ErrPullRequestsUnavailable, err)
	}
	prs := make([]worktree.PullRequest, 0, len(listed))
	for _, pr := range listed {
		prs = append(prs, pr.pullRequest())
	}

	return prs, nil
}

// PullRequest returns the pull request number, whatever its state. A number the repository has none of is worktree.ErrPullRequestNotFound; every other failure wraps worktree.ErrPullRequestsUnavailable but an interrupted read, which is the context's error.
func (g GitHub) PullRequest(ctx context.Context, number int) (worktree.PullRequest, error) {
	binary, err := g.signedIn(ctx)
	if err != nil {
		return worktree.PullRequest{}, err
	}

	out, err := g.run(ctx, binary, "gh pr view", "pr", "view", strconv.Itoa(number), "--json", prJSON)
	switch {
	case err != nil && strings.Contains(err.Error(), "Could not resolve to a PullRequest"):
		return worktree.PullRequest{}, fmt.Errorf("pull request #%d: %w", number, worktree.ErrPullRequestNotFound)
	case err != nil:
		return worktree.PullRequest{}, err
	}

	var viewed listedPR
	if err := json.Unmarshal(out, &viewed); err != nil {
		return worktree.PullRequest{}, fmt.Errorf("%w: gh pr view: unexpected output: %w", worktree.ErrPullRequestsUnavailable, err)
	}

	return viewed.pullRequest(), nil
}

// A listedPR is a pull request as gh pr list and gh pr view print it with prJSON.
type listedPR struct {
	Number            int    `json:"number"`
	URL               string `json:"url"`
	State             string `json:"state"`
	HeadRefName       string `json:"headRefName"`
	HeadRefOid        string `json:"headRefOid"`
	BaseRefName       string `json:"baseRefName"`
	IsCrossRepository bool   `json:"isCrossRepository"`
	Title             string `json:"title"`
	IsDraft           bool   `json:"isDraft"`
	Author            struct {
		Login string `json:"login"`
	} `json:"author"`
}

func (pr listedPR) pullRequest() worktree.PullRequest {
	return worktree.PullRequest{
		Number: pr.Number, URL: pr.URL, Branch: pr.HeadRefName, Head: pr.HeadRefOid, State: worktree.PullRequestState(strings.ToLower(pr.State)),
		Title: pr.Title, Draft: pr.IsDraft, Base: pr.BaseRefName, Author: pr.Author.Login, Fork: pr.IsCrossRepository,
	}
}

// signedIn returns the path of gh once it is found and signed in.
func (g GitHub) signedIn(ctx context.Context) (string, error) {
	// What failed matters less than what to do about it: the cause is left out.
	binary, err := exec.LookPath(cmp.Or(g.Binary, cli))
	if err != nil {
		return "", fmt.Errorf("%w: install gh, GitHub's CLI", worktree.ErrPullRequestsUnavailable)
	}

	if err := shell.Command(ctx, binary, "auth", "status").Run(); err != nil {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		return "", fmt.Errorf("%w: run `gh auth login`", worktree.ErrPullRequestsUnavailable)
	}

	return binary, nil
}

// run runs binary with args in the repository and returns its output, kept when
// it fails: gh api graphql prints the data it resolved beside the errors of what
// it could not, a field the token may not read. What names the command in its
// failure.
func (g GitHub) run(ctx context.Context, binary, what string, args ...string) ([]byte, error) {
	cmd := shell.Command(ctx, binary, args...)
	cmd.Dir = g.Dir
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	switch {
	case err != nil && strings.Contains(stderr.String(), "none of the git remotes configured for this repository point to a known GitHub host"):
		return out, fmt.Errorf("%w: origin is not a GitHub repository", worktree.ErrPullRequestsUnavailable)
	case err != nil:
		return out, fmt.Errorf("%w: %s: %w: %s", worktree.ErrPullRequestsUnavailable, what, err, strings.TrimSpace(stderr.String()))
	}

	return out, nil
}

// prFields are the fields read of each pull request: the last commit's check runs and statuses come counted by state, which costs one context node.
const prFields = "number url state headRefName headRefOid isCrossRepository title isDraft reviewDecision " +
	"commits(last: 1) { nodes { commit { statusCheckRollup { contexts(first: 1) { checkRunCountsByState { state count } statusContextCountsByState { state count } } } } } }"

// newestOf is gh's arguments of the GraphQL query for the newest pull requests of branches, one aliased field per branch, each name a variable never spliced into the query; gh fills {owner} and {repo} from the repository it runs in.
func newestOf(branches []string) []string {
	variables := []string{"$owner: String!", "$name: String!"}
	args := []string{"api", "graphql", "-F", "owner={owner}", "-F", "name={repo}"}
	fields := make([]string, 0, len(branches))
	for i, branch := range branches {
		alias := "b" + strconv.Itoa(i)
		variables = append(variables, "$"+alias+": String!")
		fields = append(fields, alias+": pullRequests(headRefName: $"+alias+", first: "+strconv.Itoa(newestPerBranch)+", orderBy: {field: CREATED_AT, direction: DESC}) { nodes { "+prFields+" } }")
		args = append(args, "-f", alias+"="+branch)
	}
	query := "query(" + strings.Join(variables, ", ") + ") { repository(owner: $owner, name: $name) { " + strings.Join(fields, " ") + " } }"

	return append(args, "-f", "query="+query)
}

// A count is how many of a commit's check runs, or statuses, are in a state.
type count struct {
	State string `json:"state"`
	Count int    `json:"count"`
}

// A node is a pull request as newestOf reads it.
type node struct {
	Number            int    `json:"number"`
	URL               string `json:"url"`
	State             string `json:"state"`
	HeadRefName       string `json:"headRefName"`
	HeadRefOid        string `json:"headRefOid"`
	IsCrossRepository bool   `json:"isCrossRepository"`
	Title             string `json:"title"`
	IsDraft           bool   `json:"isDraft"`
	ReviewDecision    string `json:"reviewDecision"`
	Commits           struct {
		Nodes []struct {
			Commit struct {
				StatusCheckRollup *struct {
					Contexts struct {
						CheckRuns []count `json:"checkRunCountsByState"`
						Statuses  []count `json:"statusContextCountsByState"`
					} `json:"contexts"`
				} `json:"statusCheckRollup"`
			} `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
}

// parseNewest reads the answer of newestOf: per branch, among its newest pull requests that are not a fork's, the open one, else the newest; its state lowercased.
func parseNewest(out []byte) ([]worktree.PullRequest, error) {
	var answer struct {
		Data struct {
			Repository map[string]struct {
				Nodes []*node `json:"nodes"`
			} `json:"repository"`
		} `json:"data"`
	}
	if err := json.Unmarshal(out, &answer); err != nil {
		return nil, fmt.Errorf("%w: gh api graphql: unexpected output: %w", worktree.ErrPullRequestsUnavailable, err)
	}

	var prs []worktree.PullRequest
	for _, alias := range slices.Sorted(maps.Keys(answer.Data.Repository)) {
		// GraphQL nulls a whole node when one of its fields fails.
		nodes := slices.DeleteFunc(answer.Data.Repository[alias].Nodes, func(pr *node) bool { return pr == nil })
		at := slices.IndexFunc(nodes, func(pr *node) bool { return !pr.IsCrossRepository && pr.State == "OPEN" })
		if at < 0 {
			at = slices.IndexFunc(nodes, func(pr *node) bool { return !pr.IsCrossRepository })
		}
		if at < 0 {
			continue
		}
		pr := nodes[at]
		prs = append(prs, worktree.PullRequest{
			Number: pr.Number, URL: pr.URL, Branch: pr.HeadRefName, Head: pr.HeadRefOid, State: worktree.PullRequestState(strings.ToLower(pr.State)),
			Title: pr.Title, Draft: pr.IsDraft, Checks: checksOf(*pr), Decision: pr.ReviewDecision,
		})
	}

	return prs, nil
}

// checksOf counts the check runs and statuses of pr's last commit by outcome.
func checksOf(pr node) worktree.Checks {
	var checks worktree.Checks
	for _, commit := range pr.Commits.Nodes {
		rollup := commit.Commit.StatusCheckRollup
		if rollup == nil {
			continue
		}
		for _, c := range rollup.Contexts.CheckRuns {
			add(&checks, checkRunOutcome(c.State), c.Count)
		}
		for _, c := range rollup.Contexts.Statuses {
			add(&checks, statusOutcome(c.State), c.Count)
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

// add counts n checks of outcome o in checks.
func add(checks *worktree.Checks, o outcome, n int) {
	switch o {
	case passed:
		checks.Passed += n
	case failed:
		checks.Failed += n
	case pending:
		checks.Pending += n
	}
}

// checkRunOutcome is the outcome of a check run in state, GitHub's CheckRunState, its conclusion once it ran: a skipped or neutral one passes; one still to run (queued, waiting, pending, in progress) is pending, as is one that needs authorisation to run (ACTION_REQUIRED); an unknown state fails closed, never passing silently.
func checkRunOutcome(state string) outcome {
	switch state {
	case "SUCCESS", "NEUTRAL", "SKIPPED":
		return passed
	case "ACTION_REQUIRED", "QUEUED", "IN_PROGRESS", "PENDING", "WAITING":
		return pending
	}
	return failed
}

// statusOutcome is the outcome of a status of the older Status API in state, GitHub's StatusState: pending while it is PENDING or still EXPECTED; an unknown state fails closed.
func statusOutcome(state string) outcome {
	switch state {
	case "SUCCESS":
		return passed
	case "PENDING", "EXPECTED":
		return pending
	}
	return failed
}
