package cli

import (
	"cmp"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/domain/run"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

const (
	none      = "—"
	jobsTitle = "JOBS"
)

// now is the clock ages are drawn against.
var now = time.Now

// drawStatus draws what was read: with --all the table of every worktree, however many; else the one worktree's state, or the error that kept it from being read; then each worktree that could not be read and the warnings.
func drawStatus(w io.Writer, read status.Outcome, req status.Request, view output.View) {
	switch {
	case req.All:
		drawAll(w, read.Items, view)
	case len(read.Items) == 1 && read.Items[0].Detail != nil:
		drawOne(w, *read.Items[0].Detail, view)
	}
	output.ReadEnd(w, read)
}

// ----------------------------------------------------------------------------
// One worktree

// drawOne concludes on whether anything is left to fix, reads the worktree back, how it runs first then where its branch stands, lists its jobs, and names each problem with the command that fixes it.
func drawOne(w io.Writer, d status.Detail, view output.View) {
	headline(w, cmp.Or(d.Branch, d.Path), len(d.Problems))
	output.Blank(w)

	st := output.StylesOf(w)
	var rows [][]string
	field := func(label, value string) {
		if value != "" {
			rows = append(rows, []string{st.Muted.Render(label), value})
		}
	}
	field("path", path(d))
	field("state", worktreeState(d))
	field("isolation", isolation(d))
	if d.Run == nil {
		field("run", runAbsent(d.RunToml))
	} else {
		field("addressing", string(d.Run.Addressing))
		field("ports", ports(d))
	}
	if !d.Missing {
		field("env", envFiles(d.Env))
	}
	field("base", base(d.Base))
	field("origin", origin(d))
	if d.Changes != nil {
		field("changes", changes(*d.Changes))
		field("last commit", lastCommit(d.Commits))
	}
	field("parent", d.Parent)
	field("children", strings.Join(d.Children, ", "))
	field("review", review(d.Review))
	output.Columns(w, rows)

	if d.Run != nil && d.Jobs != nil {
		output.Blank(w)
		drawJobs(w, d.Jobs)
	}
	if len(d.Problems) > 0 {
		output.Blank(w)
		drawProblems(w, d.Problems, view)
	}
}

func headline(w io.Writer, name string, problems int) {
	if problems == 0 {
		output.UnchangedLine(w, name+" — nothing to fix")
		return
	}
	output.WarningLine(w, name+" — "+counted(problems, "problem", "problems"))
}

func drawJobs(w io.Writer, jobs []status.Job) {
	if len(jobs) == 0 {
		output.UnchangedLine(w, "no job declared")
		return
	}

	output.SectionTitle(w, jobsTitle)
	st := output.StylesOf(w)
	var rows [][]string
	for _, job := range jobs {
		var held, age string
		if job.Owner != nil {
			held = "held from " + cmp.Or(job.Owner.Branch, job.Owner.Path)
		}
		if job.StartedAt != nil && (job.State == run.Running || job.State == run.Starting) {
			age = st.Muted.Render("up " + elapsed(*job.StartedAt))
		}
		rows = append(rows, []string{job.Name, stateOf(job.State, st), address(job), held, age})
	}
	output.Columns(w, rows)
}

func drawProblems(w io.Writer, problems []status.Problem, view output.View) {
	for _, problem := range problems {
		output.WarningLine(w, problem.Message)
		output.NextSteps(w, []output.NextStep{output.FollowUpStep(problem.Fix, view.CommandLine)})
	}
}

// ----------------------------------------------------------------------------
// Every worktree

// drawAll counts the worktrees, those that need attention and those not read, lists those read as a table, its run columns only when one of them runs and its review column only when one has a pull request, then names each problem under the worktree it belongs to.
func drawAll(w io.Writer, items []status.Item, view output.View) {
	var details []status.Detail
	for _, item := range items {
		if item.Detail != nil {
			details = append(details, *item.Detail)
		}
	}
	troubled := slices.DeleteFunc(slices.Clone(details), func(d status.Detail) bool { return len(d.Problems) == 0 })
	headlineAll(w, len(items), len(troubled), len(items)-len(details))
	if len(details) == 0 {
		return
	}
	output.Blank(w)

	st := output.StylesOf(w)
	withRun := slices.ContainsFunc(details, func(d status.Detail) bool { return d.Run != nil })
	withReview := slices.ContainsFunc(details, func(d status.Detail) bool { return d.Review != nil })
	header := []string{"WORKTREE"}
	if withRun {
		header = append(header, "ISOLATION", "PORTS", "JOBS")
	}
	header = append(header, "ENV")
	if withReview {
		header = append(header, "REVIEW")
	}
	for i, title := range header {
		header[i] = st.Muted.Render(title)
	}

	rows := [][]string{append([]string{""}, header...)}
	for _, d := range details {
		margin := ""
		if len(d.Problems) > 0 {
			margin = st.Warning.Render(output.GlyphAttention)
		}
		cells := []string{margin, subject(d, view.Cwd)}
		if withRun {
			cells = append(cells, isolation(d), cmp.Or(ports(d), none), jobsCell(d.Jobs))
		}
		cells = append(cells, envCell(d))
		if withReview {
			cells = append(cells, reviewCell(d.Review))
		}
		rows = append(rows, cells)
	}
	output.Columns(w, rows)

	for _, d := range troubled {
		output.Blank(w)
		output.SectionTitle(w, subject(d, view.Cwd))
		drawProblems(w, d.Problems, view)
	}
}

// headlineAll counts the worktrees, and how many need attention or could not be read; nothing to fix only when every one read and none has a problem.
func headlineAll(w io.Writer, worktrees, troubled, unread int) {
	line := counted(worktrees, "worktree", "worktrees")
	switch {
	case troubled == 1:
		line += output.Dot + "1 needs attention"
	case troubled > 1:
		line += output.Dot + strconv.Itoa(troubled) + " need attention"
	}
	if unread > 0 {
		line += output.Dot + strconv.Itoa(unread) + " not read"
	}

	switch {
	case unread > 0:
		output.ErrorLine(w, line)
	case troubled > 0:
		output.WarningLine(w, line)
	default:
		output.UnchangedLine(w, line+" — nothing to fix")
	}
}

// reviewCell is a pull request in a cell: its number, then its state when it is no longer open, else its failed checks, else its pending ones, and its review decision.
func reviewCell(r *status.Review) string {
	if r == nil {
		return none
	}
	if r.Progress == nil {
		return closedReview(r)
	}

	parts := []string{"#" + strconv.Itoa(r.Number)}
	switch {
	case r.Checks.Failed > 0:
		parts = append(parts, strconv.Itoa(r.Checks.Failed)+" failed")
	case r.Checks.Pending > 0:
		parts = append(parts, strconv.Itoa(r.Checks.Pending)+" pending")
	}
	if r.Decision != "" {
		parts = append(parts, decision(r.Decision))
	}
	return strings.Join(parts, output.Dot)
}

func jobsCell(jobs []status.Job) string {
	count := map[run.State]int{}
	for _, job := range jobs {
		count[job.State]++
	}

	var parts []string
	for _, state := range []run.State{run.Running, run.Starting, run.Crashed, run.Exited, run.Stopped} {
		if count[state] > 0 {
			parts = append(parts, strconv.Itoa(count[state])+" "+string(state))
		}
	}
	return cmp.Or(strings.Join(parts, output.Dot), none)
}

// envCell is how the worktree's .env files stand: none read when its directory is gone.
func envCell(d status.Detail) string {
	if d.Missing {
		return none
	}
	missing := missingFiles(d.Env)
	if missing > 0 {
		return strconv.Itoa(missing) + " missing"
	}
	return counted(d.Env.Declared, "file", "files")
}

// ----------------------------------------------------------------------------
// The fields

// subject names a worktree as the headline does: its branch, else its path from cwd.
func subject(d status.Detail, cwd string) string {
	return output.ShownWorktree(d.Branch, output.Shown{Cwd: cwd, Path: d.Path})
}

// worktreeState is what the worktree is in the middle of, or held by, if anything.
func worktreeState(d status.Detail) string {
	var states []string
	if d.Missing {
		states = append(states, "missing")
	}
	if d.Rebasing {
		states = append(states, "rebasing")
	}
	if d.Locked {
		states = append(states, "locked")
	}
	return strings.Join(states, output.Dot)
}

func path(d status.Detail) string {
	if d.Main {
		return d.Path + " (main)"
	}
	return d.Path
}

// isolation is the worktree's isolation, that it has not chosen one, or that its record does not tell.
func isolation(d status.Detail) string {
	switch {
	case d.Isolation != nil:
		return string(*d.Isolation)
	case d.Run != nil && d.Run.Ports == status.PortsNotChosen:
		return "not chosen"
	}
	return "unknown"
}

func runAbsent(runToml status.RunToml) string {
	if runToml == status.RunTomlInvalid {
		return "run.toml does not read"
	}
	return "no run.toml"
}

// ports says which ports the worktree binds, against the main worktree's: its own offset, the main's own, its source's, or none yet; nothing without a run.
func ports(d status.Detail) string {
	if d.Run == nil {
		return ""
	}

	switch d.Run.Ports {
	case status.PortsOffset:
		return "+" + strconv.Itoa(*d.Run.Offset)
	case status.PortsBase:
		return "base"
	case status.PortsSource:
		return "source's"
	case status.PortsNotNumbered:
		return "not numbered"
	case status.PortsNotChosen:
		return none
	case status.PortsUnknown:
	}
	return "unknown"
}

func envFiles(env status.Env) string {
	files := counted(env.Declared, "file", "files")
	if missing := missingFiles(env); missing > 0 {
		return files + output.Dot + strconv.Itoa(missing) + " missing"
	}
	return files
}

func missingFiles(env status.Env) int {
	missing := 0
	for _, file := range env.Files {
		if file.Missing {
			missing++
		}
	}
	return missing
}

func base(b *status.Base) string {
	if b == nil {
		return ""
	}
	return b.Name + " " + counts(b.Ahead, b.Behind)
}

// origin is where the branch stands against origin's, nothing on no branch.
func origin(d status.Detail) string {
	switch {
	case d.Branch == "":
		return ""
	case d.Origin == nil:
		return "not on origin"
	case d.Origin.State == worktree.UpToDate:
		return "up to date"
	}
	return counts(d.Origin.Ahead, d.Origin.Behind)
}

// counts are the commits ahead and behind, each when there are some, else up to date.
func counts(ahead, behind int) string {
	var parts []string
	if ahead > 0 {
		parts = append(parts, output.GlyphAhead+strconv.Itoa(ahead))
	}
	if behind > 0 {
		parts = append(parts, output.GlyphBehind+strconv.Itoa(behind))
	}
	return cmp.Or(strings.Join(parts, " "), "up to date")
}

func changes(c status.Changes) string {
	var parts []string
	for _, part := range []struct {
		count int
		label string
	}{{c.Staged, "staged"}, {c.Modified, "modified"}, {c.Untracked, "untracked"}} {
		if part.count > 0 {
			parts = append(parts, strconv.Itoa(part.count)+" "+part.label)
		}
	}
	if c.Insertions+c.Deletions > 0 {
		parts = append(parts, "+"+strconv.Itoa(c.Insertions)+" −"+strconv.Itoa(c.Deletions))
	}
	return cmp.Or(strings.Join(parts, output.Dot), "clean")
}

func lastCommit(commits []worktree.Commit) string {
	if len(commits) == 0 {
		return ""
	}

	last := commits[0]
	line := last.SHA + " " + last.Subject
	if last.Author != "" {
		line += output.Dot + last.Author
	}
	if !last.At.IsZero() {
		line += output.Dot + elapsed(last.At) + " ago"
	}
	return line
}

// closedReview is a pull request no longer open: its number and its state.
func closedReview(r *status.Review) string {
	return "#" + strconv.Itoa(r.Number) + " " + string(r.State)
}

func review(r *status.Review) string {
	if r == nil {
		return ""
	}
	if r.Progress == nil {
		return closedReview(r)
	}

	parts := []string{"#" + strconv.Itoa(r.Number) + " " + r.Title}
	if r.Draft {
		parts = append(parts, "draft")
	}
	for _, check := range []struct {
		count int
		label string
	}{{r.Checks.Passed, "passed"}, {r.Checks.Failed, "failed"}, {r.Checks.Pending, "pending"}} {
		if check.count > 0 {
			parts = append(parts, strconv.Itoa(check.count)+" "+check.label)
		}
	}
	if r.Decision != "" {
		parts = append(parts, decision(r.Decision))
	}
	return strings.Join(parts, output.Dot)
}

// stateOf is a job's state, its colour saying whether it is up, down or broken.
func stateOf(state run.State, st *output.Styles) string {
	switch state {
	case run.Running:
		return st.Success.Render(string(state))
	case run.Crashed:
		return st.Danger.Render(string(state))
	case run.Starting:
		return st.Warning.Render(string(state))
	case run.Exited, run.Stopped:
	}
	return st.Muted.Render(string(state))
}

// address is where a job answers: its url, else the port it publishes.
func address(job status.Job) string {
	if job.URL != "" || job.Port == nil {
		return job.URL
	}
	return "port " + strconv.Itoa(*job.Port)
}

// elapsed is the time since at, in its largest unit.
func elapsed(at time.Time) string {
	since := now().Sub(at)
	switch {
	case since < time.Minute:
		return strconv.Itoa(max(int(since.Seconds()), 0)) + "s"
	case since < time.Hour:
		return strconv.Itoa(int(since.Minutes())) + "m"
	case since < 24*time.Hour:
		return strconv.Itoa(int(since.Hours())) + "h"
	}
	return strconv.Itoa(int(since.Hours()/24)) + "d"
}

func counted(n int, one, many string) string {
	if n == 1 {
		return "1 " + one
	}
	return strconv.Itoa(n) + " " + many
}

// decision is GitHub's review decision as words: review required, changes requested, approved.
func decision(spelled string) string {
	return strings.ReplaceAll(strings.ToLower(spelled), "_", " ")
}
