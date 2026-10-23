package repository

import (
	"context"
	"sync"

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
func Ask(ctx context.Context, params AskParams, readWorktrees func()) run.Asked {
	var asked run.Asked
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
