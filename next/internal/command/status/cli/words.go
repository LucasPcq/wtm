package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/status"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of status's form.
func words(_ status.Questions, field domain.Field) form.Words {
	switch field.Spec().Path {
	case domain.PathWorktree:
		return form.Words{Flag: "<worktree>", Label: "Worktree", Help: "The worktree to read, by its branch, its path or part of its branch"}
	case status.PathAll:
		return form.Words{Flag: "--all", Label: "All", Help: "Read every worktree of the repository"}
	case status.PathWithPRs:
		return form.Words{Flag: "--with-prs", Label: "Pull requests", Help: "Ask GitHub for the worktree's newest pull request, open, merged or closed, and an open one's checks and review (needs gh)"}
	}

	return form.Words{}
}
