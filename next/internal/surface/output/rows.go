package output

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/wtm/worktree"

	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
)

const (
	anomaliesTitle = "Env ports left alone"
)

func planRowsOf(plan create.Plan) []planRow {
	rows := []planRow{
		{Label: "Branches", Values: plannedBranches(plan.Branches)},
		{Label: sourceLabel(plan.Branches), Values: []string{plan.From}},
		{Label: "Env", Values: []string{string(plan.EnvFrom)}},
	}
	if plan.Isolation != "" {
		rows = append(rows, planRow{Label: "Isolation", Values: []string{string(plan.Isolation)}})
	}
	if len(plan.FastForward) > 0 {
		rows = append(rows, planRow{Label: "Fast-forward", Values: plan.FastForward})
	}
	if len(plan.Remember) > 0 {
		rows = append(rows, planRow{Label: "Remember", Values: plan.Remember})
	}
	return rows
}

func plannedBranches(branches []create.BranchPlan) []string {
	values := make([]string, 0, len(branches))
	for _, planned := range branches {
		switch planned.Action {
		case create.ActionCheckOut:
			values = append(values, planned.Branch+" (existing)")
		case create.ActionResume:
			values = append(values, planned.Branch+" (resumes "+planned.Upstream+")")
		case create.ActionUnchanged:
			values = append(values, planned.Branch+" (already open)")
		case create.ActionCreate:
			values = append(values, planned.Branch)
		}
	}
	return values
}

func sourceLabel(branches []create.BranchPlan) string {
	for _, planned := range branches {
		if planned.Action == create.ActionCreate {
			return "From"
		}
	}
	return "Parent"
}

func itemNote(item create.Item) string {
	note := settlementNote(item.Detail.EnvPorts)
	if item.Reason == wtm.ReasonUnchanged {
		note = "already open"
	}
	if note == "" {
		return ""
	}
	return Muted.Render(dot + note)
}

func anomaliesOf(item create.Item) []string {
	var anomalies []string
	standing := item.Detail.Standing
	switch standing.OriginState {
	case worktree.Behind:
		anomalies = append(anomalies, fmt.Sprintf("%s is %d commit(s) behind origin — the worktree starts from your local branch.", item.Subject, standing.OriginBehind))
	case worktree.Diverged:
		anomalies = append(anomalies, fmt.Sprintf("%s has diverged from origin (%d ahead, %d behind) — the worktree starts from your local branch, missing commits that are on origin.", item.Subject, standing.OriginAhead, standing.OriginBehind))
	case worktree.NoOrigin, worktree.UpToDate, worktree.Ahead:
	}
	if lines := anomalyLines(item.Detail.EnvPorts); len(lines) > 0 {
		anomalies = append(anomalies, anomaliesTitle+"\n"+strings.Join(lines, "\n"))
	}
	return anomalies
}

type shown struct {
	Cwd  string
	Path string
}

func shownPath(s shown) string {
	relative, err := filepath.Rel(s.Cwd, s.Path)
	if err != nil || s.Cwd == "" {
		return s.Path
	}
	return relative
}
