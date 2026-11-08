package cli

import (
	"cmp"
	"io"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/runps"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/ui/output"

	"github.com/charmbracelet/x/ansi"
)

// now is the time the ages of the jobs count up to.
var now = time.Now

const (
	lastLineMin    = 16
	nothingRunning = "nothing running"
	sharedTitle    = "shared services"
)

// drawPs draws each group read under its title, its jobs one per row with a runner's services and a shared service's holders under it, then what could not be read; the repository names each title once the read spans several.
func drawPs(w io.Writer, read runps.Outcome, _ runps.Request, _ output.View) {
	// a daemon that cannot tell what runs says so in its warning, never as nothing running
	unknown := slices.ContainsFunc(read.Warnings, func(warning domain.Warning) bool { return warning.Code == runps.Name+domain.JobsUnknown })
	if len(read.Items) == 0 && !unknown {
		output.UnchangedLine(w, nothingRunning)
	}
	repos := map[string]bool{}
	for _, item := range read.Items {
		if item.Detail != nil {
			repos[item.Detail.Repository] = true
		}
	}

	st := output.StylesOf(w)
	for i, item := range read.Items {
		if item.Detail == nil {
			continue
		}
		if i > 0 {
			output.Blank(w)
		}
		output.SectionTitle(w, title(*item.Detail, len(repos) > 1))
		var rows [][]string
		for _, job := range item.Detail.Jobs {
			rows = append(rows, []string{"  " + job.Name, stateOf(job, st), cmp.Or(job.Address, job.URL), lastLine(job, st)})
			for _, service := range job.Services {
				rows = append(rows, []string{"    " + service.Name, "", service.Address, ""})
			}
			for _, holder := range job.Holders {
				rows = append(rows, []string{"    " + cmp.Or(holder.Branch, holder.Path), "", holder.Namespace, since(holder, st)})
			}
		}
		output.Columns(w, fitLast(rows, output.BlockWidth(w)))
	}
	output.ReadEnd(w, read)
}

// title is a group's title: the worktree's branch, or shared services, then its profile and its counts; its repository first when the read spans several.
func title(d runps.Detail, repos bool) string {
	parts := []string{cmp.Or(d.Branch, d.Path)}
	if d.Shared {
		parts[0] = sharedTitle
	}
	if repos {
		parts[0] = filepath.Base(d.Repository) + " " + parts[0]
	}
	if d.Profile != "" {
		parts = append(parts, "profile "+d.Profile)
	}
	c := d.Counts
	for _, count := range []struct {
		n    int
		word string
	}{{c.Running, "up"}, {c.Starting, "starting"}, {c.Crashed, "crashed"}, {c.Exited, "exited"}, {c.Stopped, "stopped"}, {c.NotStarted, "not started"}} {
		if count.n > 0 {
			parts = append(parts, strconv.Itoa(count.n)+" "+count.word)
		}
	}
	if c.Holders > 0 {
		parts = append(parts, "held by "+output.Counted(c.Holders, "worktree", "worktrees"))
	}
	return strings.Join(parts, output.Dot)
}

// stateOf is a job's state with what it says of time: how long a job up has run, the code a job that ended ended with and how long ago; its colour says whether it is up, down or broken.
func stateOf(job runps.Job, st *output.Styles) string {
	words := []string{word(job)}
	if job.ExitCode != nil {
		words = append(words, "exit "+strconv.Itoa(*job.ExitCode))
	}
	switch {
	case job.EndedAt != nil:
		words = append(words, output.Span(now().Sub(*job.EndedAt))+" ago")
	case job.StartedAt != nil && (job.State == run.Running || job.State == run.Starting):
		words = append(words, "up "+output.Span(now().Sub(*job.StartedAt)))
	}
	text := strings.Join(words, output.Dot)

	switch job.State {
	case run.Running:
		return st.Success.Render(text)
	case run.Crashed:
		return st.Danger.Render(text)
	case run.Starting:
		return st.Warning.Render(text)
	case run.Exited, run.Stopped, run.NotStarted:
	}
	return st.Muted.Render(text)
}

// word is a job's state as a word: detached for a stack its launcher left running.
func word(job runps.Job) string {
	switch {
	case job.State == run.NotStarted:
		return "not started"
	case job.Hold == run.HoldDetached && job.State == run.Running:
		return "detached"
	}
	return string(job.State)
}

// lastLine is the last line a job printed, muted.
func lastLine(job runps.Job, st *output.Styles) string {
	if job.LastLine == nil {
		return ""
	}
	return st.Muted.Render(job.LastLine.Text)
}

// since is how long a holder has held its shared service.
func since(holder runps.Holder, st *output.Styles) string {
	if holder.Since == nil {
		return ""
	}
	return st.Muted.Render("since " + output.Span(now().Sub(*holder.Since)))
}

// fitLast cuts the last cell of each row, a job's last line, so the row fits width beside the cells before it, keeping at least lastLineMin columns of it.
func fitLast(rows [][]string, width int) [][]string {
	used := 0
	for col := range 3 {
		widest := 0
		for _, row := range rows {
			widest = max(widest, ansi.StringWidth(row[col]))
		}
		if widest > 0 {
			used += widest + len("  ")
		}
	}
	for _, row := range rows {
		row[3] = output.Truncate(row[3], max(width-used, lastLineMin))
	}
	return rows
}
