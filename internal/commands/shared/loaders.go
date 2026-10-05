package shared

import (
	"context"

	"github.com/LucasPcq/wtm/internal/domain"
	ghservice "github.com/LucasPcq/wtm/internal/service/github"
	"github.com/LucasPcq/wtm/internal/service/runjobs"
)

// LoadPRs fetches open PRs for the project and reports the GitHub CLI
// connection status, distinguishing "no PRs" from "gh unavailable" so callers
// can hint the user. Returns nil PRs on any error.
func LoadPRs(ctx context.Context, projectDir string) ([]domain.PRInfo, domain.GHConnection) {
	return LoadPRsFiltered(ctx, projectDir, domain.PRFilterAll)
}

// LoadPRsWithChecks is LoadPRs plus the CI rollup and the review decision. Only
// the dashboard renders those, and asking for them costs a per-pull-request
// resolution, so the other surfaces stay on the narrow field set.
func LoadPRsWithChecks(ctx context.Context, projectDir string) ([]domain.PRInfo, domain.GHConnection) {
	return loadPRs(ctx, loadPRsParams{ProjectDir: projectDir, Filter: domain.PRFilterAll, WithChecks: true})
}

// LoadPRsFiltered is LoadPRs with an explicit filter (all, mine, review-requested).
func LoadPRsFiltered(ctx context.Context, projectDir string, filter domain.PRFilter) ([]domain.PRInfo, domain.GHConnection) {
	return loadPRs(ctx, loadPRsParams{ProjectDir: projectDir, Filter: filter})
}

type loadPRsParams struct {
	ProjectDir string
	Filter     domain.PRFilter
	WithChecks bool
}

func loadPRs(ctx context.Context, params loadPRsParams) ([]domain.PRInfo, domain.GHConnection) {
	return ghservice.ListOpenPRsWithConnection(ctx, ghservice.ListPRsParams{
		ProjectDir: params.ProjectDir,
		Filter:     params.Filter,
		WithChecks: params.WithChecks,
	})
}

// LoadJobsGraceful fetches the daemon's jobs, returning nil when there are none
// to fetch.
func LoadJobsGraceful() []domain.JobInfo { return runjobs.Load() }

// LoadJobs is LoadJobsGraceful for the callers whose whole output is that list,
// and which therefore have to report a daemon of another build.
func LoadJobs() runjobs.Listing { return runjobs.List() }
