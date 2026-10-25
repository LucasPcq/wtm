package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/create"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of create's form.
func words(q create.Questions, field domain.Field) form.Words {
	path := field.Spec().Path
	newWorktree := form.NewWorktree{EnvHelp: "Where its .env files come from: config, example, main or parent; config.toml's strategy by default", Configured: q.Configured, Behind: q.Behind, Many: q.Many}
	if said, ok := newWorktree.Words(path); ok {
		return said
	}

	switch path {
	case create.PathBranches:
		return form.Words{Flag: "<branches>", Label: "Branches", Help: "One per line; a branch that exists is checked out as it is"}
	case create.PathFrom:
		return form.Words{Flag: "--from", Label: "Source branch", Help: "Where a new branch starts, and the parent wtm sync rebases it onto"}
	case create.PathIfNotExists:
		return form.Words{Flag: "--if-not-exists", Label: "If not exists", Help: "Succeed with no change when the worktree exists"}
	}

	return form.Words{}
}
