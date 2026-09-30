package rules

import "github.com/LucasPcq/wtm/internal/domain"

// TasksFirst orders a run's jobs so every task precedes every service, stable
// within each group. A task is a step a service depends on — a migration, a
// seed — and package.json scripts arrive here in alphabetical order, which puts
// `dev` ahead of `migrate` and starts the server on a database nothing has
// migrated.
func TasksFirst(jobs []domain.JobConfig) []domain.JobConfig {
	ordered := make([]domain.JobConfig, 0, len(jobs))
	for _, job := range jobs {
		if job.Kind == domain.JobKindTask {
			ordered = append(ordered, job)
		}
	}
	for _, job := range jobs {
		if job.Kind != domain.JobKindTask {
			ordered = append(ordered, job)
		}
	}
	return ordered
}

// JobNames reads the names off a job list, keeping its order.
func JobNames(jobs []domain.JobConfig) []string {
	names := make([]string, 0, len(jobs))
	for _, job := range jobs {
		names = append(names, job.Name)
	}
	return names
}
