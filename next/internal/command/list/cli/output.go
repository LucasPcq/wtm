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
	labelOrigin = "origin"
	glyphAhead  = "↑"
	glyphBehind = "↓"
)

// drawList draws a row per worktree read, the one holding the view's directory marked active, then each worktree that could not be read and the warnings.
func drawList(w io.Writer, read list.Outcome, view output.View) {
	cwd := view.Cwd
	st := output.StylesOf(w)
	active := list.Holding(read.Items, cwd)

	var rows [][]string
	for _, item := range read.Items {
		if item.Detail != nil {
			rows = append(rows, append([]string{shown(item, cwd)}, cellsOf(*item.Detail, item.Subject == active, st)...))
		}
	}
	output.Columns(w, rows)

	for _, item := range read.Items {
		if item.Err != nil {
			output.ErrorLine(w, item.Err.Error())
		}
	}
	for _, warning := range read.Warnings {
		output.WarningLine(w, warning.Message)
	}
}

// shown is how a row names its worktree: its branch, else its path from cwd.
func shown(item list.Item, cwd string) string {
	if item.Detail.Branch != "" {
		return item.Subject
	}
	return output.ShownPath(output.Shown{Cwd: cwd, Path: item.Detail.Path})
}

// cellsOf are the cells of a worktree's row after its subject: its tags, its pull request, the jobs up, its commits past the base branch, where it stands against origin and its state.
func cellsOf(d list.Detail, active bool, st *output.Styles) []string {
	return []string{tags(d, active, st), pr(d, st), services(d, st), ahead(d, st), origin(d, st), state(d, st)}
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
	return st.Success.Render("PR #" + strconv.Itoa(d.PR.Number))
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
	return st.Muted.Render(labelBase + " " + glyphAhead + strconv.Itoa(d.CommitsAhead))
}

// origin is where the branch stands against origin, nothing when up to date or not on origin.
func origin(d list.Detail, st *output.Styles) string {
	if d.Origin == nil {
		return ""
	}

	ahead, behind := glyphAhead+strconv.Itoa(d.Origin.Ahead), glyphBehind+strconv.Itoa(d.Origin.Behind)
	switch d.Origin.State {
	case worktree.Ahead:
		return st.Muted.Render(labelOrigin + " " + ahead)
	case worktree.Behind:
		return st.Warning.Render(labelOrigin + " " + behind)
	case worktree.Diverged:
		return st.Danger.Render(labelOrigin + " " + ahead + " " + behind)
	case worktree.NoOrigin, worktree.UpToDate:
	}
	return ""
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
