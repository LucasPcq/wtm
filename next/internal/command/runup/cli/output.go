package cli

import (
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/runup"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// drawOutcome draws the conclusion of the start: the tally, the plan with where each job answers, and for a worktree whose start failed the last lines and the log of the job it failed on.
func drawOutcome(w io.Writer, outcome runup.Outcome, view output.View) {
	output.DrawApplied(w, outcome, output.AppliedParams{Command: Use, View: view, Excerpt: excerpt(outcome)})
}

// excerpt is the last lines and the log of the job each worktree's start failed on, by its name.
func excerpt(outcome runup.Outcome) func(subject string) ([]string, string) {
	failed := map[string]runup.JobResult{}
	for _, item := range outcome.Result.Items {
		if item.Detail == nil {
			continue
		}
		for _, job := range item.Detail.Jobs {
			if job.Status == runup.JobFailed || job.Status == runup.JobCrashed {
				failed[item.Subject] = job
				break
			}
		}
	}

	return func(subject string) ([]string, string) {
		job := failed[subject]
		return job.Lines, job.Log
	}
}
