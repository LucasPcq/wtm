package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/envshow"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of env show's form.
func words(_ envshow.Questions, field domain.Field) form.Words {
	switch field.Spec().Path {
	case domain.PathWorktree:
		return form.Words{Flag: "<worktree>", Label: "Worktree", Help: "The worktree whose .env files to show, by its branch, its path or part of its branch"}
	case envshow.PathShowValues:
		return form.Words{Flag: "--show-values", Label: "Show values", Help: "Show every value whole, secrets included; by default only what wtm writes is shown"}
	}

	return form.Words{}
}
