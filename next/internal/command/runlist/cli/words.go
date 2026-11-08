package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/runlist"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of run list's form.
func words(_ runlist.Questions, field domain.Field) form.Words {
	if field.Spec().Path == domain.PathWorktree {
		return form.Words{Flag: "<worktree>", Label: "Worktree", Help: "The worktree whose jobs and profiles to list, by its branch, its path or part of its branch"}
	}

	return form.Words{}
}
