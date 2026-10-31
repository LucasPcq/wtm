package repository

import (
	"context"
	"errors"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// AskParams are what a read asks besides git: the run daemon's jobs, and, WithPRs, the forge's pull request of each of Branches.
type AskParams struct {
	Jobs         func(ctx context.Context) ([]run.Job, error)
	PullRequests func(ctx context.Context, branches []string) ([]worktree.PullRequest, error)
	Branches     []string
	WithPRs      bool
}

// Ask asks what params names while readWorktrees runs, all three at once, and returns what was said once they are done; the read is interrupted when ctx is done by then.
func Ask(ctx context.Context, params AskParams, readWorktrees func()) Asked {
	var asked Asked
	var wg sync.WaitGroup
	wg.Go(func() { asked.Jobs, asked.JobsErr = params.Jobs(ctx) })
	if params.WithPRs {
		wg.Go(func() { asked.PullRequests, asked.PRsErr = params.PullRequests(ctx, params.Branches) })
	}
	readWorktrees()
	wg.Wait()

	asked.Interrupted = ctx.Err() != nil
	return asked
}

// Asked is what a read asks besides git while it reads the worktrees: the run daemon's jobs and the forge's pull requests, each with the error that kept it from being read, and whether the read was interrupted; PullRequests is nil when the forge was not asked.
type Asked struct {
	Jobs         []run.Job
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

	return run.UpIn(a.Jobs, dir)
}

// URLs are the URLs of the jobs up in the worktree at dir, nil when the daemon could not tell.
func (a Asked) URLs(dir string) []string {
	if a.JobsErr != nil {
		return nil
	}

	return run.URLsIn(a.Jobs, dir)
}

// Warnings are the warnings of what the read of the command named command could not ask: <command>.jobs_unknown when the daemon could not tell which jobs are up, <command>.prs_unavailable when the forge could not be asked; none once the read was interrupted, which says so and nothing more.
func (a Asked) Warnings(command string) []domain.Warning {
	if a.Interrupted {
		return nil
	}

	var warnings []domain.Warning
	if a.JobsErr != nil {
		warnings = append(warnings, domain.Warning{Code: command + domain.JobsUnknown, Message: run.UnknownMessage(a.JobsErr)})
	}
	if a.PRsErr != nil {
		warnings = append(warnings, domain.Warning{Code: command + domain.PRsUnavailable, Message: a.PRsErr.Error(), Params: domain.UnavailableParams(a.PRsErr)})
	}

	return warnings
}

// Interrupted is err when it is the read's interruption, which cancels the worktree being read; any other error of a count git cannot make leaves it not counted rather than unreadable, nil unless ctx is done by then.
func Interrupted(ctx context.Context, err error) error {
	if err == nil || errors.Is(err, context.Canceled) {
		return err
	}

	return ctx.Err()
}
