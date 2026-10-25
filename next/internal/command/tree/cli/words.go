package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/tree"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of tree's form.
func words(_ tree.Questions, field domain.Field) form.Words {
	if field.Spec().Path == tree.PathWithPRs {
		return form.Words{Flag: "--with-prs", Label: "Pull requests", Help: "Ask GitHub for each branch's newest pull request, open, merged or closed (needs gh)"}
	}

	return form.Words{}
}
