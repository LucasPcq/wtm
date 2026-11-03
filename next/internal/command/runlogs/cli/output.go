package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/LucasPcq/wtm/next/internal/command/runlogs"
	"github.com/LucasPcq/wtm/next/internal/domain"
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
		drawAbsent(w, item.Subject, "")
		return
	}
	for _, job := range item.Detail.Jobs {
		if job.State == runlogs.JobNotStarted {
			drawAbsent(w, item.Subject, job.Name)
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

// drawAbsent says a worktree where no job ran, or the job named, which never ran there.
func drawAbsent(w io.Writer, worktree, job string) {
	if job == "" {
		output.UnchangedLine(w, worktree+": no job has run")
		return
	}
	output.UnchangedLine(w, fmt.Sprintf("%s: %s has not run", worktree, job))
}

// The types of the lines of JSON a follow writes: a line a job printed, a worktree or job named with none, a warning, and the failure that ended it.
const (
	typeLine    = "line"
	typeAbsent  = "absent"
	typeWarning = "warning"
	typeError   = "error"
)

type streamedLine struct {
	Type string `json:"type"`
	runlogs.Entry
}

type streamedAbsent struct {
	Type string `json:"type"`
	runlogs.Absent
}

type streamedWarning struct {
	Type string `json:"type"`
	domain.Warning
}

type streamedError struct {
	Type  string        `json:"type"`
	Error *domain.Error `json:"error"`
}

// writeLine writes v to w as one line of JSON.
func writeLine(w io.Writer, v any) error {
	line, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(append(line, '\n'))
	return err
}
