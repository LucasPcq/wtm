package repository

import (
	"context"
	"errors"
	"sync"

	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
)

// AskParams are what a read asks besides git: the run daemon's jobs, and, WithPRs, the forge's pull request of each of Branches; Restart is the command that hands a skewed daemon's jobs over to this wtm's, offered when one answers.
type AskParams struct {
	Jobs         func(ctx context.Context) ([]run.Job, error)
	Restart      *domain.FollowUp
	PullRequests func(ctx context.Context, branches []string) ([]worktree.PullRequest, error)
	Branches     []string
	WithPRs      bool
}

// Ask asks what params names while readWorktrees runs, all three at once, and returns what was said once they are done; the read is interrupted when ctx is done by then.
func Ask(ctx context.Context, params AskParams, readWorktrees func()) Asked {
	var asked Asked
	var wg sync.WaitGroup
	wg.Go(func() {
		asked.Jobs, asked.JobsErr = params.Jobs(ctx)
		if skewed, ok := errors.AsType[*run.Skewed](asked.JobsErr); ok {
			asked.Skewed, asked.JobsErr = skewed, nil
		}
	})
	asked.restart = params.Restart
	if params.WithPRs {
		wg.Go(func() { asked.PullRequests, asked.PRsErr = params.PullRequests(ctx, params.Branches) })
	}
	readWorktrees()
	wg.Wait()

	asked.Interrupted = ctx.Err() != nil
	return asked
}

// Asked is what a read asks besides git while it reads the worktrees: the run daemon's jobs and the forge's pull requests, each with the error that kept it from being read, the daemon's skew when one of another build answered, and whether the read was interrupted; PullRequests is nil when the forge was not asked.
type Asked struct {
	Jobs         []run.Job
	JobsErr      error
	Skewed       *run.Skewed
	restart      *domain.FollowUp
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

// Warnings are the warnings of what the read of the command named command could not ask: <command>.jobs_unknown when the daemon could not tell which jobs are up, <command>.daemon_skew when one of another build answered (with the restart that hands its jobs over), <command>.prs_unavailable when the forge could not be asked; none once the read was interrupted, which says so and nothing more.
func (a Asked) Warnings(command string) []domain.Warning {
	if a.Interrupted {
		return nil
	}

	var warnings []domain.Warning
	if a.JobsErr != nil {
		warnings = append(warnings, domain.Warning{Code: command + domain.JobsUnknown, Message: run.UnknownMessage(a.JobsErr)})
	}
	if a.Skewed != nil {
		warnings = append(warnings, SkewWarning(SkewWarningParams{Command: command, Skewed: a.Skewed, Restart: a.restart}))
	}
	if a.PRsErr != nil {
		warnings = append(warnings, domain.Warning{Code: command + domain.PRsUnavailable, Message: a.PRsErr.Error(), Params: worktree.UnavailableParams(a.PRsErr)})
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

// SkewWarningParams are a daemon of another build that answered the command named Command, and the follow-up that restarts it.
type SkewWarningParams struct {
	Command string
	Skewed  *run.Skewed
	Restart *domain.FollowUp
}

// SkewWarning is <command>.daemon_skew: the daemon that answered is of another build, or the legacy wtm's, its skew and build in its params.
func SkewWarning(params SkewWarningParams) domain.Warning {
	warning := domain.Warning{Code: params.Command + domain.DaemonSkew, Message: params.Skewed.Error(), Params: domain.Params{"skew": string(params.Skewed.Skew), "daemon": params.Skewed.Daemon}}
	if params.Restart != nil {
		restart := *params.Restart
		restart.Code, restart.Message = params.Command+".restart_daemon", "restart it: its services stop, its stacks are kept"
		warning.FollowUp = &restart
	}
	return warning
}
