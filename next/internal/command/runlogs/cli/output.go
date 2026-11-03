package cli

import (
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/runlogs"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// draw draws what was read for a person: each job's lines, `[job] text` (`[job · worktree]` over several worktrees), a worktree where no job ran, a job named that never ran, then what could not be read; nothing of the jobs once the live view showed them.
func (c *logsCommand) draw(w io.Writer, read runlogs.Outcome, _ runlogs.Request, _ output.View) {
	if !c.viewed {
		prefix := prefixer(len(read.Items) > 1)
		for _, item := range read.Items {
			drawItem(w, item, prefix)
		}
	}
	output.ReadEnd(w, read)
}

func drawItem(w io.Writer, item runlogs.Item, prefix func(worktree, job string) string) {
	if item.Detail == nil {
		return
	}
	if len(item.Detail.Jobs) == 0 {
		output.UnchangedLine(w, item.Subject+": no job has run")
		return
	}
	for _, job := range item.Detail.Jobs {
		if job.State == runlogs.JobNotStarted {
			output.UnchangedLine(w, fmt.Sprintf("%s: %s has not run", item.Subject, job.Name))
			continue
		}
		for _, line := range job.Lines {
			drawLine(w, prefix(item.Subject, job.Name), line.Text)
		}
	}
}

// prefixer is how a line says where it comes from: its job, and its worktree over several.
func prefixer(several bool) func(worktree, job string) string {
	return func(worktree, job string) string {
		if several {
			return "[" + job + output.Dot + worktree + "]"
		}
		return "[" + job + "]"
	}
}

// drawLine draws a line a job printed under prefix, muted: the prefix is chrome, the job's text its own.
func drawLine(w io.Writer, prefix, text string) {
	fmt.Fprintf(w, "%s%s %s\n", output.Indent, output.StylesOf(w).Muted.Render(prefix), text)
}
