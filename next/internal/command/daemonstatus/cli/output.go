package cli

import (
	"cmp"
	"io"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/daemonstatus"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

// drawStatus draws the daemon read: whether it runs, then its build, the jobs it holds by repository and worktree, its proxy, its socket and its index, then the warnings.
func drawStatus(w io.Writer, read daemonstatus.Outcome, _ daemonstatus.Request, view output.View) {
	if len(read.Items) == 1 && read.Items[0].Detail != nil {
		drawDaemon(w, *read.Items[0].Detail)
	}
	output.ReadEnd(w, read)
	if steps := output.WarningSteps(read.Warnings, view.CommandLine); len(steps) > 0 {
		output.Blank(w)
		output.NextSteps(w, steps)
	}
}

func drawDaemon(w io.Writer, d daemonstatus.Detail) {
	headline(w, d)
	output.Blank(w)

	st := output.StylesOf(w)
	var rows [][]string
	field := func(label, value string) {
		rows = append(rows, []string{st.Muted.Render(label), value})
	}
	field("build", build(d))
	switch {
	case d.JobsUnreadable:
		field("jobs", "can't be read: it speaks protocol "+strconv.Itoa(d.Protocol))
	case len(d.Repos) == 0:
		field("jobs", "none")
	}
	width := branchWidth(d.Repos)
	for i, repo := range d.Repos {
		label := ""
		if i == 0 {
			label = "jobs"
		}
		field(label, repoName(repo.Root))
		for _, wt := range repo.Worktrees {
			field("", "  "+pad(cmp.Or(wt.Branch, wt.Path), width)+"  "+jobsOf(wt.Jobs))
		}
	}
	if len(d.Left) > 0 {
		field("left", strings.Join(run.Names(d.Left), ", ")+" — by a daemon that died")
	}
	if d.Proxy != nil {
		field("proxy", proxy(*d.Proxy))
	}
	field("socket", d.Socket)
	field("index", d.Index)
	output.Columns(w, rows)
}

func headline(w io.Writer, d daemonstatus.Detail) {
	switch {
	case !d.Running:
		output.UnchangedLine(w, "wtm's daemon is not running")
	case d.Skew != run.SkewNone:
		output.WarningLine(w, "wtm's daemon runs another build · pid "+strconv.Itoa(d.PID))
	default:
		output.SuccessLine(w, "wtm's daemon runs · pid "+strconv.Itoa(d.PID))
	}
}

// build is this wtm's build, and the daemon's when it runs another.
func build(d daemonstatus.Detail) string {
	switch {
	case !d.Running || d.Skew == run.SkewNone:
		return d.Build
	case d.Skew == run.SkewLegacy:
		return "the legacy wtm's " + cmp.Or(d.Daemon, "(older)") + " · this wtm is " + d.Build
	}
	return cmp.Or(d.Daemon, "(older)") + " · this wtm is " + d.Build
}

// jobsOf names each job of a worktree, a stack, a shared job's own run and a hold on one said so.
func jobsOf(jobs []run.HeldJob) string {
	names := make([]string, 0, len(jobs))
	for _, job := range jobs {
		switch {
		case job.Hold == run.HoldClaim:
			names = append(names, job.Name+" (held)")
		case job.Shared:
			names = append(names, job.Name+" (shared)")
		case job.Hold == run.HoldDetached:
			names = append(names, job.Name+" (detached)")
		default:
			names = append(names, job.Name)
		}
	}
	return strings.Join(names, ", ")
}

func proxy(p daemonstatus.Proxy) string {
	if p.PublicPort > 0 && p.PublicPort != p.Port {
		return "port " + strconv.Itoa(p.Port) + " · addresses on :" + strconv.Itoa(p.PublicPort)
	}
	return "port " + strconv.Itoa(p.Port)
}

// branchWidth is the width of the longest worktree name of repos, the column the jobs start after.
func branchWidth(repos []run.HeldRepo) int {
	width := 0
	for _, repo := range repos {
		for _, wt := range repo.Worktrees {
			width = max(width, len(cmp.Or(wt.Branch, wt.Path)))
		}
	}
	return width
}

func pad(name string, width int) string {
	return name + strings.Repeat(" ", width-len(name))
}

// repoName is a repository by the name of its main checkout's directory.
func repoName(root string) string {
	if root == "" {
		return "(no repository)"
	}
	return filepath.Base(root)
}
