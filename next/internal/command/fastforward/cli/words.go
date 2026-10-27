package cli

import (
	"strings"

	"github.com/LucasPcq/wtm/next/internal/command/fastforward"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of fast-forward's form; force names the worktrees it would force on the questions observed.
func words(q fastforward.Questions, field domain.Field) form.Words {
	switch field.Spec().Path {
	case domain.PathWorktree:
		return form.Words{Flag: "<worktree>", Label: "Worktrees", Help: "The worktrees to fast-forward, by branch or by path; those behind origin as last fetched are checked", Empty: "Select at least one worktree"}
	case fastforward.PathAll:
		return form.Words{Flag: "--all", Label: "All", Help: "Fast-forward every worktree; one with uncommitted changes is left as it is unless --force", Options: map[string]form.OptionWords{
			"true":  {Label: "Every worktree", Short: "every worktree"},
			"false": {Label: "Those named", Short: "those named"},
		}}
	case fastforward.PathForce:
		return form.Words{Flag: "--force", Label: "Force", Help: forceHelp(q.Unsafe), Options: map[string]form.OptionWords{
			"true":  {Label: "Yes, fast-forward them anyway", Short: "forced"},
			"false": {Label: "No", Short: "not forced"},
		}}
	}

	return form.Words{}
}

func forceHelp(unsafe []fastforward.Advance) string {
	if len(unsafe) == 0 {
		return "Fast-forward a worktree that has uncommitted changes"
	}

	lines := []string{"These worktrees have uncommitted changes:"}
	for _, a := range unsafe {
		lines = append(lines, "  "+a.Worktree)
	}
	return strings.Join(lines, "\n")
}
