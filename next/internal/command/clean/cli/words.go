package cli

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/clean"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of clean's form; the switches name what they decide on the questions observed: the children that would move, the worktrees that are unsafe.
func words(q clean.Questions, field domain.Field) form.Words {
	switch field.Spec().Path {
	case domain.PathWorktree:
		return form.Words{Flag: "<worktree>", Label: "Worktrees", Help: "The worktrees to remove, by branch, or by path on no branch", Empty: "Select at least one worktree"}
	case clean.PathReparentChildren:
		return form.Words{Flag: "--reparent-children", Label: "Reparent children", Help: reparentHelp(q.Orphans), Options: map[string]form.OptionWords{
			"true":  {Label: "Reparent them", Short: "reparent"},
			"false": {Label: "Leave them orphaned", Short: "leave orphaned"},
		}}
	case clean.PathForce:
		return form.Words{Flag: "--force", Label: "Force", Help: forceHelp(q.Unsafe), Options: map[string]form.OptionWords{
			"true":  {Label: "Yes, remove them anyway", Short: "forced"},
			"false": {Label: "No", Short: "not forced"},
		}}
	}

	return form.Words{}
}

func reparentHelp(orphans []clean.Move) string {
	if len(orphans) == 0 {
		return "Move the children a removed worktree leaves behind onto its nearest surviving ancestor, else the base branch"
	}

	lines := []string{"These children would be left orphaned:"}
	for _, move := range orphans {
		lines = append(lines, "  "+move.Branch+" would rebase onto "+move.To+" instead of "+move.From)
	}
	return strings.Join(lines, "\n")
}

var blockerWords = map[clean.Blocker]string{
	clean.BlockerLocked:          "locked",
	clean.BlockerDirty:           "uncommitted changes",
	clean.BlockerUnpushed:        "unpushed commits",
	clean.BlockerOpenPullRequest: "an open pull request",
	clean.BlockerJobsRunning:     "jobs running",
}

func forceHelp(unsafe []clean.Removal) string {
	if len(unsafe) == 0 {
		return "Remove a worktree that is locked, has uncommitted changes, unpushed commits, an open pull request or jobs running"
	}

	lines := []string{"These worktrees are unsafe to remove:"}
	for _, removal := range unsafe {
		reasons := make([]string, 0, len(removal.Blockers))
		for _, blocker := range removal.Blockers {
			reasons = append(reasons, blockerWords[blocker])
		}
		lines = append(lines, "  "+removal.Worktree+" — "+strings.Join(reasons, ", "))
	}
	return strings.Join(lines, "\n")
}
