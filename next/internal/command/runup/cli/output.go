package cli

import (
	"fmt"
	"io"
	"slices"

	"github.com/LucasPcq/wtm/next/internal/command/runup"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

const addressesTitle = "Addresses"

// drawOutcome draws the conclusion of the start: the tally, the plan but the drops owed it planned, where each job answers, each drop owed made, and for a worktree whose start failed the last lines and the log of the job it failed on.
func drawOutcome(w io.Writer, outcome runup.Outcome, view output.View) {
	outcome.Plan.Recap = slices.DeleteFunc(slices.Clone(outcome.Plan.Recap), func(row domain.RecapRow) bool { return row.Key == runup.RecapOwed })
	block := func(w io.Writer) {
		addresses(outcome)(w)
		drawPaid(w, outcome.Result.Items)
	}
	output.DrawApplied(w, outcome, output.AppliedParams{Command: Use, View: view, Block: block, Excerpt: excerpt(outcome)})
}

// drawPaid draws a line for each drop owed the start made.
func drawPaid(w io.Writer, items []runup.Item) {
	first := true
	for _, item := range items {
		if item.Detail == nil || item.Detail.Owed == nil || item.Status != domain.StatusDone {
			continue
		}
		if first {
			output.Blank(w)
			first = false
		}
		debt := item.Detail.Owed
		output.SuccessLine(w, "dropped "+debt.Name+" from "+debt.Job+", owed since "+debt.Worktree+" was removed")
	}
}

// addresses draws where each job that answers at a URL is reached, one job per line, the URLs in one column, under each worktree's branch when the start ran in several.
func addresses(outcome runup.Outcome) func(io.Writer) {
	type reached struct {
		subject string
		jobs    []runup.JobResult
	}

	return func(w io.Writer) {
		var all []reached
		width := 0
		for _, item := range outcome.Result.Items {
			if item.Detail == nil {
				continue
			}
			if jobs := item.Detail.Reached(); len(jobs) > 0 {
				all = append(all, reached{subject: item.Subject, jobs: jobs})
				for _, job := range jobs {
					width = max(width, len(job.Name))
				}
			}
		}
		if len(all) == 0 {
			return
		}

		output.Blank(w)
		output.SectionTitle(w, addressesTitle)
		for _, r := range all {
			indent := output.Indent
			if len(outcome.Plan.Worktrees) > 1 {
				output.Message(w, output.Indent+r.subject)
				indent += output.Indent
			}
			for _, job := range r.jobs {
				output.Message(w, fmt.Sprintf("%s%-*s  %s", indent, width, job.Name, job.URL))
			}
		}
	}
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
