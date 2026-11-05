package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/rundown"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of run down's form.
func words(q rundown.Questions, field domain.Field) form.Words {
	switch field.Spec().Path {
	case domain.PathWorktree:
		return form.Words{Flag: "<worktree>", Label: "Worktrees", Help: "The worktrees to stop the jobs of, by branch, or by path on no branch or once removed (default: the one you stand in)", Empty: "Select at least one worktree", Options: form.RunningWords(q.Options)}
	case rundown.PathProfile:
		return form.Words{Flag: "--profile", Label: "Profile", Help: "Stop only this profile's jobs (default: every job the worktree runs)"}
	case rundown.PathAll:
		return form.Words{Flag: "--all", Label: "All", Help: "Stop the jobs of every worktree of this repository"}
	}

	return form.Words{}
}
