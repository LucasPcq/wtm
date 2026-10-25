package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/list"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of list's form.
func words(_ list.Questions, field domain.Field) form.Words {
	switch field.Spec().Path {
	case list.PathWithPRs:
		return form.Words{Flag: "--with-prs", Label: "Pull requests", Help: "Ask GitHub for each worktree's newest pull request, open, merged or closed (needs gh)"}
	case list.PathFetch:
		return form.Words{Flag: "--fetch", Label: "Fetch", Help: "Fetch origin first, as the picker's refresh (r) does"}
	}

	return form.Words{}
}
