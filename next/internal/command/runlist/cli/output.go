package cli

import (
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/runlist"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

const (
	profilesTitle = "Profiles"
	jobsTitle     = "Jobs"
	tagDefault    = "default"
	tagRunning    = "running"
	tagShared     = "shared"
	stateUnknown  = "unknown"
)

// drawList draws the worktree read: its profiles, each with its jobs, then its jobs, each with its state there, the ports it binds there and its command; then what could not be read.
func drawList(w io.Writer, read runlist.Outcome, _ runlist.Request, _ output.View) {
	st := output.StylesOf(w)
	for _, item := range read.Items {
		if item.Detail == nil {
			continue
		}
		d := *item.Detail
		output.SectionTitle(w, profilesTitle+output.Dot+item.Subject)
		var rows [][]string
		for _, p := range d.Profiles {
			rows = append(rows, []string{"  " + p.Name, profileTags(p, st), strings.Join(p.Jobs, ", ")})
		}
		output.Columns(w, rows)

		output.Blank(w)
		output.SectionTitle(w, jobsTitle)
		rows = nil
		for _, job := range d.Jobs {
			rows = append(rows, []string{"  " + job.Name, string(job.Kind), stateOf(job, st), ports(job), st.Muted.Render(job.Command)})
		}
		output.Columns(w, rows)
	}
	output.ReadEnd(w, read)
}

// profileTags are a profile's tags: whether a start with no profile named starts it, and whether the worktree runs it.
func profileTags(p runlist.Profile, st *output.Styles) string {
	var tags []string
	if p.Default {
		tags = append(tags, st.Muted.Render(tagDefault))
	}
	if p.Running {
		tags = append(tags, st.Success.Render(tagRunning))
	}
	return strings.Join(tags, " ")
}

// stateOf is where a job stands in the worktree, its colour saying whether it is up, down or broken.
func stateOf(job runlist.Job, st *output.Styles) string {
	if job.State == nil {
		return st.Muted.Render(stateUnknown)
	}
	switch *job.State {
	case run.Running:
		return st.Success.Render(string(*job.State))
	case run.Crashed:
		return st.Danger.Render(string(*job.State))
	case run.Starting:
		return st.Warning.Render(string(*job.State))
	case run.NotStarted:
		return st.Muted.Render("not started")
	case run.Exited, run.Stopped:
	}
	return st.Muted.Render(string(*job.State))
}

// ports are the ports a job binds in the worktree, by name, then shared for a job every worktree shares.
func ports(job runlist.Job) string {
	var parts []string
	for _, name := range slices.Sorted(maps.Keys(job.Ports)) {
		parts = append(parts, name+"="+strconv.Itoa(job.Ports[name]))
	}
	if job.Shared {
		parts = append(parts, tagShared)
	}
	return strings.Join(parts, "  ")
}
