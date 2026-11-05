package cli

import (
	"context"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/command/runstart"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/LucasPcq/wtm/next/internal/ui/runview"
	"github.com/spf13/cobra"
)

type ran = form.Ran[runstart.Request, runstart.Outcome]

// conclude opens the live view on the job a start left up, then draws the
// outcome under it: the view changes nothing of how the run ended, nor of its
// exit code.
func (c *startCommand) conclude(cmd *cobra.Command, r ran, err error) error {
	if err == nil && c.watches(cmd.Context(), r) {
		if failed := runview.Run(cmd.Context(), c.viewOf(r.Outcome)); failed != nil {
			output.WarningLine(c.io.Streams.Err, failed.Error())
		}
	}
	return c.form.Conclude(cmd, r, err)
}

// watches reports whether the start is shown live: it applied, was not
// interrupted and left its job up (never a task, gone once done), for a
// person at a terminal who neither detached it nor asked for quiet.
func (c *startCommand) watches(ctx context.Context, r ran) bool {
	if c.view.Daemon == nil || r.Request.Detach || !c.io.Human() || c.io.Quiet || !c.io.Streams.CanPrompt() {
		return false
	}
	if ctx.Err() != nil || !r.Outcome.Applied() || r.Outcome.Result == nil {
		return false
	}
	items := r.Outcome.Result.Items
	if slices.ContainsFunc(items, func(item runstart.Item) bool { return item.Status == domain.StatusCancelled }) {
		return false
	}
	return slices.ContainsFunc(items, func(item runstart.Item) bool {
		return item.Detail != nil && item.Detail.LeftUp()
	})
}

// viewOf is the live view of outcome: the worktree with the job, selected, the
// namespace it holds, the main checkout a shared service runs in, and as
// notices the failures and the warnings of the start.
func (c *startCommand) viewOf(outcome runstart.Outcome) runview.Params {
	params := c.view.Params()
	params.Job = outcome.Plan.Job
	params.In = c.io.Streams.In
	params.Out = c.io.Streams.Err
	for _, target := range outcome.Plan.Worktrees {
		wt := runview.Worktree{Branch: target.Worktree, Path: target.Path, Namespaces: map[string]string{}}
		for _, job := range target.Jobs {
			wt.Jobs = append(wt.Jobs, job.Name)
			if job.Namespace != "" {
				wt.Namespaces[job.Name] = job.Namespace
			}
			if params.Main == "" {
				params.Main = job.SharedIn
			}
		}
		params.Worktrees = append(params.Worktrees, wt)
	}

	result := outcome.Result
	warnings := slices.Concat(outcome.Warnings, result.Warnings)
	for _, item := range result.Items {
		if item.Err != nil {
			params.Notices = append(params.Notices, item.Subject+": "+item.Err.Message)
		}
		warnings = append(warnings, item.Warnings...)
	}
	for _, warning := range warnings {
		params.Notices = append(params.Notices, warning.Message)
	}
	return params
}
