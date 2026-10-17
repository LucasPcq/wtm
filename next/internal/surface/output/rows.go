package output

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/LucasPcq/wtm/next/internal/app/create"
	"github.com/LucasPcq/wtm/next/internal/wtm"
	"github.com/LucasPcq/wtm/next/internal/wtm/branch"
)

const (
	noteSeparatorRow = " · "
	anomaliesTitle   = "Env ports left alone"
)

func planRowsOf(plan create.Planned) []planRow {
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

type itemView struct {
	row       planRow
	anomalies []string
	next      NextStep
}

func viewOf(item create.Item) itemView {
	detail := item.Detail
	return itemView{
		row:       planRow{Label: item.Subject, Values: []string{shownPath(detail.Path) + itemNote(item)}},
		anomalies: anomaliesOf(item),
		next:      NextStep{Command: "wtm go " + item.Subject},
	}
}

func itemNote(item create.Item) string {
	note := SettlementNote(item.Detail.EnvPorts)
	if item.Reason == wtm.ReasonUnchanged {
		note = "already open"
	}
	if note == "" {
		return ""
	}
	return muted.Render(noteSeparatorRow + note)
}

func anomaliesOf(item create.Item) []string {
	var anomalies []string
	standing := item.Detail.Standing
	counts := branch.AheadBehind{Ahead: standing.OriginAhead, Behind: standing.OriginBehind}
	switch {
	case !standing.ExistingBranch:
	case counts.Divergence() == branch.Behind:
		anomalies = append(anomalies, fmt.Sprintf("%s is %d commit(s) behind origin — the worktree starts from your local branch.", item.Subject, counts.Behind))
	case counts.Divergence() == branch.Diverged:
		anomalies = append(anomalies, fmt.Sprintf("%s has diverged from origin (%d ahead, %d behind) — the worktree starts from your local branch, missing commits that are on origin.", item.Subject, counts.Ahead, counts.Behind))
	}
	if lines := AnomalyLines(item.Detail.EnvPorts); len(lines) > 0 {
		anomalies = append(anomalies, anomaliesTitle+"\n"+strings.Join(lines, "\n"))
	}
	return anomalies
}

func shownPath(path string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return path
	}
	relative, err := filepath.Rel(resolvedOr(cwd), resolvedOr(path))
	if err != nil {
		return path
	}
	return relative
}

func resolvedOr(path string) string {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	return resolved
}
