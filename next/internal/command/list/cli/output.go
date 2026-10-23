package cli

import (
	"io"
	"strconv"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/domain/worktree"
	"github.com/LucasPcq/wtm/next/internal/ui/output"
)

const (
	tagMain     = "main"
	tagActive   = "• active"
	tagLocked   = "locked"
	tagServices = "services"
	labelBase   = "base"
)

// drawList draws a row per worktree read, the one holding the view's directory marked active, then each worktree that could not be read and the warnings.
func drawList(w io.Writer, read list.Outcome, _ list.Request, view output.View) {
	cwd := view.Cwd
	st := output.StylesOf(w)
	active := list.Holding(read.Items, cwd)

	var rows [][]string
	for _, item := range read.Items {
		if item.Detail != nil {
			rows = append(rows, append([]string{shown(*item.Detail, cwd)}, cellsOf(*item.Detail, item.Subject == active, st)...))
		}
	}
	output.Columns(w, rows)
	output.ReadEnd(w, read)
}

// shown is how a row names its worktree: its branch, else its path from cwd.
func shown(d list.Detail, cwd string) string {
	return output.ShownWorktree(d.Branch, output.Shown{Cwd: cwd, Path: d.Path})
}

// cellsOf are the cells of a worktree's row after its subject: its tags, its pull request, the jobs up, its commits past the base branch, where it stands against origin and its state.
func cellsOf(d list.Detail, active bool, st *output.Styles) []string {
	return []string{tags(d, active, st), pr(d, st), services(d, st), ahead(d, st), st.Origin(d.Origin), state(d, st)}
}

func tags(d list.Detail, active bool, st *output.Styles) string {
	var tags []string
	if d.Main {
		tags = append(tags, st.Muted.Render(tagMain))
	}
	if active {
		tags = append(tags, st.Success.Render(tagActive))
	}
	if d.Locked {
		tags = append(tags, st.Warning.Render(output.GlyphAttention)+" "+tagLocked)
	}
	return strings.Join(tags, "  ")
}

func pr(d list.Detail, st *output.Styles) string {
	if d.PR == nil {
		return ""
	}
	label := "PR #" + strconv.Itoa(d.PR.Number)
	if d.PR.State == worktree.PullRequestOpen {
		return st.Success.Render(label)
	}
	return st.Muted.Render(label + " " + string(d.PR.State))
}

func services(d list.Detail, st *output.Styles) string {
	if len(d.Services) == 0 {
		return ""
	}
	return st.Success.Render(tagServices)
}

func ahead(d list.Detail, st *output.Styles) string {
	if d.CommitsAhead == 0 {
		return ""
	}
	return st.Muted.Render(labelBase + " " + output.GlyphAhead + strconv.Itoa(d.CommitsAhead))
}

// state is the worktree's state, its glyph coloured: a gone directory, then a rebase in progress, ahead of uncommitted changes.
func state(d list.Detail, st *output.Styles) string {
	switch {
	case d.Missing:
		return st.Danger.Render(output.GlyphFailure) + " missing"
	case d.Rebasing:
		return st.Warning.Render(output.GlyphAttention) + " rebasing"
	case d.Dirty:
		return st.Warning.Render(output.GlyphAttention) + " dirty"
	}
	return st.Success.Render(output.GlyphSuccess) + " clean"
}
