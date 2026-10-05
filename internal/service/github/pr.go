package github

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/internal/domain"
)

// HasOpenPRParams holds inputs for checking open PRs.
type HasOpenPRParams struct {
	ProjectDir string
	Branch     string
}

// HasOpenPR checks if a branch has an open PR via the gh CLI. Returns the PR
// number and URL when found. Returns false gracefully if gh is not installed,
// not authenticated, or the call fails.
func HasOpenPR(ctx context.Context, params HasOpenPRParams) (bool, int, string) {
	if err := ensureAuth(ctx); err != nil {
		return false, 0, ""
	}

	type prItem struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
	}

	data, err := runGH(ctx, params.ProjectDir, "pr", "list",
		"--head", params.Branch,
		"--state", "open",
		"--json", "number,url",
		"--limit", "1",
	)
	if err != nil {
		return false, 0, ""
	}

	items, err := parseJSON[[]prItem](data)
	if err != nil || len(items) == 0 {
		return false, 0, ""
	}

	return true, items[0].Number, items[0].URL
}

const openPRsLimit = 200

// OpenPRsByBranch maps each branch with an open pull request to its URL, in one
// call however many worktrees ask. It answers an empty map wherever HasOpenPR
// would answer false. complete is false when the list hit its limit: a branch
// missing from it may then still have an open pull request.
func OpenPRsByBranch(ctx context.Context, projectDir string) (open map[string]string, complete bool) {
	open = map[string]string{}
	if err := ensureAuth(ctx); err != nil {
		return open, true
	}

	type prItem struct {
		Branch string `json:"headRefName"`
		URL    string `json:"url"`
	}

	data, err := runGH(ctx, projectDir, "pr", "list",
		"--state", "open",
		"--json", "headRefName,url",
		"--limit", strconv.Itoa(openPRsLimit),
	)
	if err != nil {
		return open, true
	}
	items, err := parseJSON[[]prItem](data)
	if err != nil {
		return open, true
	}
	for _, item := range items {
		if _, seen := open[item.Branch]; !seen {
			open[item.Branch] = item.URL
		}
	}
	return open, len(items) < openPRsLimit
}

// ListPRsParams holds inputs for listing pull requests.
type ListPRsParams struct {
	ProjectDir string
	Filter     domain.PRFilter
	// WithChecks asks for the CI rollup and review decision, which cost a
	// per-pull-request resolution. Only a caller that renders them sets it.
	WithChecks bool
}

// ListPRs fetches open PRs via gh CLI and filters them.
func ListPRs(ctx context.Context, params ListPRsParams) ([]domain.PRInfo, error) {
	if err := ensureAuth(ctx); err != nil {
		return nil, err
	}

	fields := domain.GHPRFields
	if params.WithChecks {
		fields = domain.GHPRFieldsWithChecks
	}
	args := []string{
		"pr", "list",
		"--state", "open",
		"--json", fields,
		"--limit", "50",
	}

	switch params.Filter {
	case domain.PRFilterMine:
		args = append(args, "--author", "@me")
	case domain.PRFilterReviewRequested:
		args = append(args, "--search", "review-requested:@me")
	}

	data, err := runGH(ctx, params.ProjectDir, args...)
	if err != nil {
		return nil, fmt.Errorf("list PRs: %w", err)
	}

	ghPRs, err := parseJSON[[]ghPR](data)
	if err != nil {
		return nil, err
	}

	prs := make([]domain.PRInfo, 0, len(ghPRs))
	for _, g := range ghPRs {
		prs = append(prs, convertGHPR(g))
	}
	return prs, nil
}

// ListPRsAllStates fetches PRs across every state (open, merged, closed) so a
// branch whose PR was merged or closed can be flagged as a clean candidate by
// `wtm tree --with-prs`. Results are newest-first; callers match by head branch
// and take the first hit. State is normalised to lowercase ("open"/"merged"/
// "closed").
func ListPRsAllStates(ctx context.Context, projectDir string) ([]domain.PRInfo, error) {
	if err := ensureAuth(ctx); err != nil {
		return nil, err
	}

	data, err := runGH(ctx, projectDir, "pr", "list",
		"--state", "all",
		"--json", domain.GHPRFieldsWithState,
		"--limit", "100",
	)
	if err != nil {
		return nil, fmt.Errorf("list PRs: %w", err)
	}

	type statePR struct {
		Number      int    `json:"number"`
		HeadRefName string `json:"headRefName"`
		URL         string `json:"url"`
		State       string `json:"state"`
	}

	items, err := parseJSON[[]statePR](data)
	if err != nil {
		return nil, err
	}

	prs := make([]domain.PRInfo, 0, len(items))
	for _, it := range items {
		prs = append(prs, domain.PRInfo{
			Number: it.Number,
			Branch: it.HeadRefName,
			URL:    it.URL,
			State:  strings.ToLower(it.State),
		})
	}
	return prs, nil
}

// ListPRsWithConnection is ListPRsAllStates with the CLI's availability kept
// alongside the result, so a caller can tell "gh unavailable" apart from "no
// PRs" and say so. It lives here rather than in commands/shared because the
// flow layer needs it and may not reach that far up.
func ListPRsWithConnection(ctx context.Context, projectDir string) ([]domain.PRInfo, domain.GHConnection) {
	prs, err := ListPRsAllStates(ctx, projectDir)
	return prs, connectionOf(err)
}

// ListOpenPRsWithConnection is ListPRs with the CLI's availability kept alongside
// the result, the way ListPRsWithConnection keeps it for every state.
func ListOpenPRsWithConnection(ctx context.Context, params ListPRsParams) ([]domain.PRInfo, domain.GHConnection) {
	prs, err := ListPRs(ctx, params)
	if err != nil {
		return nil, connectionOf(err)
	}
	return prs, domain.GHConnectionOK
}

// connectionOf reads a listing error as the CLI's availability. Any other
// failure still reads as connected: there is nothing the user could set up.
func connectionOf(err error) domain.GHConnection {
	switch {
	case errors.Is(err, domain.ErrGHNotInstalled):
		return domain.GHConnectionNotInstalled
	case errors.Is(err, domain.ErrGHNotAuthenticated):
		return domain.GHConnectionNotAuthenticated
	}
	return domain.GHConnectionOK
}

// GetPRDetailParams holds inputs for fetching a single PR's detail.
type GetPRDetailParams struct {
	ProjectDir string
	Number     int
}

// GetPRDetail fetches a single PR's identity and head/base branches via gh CLI.
func GetPRDetail(ctx context.Context, params GetPRDetailParams) (domain.PRInfo, error) {
	if err := ensureAuth(ctx); err != nil {
		return domain.PRInfo{}, err
	}

	data, err := runGH(ctx, params.ProjectDir, "pr", "view", strconv.Itoa(params.Number),
		"--json", domain.GHPRFields,
	)
	if err != nil {
		return domain.PRInfo{}, fmt.Errorf("get PR: %w", err)
	}

	g, err := parseJSON[ghPR](data)
	if err != nil {
		return domain.PRInfo{}, err
	}

	return convertGHPR(g), nil
}

// OpenPRParams holds inputs for opening a pull request in the browser.
type OpenPRParams struct {
	ProjectDir string
	Number     int
}

// OpenPR opens a pull request in the browser via `gh pr view --web`, run in
// the project directory: gh already knows the repository and carries its own
// authentication, which an OS-level URL opener would not.
func OpenPR(ctx context.Context, params OpenPRParams) error {
	if err := ensureAuth(ctx); err != nil {
		return err
	}
	_, err := runGH(ctx, params.ProjectDir, "pr", "view", "--web", strconv.Itoa(params.Number))
	if err != nil {
		return fmt.Errorf("open PR: %w", err)
	}
	return nil
}
