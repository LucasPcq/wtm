package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/resolve"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of resolve's form.
func words(_ resolve.Questions, field domain.Field) form.Words {
	if field.Spec().Path == domain.PathWorktree {
		return form.Words{Flag: "<worktree>", Label: "Worktree", Help: "The worktree to resolve, by its branch, its path or part of its branch"}
	}

	return form.Words{}
}
