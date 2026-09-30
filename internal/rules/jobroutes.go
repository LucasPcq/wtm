package rules

import "github.com/LucasPcq/wtm/internal/domain"

type JobRoutesParams struct {
	Config domain.RunConfig
	Job    domain.JobConfig
	// Worktree and Project are the two labels that place a host, as everywhere
	// else: <job>.<worktree>.<project>.localhost.
	Worktree string
	Project  string
}

// JobRoutes is every name the proxy must serve once this job is started: its
// own, and one per published job it runs.
//
// The children are the whole point. A runner is one process — `turbo run dev`,
// a compose stack — and the apps behind it are never started as jobs, so the
// daemon would publish nothing for them and their names would answer 404 while
// their ports worked. The ports already reach them: EffectiveJobPorts gives the
// runner its children's, which is what makes the resolved port behind each of
// these hosts a real one.
//
// A job publishing nothing contributes nothing, runner or not — the proxy only
// speaks HTTP, and a name nothing answers under is worse than no name.
func JobRoutes(params JobRoutesParams) []domain.JobRoute {
	var routes []domain.JobRoute
	add := func(job domain.JobConfig) {
		if job.URL == nil {
			return
		}
		host := RouteHost(RouteHostParams{Job: job, Worktree: params.Worktree, Project: params.Project})
		if host == "" {
			return
		}
		routes = append(routes, domain.JobRoute{Job: job.Name, Host: host, Port: job.URL.Port})
	}

	add(params.Job)
	byName := jobsByName(params.Config)
	for _, child := range RunnerChildren(params.Config, params.Job.Name) {
		add(byName[child])
	}
	return routes
}

// JobOwnRoute is the name a job publishes for itself, empty for one that
// publishes none. A surface reporting a single job — `run ps`, a start line —
// says where that job answers, never where its children do.
func JobOwnRoute(routes []domain.JobRoute, job string) string {
	for _, route := range routes {
		if route.Job == job {
			return route.Host
		}
	}
	return ""
}
