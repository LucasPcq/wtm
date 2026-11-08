package cli

import (
	"github.com/LucasPcq/wtm/next/internal/command/runstop"
	"github.com/LucasPcq/wtm/next/internal/domain"
	"github.com/LucasPcq/wtm/next/internal/ui/form"
)

// words are how the command line words each field of run stop's form.
func words(q runstop.Questions, field domain.Field) form.Words {
	switch field.Spec().Path {
	case domain.PathWorktree:
		return form.Words{Flag: "<worktree>", Label: "Worktrees", Help: "The worktrees to stop the job in, by branch, or by path on no branch or once removed (default: the one you stand in)", Empty: "Select at least one worktree", Options: form.RunningWords(q.Options)}
	case runstop.PathJob:
		return form.Words{Flag: "--job", Label: "Job", Help: "The job to stop, a job of run.toml or one wtm's daemon runs there", Options: form.JobWords(q.Jobs)}
	}

	return form.Words{}
}
