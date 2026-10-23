package run

import (
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// Asked is what a read asks besides git while it reads the worktrees: the run daemon's jobs and the forge's pull requests, each with the error that kept it from being read, and whether the read was interrupted; PullRequests is nil when the forge was not asked.
type Asked struct {
	Jobs         []Job
	JobsErr      error
	PullRequests []worktree.PullRequest
	PRsErr       error
	Interrupted  bool
}

// Services returns the names of the jobs up in the worktree at dir, sorted, nil when the daemon could not tell.
func (a Asked) Services(dir string) []string {
	if a.JobsErr != nil {
		return nil
	}

	return UpIn(a.Jobs, dir)
}

// Warnings are the warnings of what the read of the command named command could not ask: <command>.jobs_unknown when the daemon could not tell which jobs are up, <command>.prs_unavailable when the forge could not be asked; none once the read was interrupted, which says so and nothing more.
func (a Asked) Warnings(command string) []domain.Warning {
	if a.Interrupted {
		return nil
	}

	var warnings []domain.Warning
	if a.JobsErr != nil {
		warnings = append(warnings, domain.Warning{Code: command + domain.JobsUnknown, Message: UnknownMessage(a.JobsErr)})
	}
	if a.PRsErr != nil {
		warnings = append(warnings, domain.Warning{Code: command + domain.PRsUnavailable, Message: a.PRsErr.Error()})
	}

	return warnings
}
