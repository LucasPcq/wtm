package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/runlogs"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of run logs' form.
func words(q runlogs.Questions, field domain.Field) form.Words {
	switch field.Spec().Path {
	case domain.PathWorktree:
		return form.Words{Flag: "<worktree>", Label: "Worktrees", Help: "The worktrees whose jobs to read, by branch, or by path on no branch (default: the one you stand in)", Empty: "Select at least one worktree", Options: form.RunningWords(q.Worktrees.Options)}
	case runlogs.PathJob:
		return form.Words{Flag: "--job", Label: "Job", Help: "Read this job of run.toml alone (default: every job that ran)"}
	case runlogs.PathFollow:
		return form.Words{Flag: "--follow", Shorthand: "f", Label: "Follow", Help: "Print each new line as the jobs print it, until they end or Ctrl+C"}
	}

	return form.Words{}
}
