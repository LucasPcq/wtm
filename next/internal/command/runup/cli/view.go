package cli

import (
	"slices"

	"github.com/LucasPcq/wtm/next/internal/command/runup"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
	"github.com/LucasPcq/wtm/next/internal/ui/runview"
	"github.com/spf13/cobra"
)

// View is where run up shows the jobs it started: the run daemon its live
// view asks, and how it opens an address in the browser (nil opens none). A
// zero View shows none.
type View struct {
	Daemon runview.Daemon
	Open   func(url string) error
}

type ran = form.Ran[runup.Request, runup.Outcome]

// conclude opens the live view on the jobs a start left up, once every one is
// started, then draws the outcome under it: the view changes nothing of how
// the run ended, nor of its exit code.
func (c *upCommand) conclude(cmd *cobra.Command, r ran, err error) error {
	if err == nil && c.watches(r) {
		if failed := runview.Run(cmd.Context(), c.viewOf(r.Outcome)); failed != nil {
			output.WarningLine(c.io.Streams.Err, failed.Error())
		}
	}
	return c.form.Conclude(cmd, r, err)
}

// watches reports whether the start is shown live: it applied and left a job
// up, for a person at a terminal who neither detached it nor asked for quiet.
func (c *upCommand) watches(r ran) bool {
	if c.view.Daemon == nil || r.Request.Detach || !c.io.Human() || c.io.Quiet || !c.io.Streams.CanPrompt() {
		return false
	}
	if !r.Outcome.Applied() || r.Outcome.Result == nil {
		return false
	}
	return slices.ContainsFunc(r.Outcome.Result.Items, func(item runup.Item) bool {
		return item.Detail != nil && item.Detail.LeftUp()
	})
}

// viewOf is the live view of outcome: each worktree of the start with the jobs
// of its profile, the main checkout the shared services run in, and as
// notices the failures and the warnings of the start.
func (c *upCommand) viewOf(outcome runup.Outcome) runview.Params {
	params := runview.Params{Daemon: c.view.Daemon, Open: c.view.Open, In: c.io.Streams.In, Out: c.io.Streams.Err}
	for _, target := range outcome.Plan.Worktrees {
		wt := runview.Worktree{Branch: target.Worktree, Path: target.Path}
		for _, job := range target.Jobs {
			wt.Jobs = append(wt.Jobs, job.Name)
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
